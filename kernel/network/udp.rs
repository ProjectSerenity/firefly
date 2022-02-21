// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides support for sending and receiving UDP packets.
//!
//! This module includes functionality to [`open`](Config::open)
//! UDP ports, allowing UDP packets to be sent and received.
//! Note that the API used here is slightly different from
//! [Berkeley sockets](https://en.wikipedia.org/wiki/Berkeley_sockets).
//!
//! ## UDP ports
//!
//! Unlike the [`tcp`](super::tcp) module, `udp` does not
//! differentiate between clients and servers. To send or
//! receive packets, a local [`Port`] must be opened.
//!
//! To open a port, customise a [`Config`], then call its
//! [`open`](Config::open) method. With the port open,
//! packets can be sent using [`send_to`](Port::send_to)
//! and received using [`recv_from`](Port::recv_from).
//!
//! # Examples
//!
//! A simple echo server, returning the first Kibibyte
//! received in each packet to its sender.
//!
//! ```
//! // Send and receive packets on port 1234.
//! let config = udp::Config::default();
//! let port = config.open(1234).unwrap();
//! let mut buf = [0u8; 1024]; // Our 1 KiB echo buffer.
//!
//! // Process 10 packets.
//! for _ in 0..10 {
//!     // Wait for the next packet.
//!     let (n, peer) = port.recv_from(&mut buf[..]).unwrap();
//!
//!     // Print the payload and peer address.
//!     println!("{} from {:?}", str::from_utf8(&buf[..n]).unwrap(), peer);
//!
//!     // Return the packet.
//!     port.send_to(&buf[..n], peer).unwrap();
//! }
//!
//! // Close the port.
//! port.close();
//! ```

use super::{Error, InterfaceHandle, INTERFACES};
use alloc::collections::BTreeSet;
use alloc::vec;
use multitasking::thread::{current_thread_waker, prevent_next_sleep, suspend};
use smoltcp::iface::SocketHandle;
use smoltcp::socket::{UdpPacketMetadata, UdpSocket, UdpSocketBuffer};
use smoltcp::wire::IpEndpoint;
use spin::Mutex;
use x86_64::instructions::interrupts::without_interrupts;

/// Used as the number of packets in each port's receive
/// buffer.
///
const DEFAULT_RECV_PACKETS_BUFFER: usize = 64;

/// Used as the number of packets in each port's send
/// buffer.
///
const DEFAULT_SEND_PACKETS_BUFFER: usize = 64;

/// Used as the number of bytes in each port's receive
/// buffer.
///
const DEFAULT_RECV_BUFFER_SIZE: usize = 8192;

/// Used as the number of bytes in each port's send
/// buffer.
///
const DEFAULT_SEND_BUFFER_SIZE: usize = 4096;

/// Contains the set of UDP ports in active use.
//
// TODO: use a separate set of used ports for each interface.
//
static ACTIVE_PORTS: Mutex<BTreeSet<u16>> = Mutex::new(BTreeSet::new());

/// Returns a random port number in the range 49152 - 65535.
///
/// The returned port is guaranteed not to be in active use yet.
///
pub fn ephemeral_port() -> u16 {
    let mut active = ACTIVE_PORTS.lock();
    let mut buf = [0u8; 16];

    // Loop until we find a port we're happy to use.
    loop {
        // We give ourselves 8 tries at a time to avoid
        // calling random::read too often.
        random::read(&mut buf[..]);

        for i in 0..(buf.len() / 2) {
            let port = ((buf[i * 2] as u16) << 8) + (buf[i * 2 + 1] as u16);
            if port < 49152 {
                continue;
            }

            if active.contains(&port) {
                continue;
            }

            // Success!
            active.insert(port);
            return port;
        }
    }
}

/// Contains the configuration options for a UDP port.
///
pub struct Config {
    // If true, calls to send_to and recv_from on new
    // ports will return `Err(Error::NotReady)` if they
    // cannot occur immediately, instead of blocking.
    non_blocking: bool,

    // The number of packets in the receive buffer for
    // new ports.
    recv_packet_size: usize,

    // The number of packets in the send buffer for new
    // ports.
    send_packet_size: usize,

    // The number of bytes in the receive buffer for
    // new ports.
    recv_buffer_size: usize,

    // The number of bytes in the send buffer for new
    // ports.
    send_buffer_size: usize,
}

impl Default for Config {
    /// Returns the default port configuration.
    ///
    fn default() -> Self {
        Config {
            non_blocking: false,
            recv_packet_size: DEFAULT_RECV_PACKETS_BUFFER,
            send_packet_size: DEFAULT_SEND_PACKETS_BUFFER,
            recv_buffer_size: DEFAULT_RECV_BUFFER_SIZE,
            send_buffer_size: DEFAULT_SEND_BUFFER_SIZE,
        }
    }
}

impl Config {
    /// Sets new ports to non-blocking mode.
    ///
    /// If a call to [`send_to`](Port::send_to) or [`recv_from`](Port::recv_from)
    /// on a non-blocking port would otherwise block, it
    /// will instead return [`Error::NotReady`](super::Error::NotReady).
    ///
    /// Repeating the call at a later time may succeed.
    ///
    pub fn set_non_blocking(self) -> Self {
        Config {
            non_blocking: true,
            ..self
        }
    }

    /// Sets the number of packets in the receive buffer
    /// in new ports.
    ///
    /// `set_recv_packet_size` will panic if given a
    /// `num_packets` of zero.
    ///
    pub fn set_recv_packet_size(self, num_packets: usize) -> Self {
        Config {
            recv_packet_size: num_packets,
            ..self
        }
    }

    /// Sets the number of packets in the send buffer
    /// in new ports.
    ///
    /// `set_send_packet_size` will panic if given a `num_packets` of zero.
    ///
    pub fn set_send_packet_size(self, num_packets: usize) -> Self {
        Config {
            send_packet_size: num_packets,
            ..self
        }
    }

    /// Sets the size of the receive buffer in new ports.
    ///
    /// `set_recv_buffer_size` will panic if given a `size` of zero.
    ///
    pub fn set_recv_buffer_size(self, size: usize) -> Self {
        Config {
            recv_buffer_size: size,
            ..self
        }
    }

    /// Sets the size of the send buffer in new ports.
    ///
    /// `set_send_buffer_size` will panic if given a `size` of zero.
    ///
    pub fn set_send_buffer_size(self, size: usize) -> Self {
        Config {
            send_buffer_size: size,
            ..self
        }
    }

    /// Take ownership of a UDP port.
    ///
    /// Once the port is open, it can be used to send and receive UDP
    /// packets.
    ///
    /// If the local port is `0`, a random available port will be chosen.
    /// [`local_addr`](Port::local_addr) can be called to retrieve the
    /// chosen port.
    ///
    pub fn open<T: Into<IpEndpoint>>(&self, local: T) -> Result<Port, Error> {
        // Realise the arguments.
        let mut local = local.into();
        if local.port == 0 {
            local.port = ephemeral_port();
        } else {
            let mut active = ACTIVE_PORTS.lock();
            if active.contains(&local.port) {
                return Err(Error::PortInUse);
            }

            active.insert(local.port);
        }

        // Create the socket and bind it
        // to the local endpoint.
        let iface_handle = InterfaceHandle::new(0); // TODO: get this properly.
        let recv_buffer = UdpSocketBuffer::new(
            vec![UdpPacketMetadata::EMPTY; self.recv_packet_size],
            vec![0u8; self.recv_buffer_size],
        );
        let send_buffer = UdpSocketBuffer::new(
            vec![UdpPacketMetadata::EMPTY; self.send_packet_size],
            vec![0u8; self.send_buffer_size],
        );
        let mut socket = UdpSocket::new(recv_buffer, send_buffer);
        socket.bind(local)?;

        without_interrupts(|| {
            let mut ifaces = INTERFACES.lock();
            let iface = ifaces
                .get_mut(iface_handle.0)
                .expect("invalid interface handle");

            // Add the socket to the interface.
            let sock_handle = iface.iface.add_socket(socket);

            Ok(Port {
                iface: iface_handle,
                socket: sock_handle,
                local,
                non_blocking: self.non_blocking,
            })
        })
    }
}

/// Represents a UDP port, which can send and receive
/// packets.
///
pub struct Port {
    // The interface that owns this socket.
    iface: InterfaceHandle,

    // The socket we use to send and receive packets.
    socket: SocketHandle,

    // The address at our end of the connection.
    local: IpEndpoint,

    // Whether the port's APIs will return an error,
    // rather than blocking.
    non_blocking: bool,
}

impl Port {
    /// Close the connection.
    ///
    pub fn close(&self) {
        without_interrupts(|| {
            let mut ifaces = INTERFACES.lock();
            let iface = ifaces
                .get_mut(self.iface.0)
                .expect("invalid interface handle");

            // Close the connection.
            iface.iface.get_socket::<UdpSocket>(self.socket).close();
        });
    }

    /// Returns the address of this end of the connection.
    ///
    pub fn local_addr(&self) -> IpEndpoint {
        self.local
    }

    /// Send the given byte sequence to the specified peer.
    ///
    /// Returns the number of bytes sent. If no error is
    /// returned, then the number of bytes sent will be
    /// the length of `buf`.
    ///
    /// If the port is in non-blocking mode, and there is
    /// insufficient buffer space to send the packet now,
    /// `send_to` will return [`Error::NotReady`](super::Error::NotReady).
    ///
    pub fn send_to<T: Into<IpEndpoint>>(&self, buf: &[u8], peer: T) -> Result<usize, Error> {
        // Realise the peer's address.
        let peer = peer.into();
        let waker = current_thread_waker();

        without_interrupts(|| {
            // Wait until we're able to send.
            loop {
                let mut ifaces = INTERFACES.lock();
                let iface = ifaces
                    .get_mut(self.iface.0)
                    .expect("invalid interface handle");

                let socket = iface.iface.get_socket::<UdpSocket>(self.socket);
                if !socket.is_open() {
                    return Err(Error::ConnectionClosed);
                }

                if !socket.can_send() {
                    if self.non_blocking {
                        return Err(Error::NotReady);
                    }

                    prevent_next_sleep();
                    socket.register_send_waker(&waker);

                    // Drop our handles to avoid a deadlock.
                    drop(ifaces);

                    // Sleep until the state changes.
                    suspend();

                    continue;
                }

                socket.send_slice(buf, peer)?;

                // Send the packet.
                iface.poll();

                return Ok(buf.len());
            }
        })
    }

    /// Receive a byte sequence from a peer.
    ///
    /// Returns the number of bytes written to `buf`. If no
    /// error is returned, then the number of bytes will be
    /// non-zero.
    ///
    /// If the port is in non-blocking mode, and there are
    /// no packets currently buffered, `recv_from` will return
    /// [`Error::NotReady`](super::Error::NotReady).
    ///
    pub fn recv_from(&self, buf: &mut [u8]) -> Result<(usize, IpEndpoint), Error> {
        let waker = current_thread_waker();

        without_interrupts(|| {
            // Wait until we're able to receive.
            loop {
                let mut ifaces = INTERFACES.lock();
                let iface = ifaces
                    .get_mut(self.iface.0)
                    .expect("invalid interface handle");

                let socket = iface.iface.get_socket::<UdpSocket>(self.socket);
                if !socket.is_open() {
                    return Err(Error::ConnectionClosed);
                }

                if !socket.can_recv() {
                    if self.non_blocking {
                        return Err(Error::NotReady);
                    }

                    prevent_next_sleep();
                    socket.register_recv_waker(&waker);

                    // Drop our handles to avoid a deadlock.
                    drop(ifaces);

                    // Sleep until the state changes.
                    suspend();

                    continue;
                }

                let (bytes_read, peer) = socket.recv_slice(buf)?;
                if bytes_read > 0 {
                    return Ok((bytes_read, peer));
                }

                if self.non_blocking {
                    return Err(Error::NotReady);
                }

                // Try again.
                drop(ifaces);

                // Sleep until the state changes.
                suspend();
            }
        })
    }
}

impl Drop for Port {
    fn drop(&mut self) {
        without_interrupts(|| {
            let mut ifaces = INTERFACES.lock();
            let iface = ifaces
                .get_mut(self.iface.0)
                .expect("invalid interface handle");

            iface.iface.remove_socket(self.socket);

            // Release the port.
            ACTIVE_PORTS.lock().remove(&self.local.port);
        });
    }
}
