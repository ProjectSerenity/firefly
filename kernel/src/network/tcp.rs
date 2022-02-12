// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides support for TCP client and server connections.
//!
//! This module includes functionality to [`dial`](DialConfig::dial)
//! servers to create client connections and [`listen`](ListenConfig::listen)
//! as a server for client connections. Note that the API
//! used here is slightly different from [Berkeley sockets](https://en.wikipedia.org/wiki/Berkeley_sockets).
//!
//! ## TCP server
//!
//! To start a TCP server, customise a [`ListenConfig`],
//! then call its [`listen`](ListenConfig::listen) method
//! to listen for inbound connections. The server should
//! then iteratively call [`accept`](Listener::accept) to
//! accept a pending connection.
//!
//! ## TCP client
//!
//! To start a TCP client connection, customise a [`DialConfig`],
//! then call its [`dial`](DialConfig::dial) method to
//! connect to the remote server.
//!
//! # Examples
//!
//! A simple echo server, which services one connection
//! at a time, returning the first Kibibyte received on
//! each connection:
//!
//! ```
//! // Listen on port 80.
//! let config = tcp::ListenConfig::default();
//! let mut listener = config.listen(80).unwrap();
//! let mut buf = [0u8; 1024]; // Our 1 KiB echo buffer.
//!
//! // Process 10 connection.
//! for _ in 0..10 {
//!     let conn = listener.accept.unwrap();
//!     let n = conn.recv(&mut buf[..]).unwrap();
//!     conn.send(&buf[..n]).unwrap();
//!     conn.close();
//! }
//!
//! // Close the listener, rejecting
//! // any unaccepted connections.
//! listener.close();
//! ```
//!
//! A naive HTTP request for `http://example.com/`:
//!
//! ```
//! // Create the destination IP endpoint.
//! let config = tcp::DialConfig::default();
//! let addr = (IpAddress::v4(1, 2, 3, 4), 80);
//! let conn = config.dial(addr).unwrap();
//!
//! // Send our HTTP request.
//! conn.send(b"GET / HTTP/1.0\r\nHost: example.com\r\n\r\n").unwrap();
//!
//! // Receive and print the first 4 KiB
//! // of the response.
//! let mut buf = [0u8; 4096];
//! let n = conn.recv(&mut buf[..]).unwrap();
//! println!("{}", str::from_utf8(&buf[..n]).unwrap());
//!
//! // Close the connection.
//! conn.close();
//! ```

use super::{Error, InterfaceHandle, INTERFACES};
use crate::multitasking::thread::{current_global_thread_id, suspend};
use alloc::collections::BTreeSet;
use alloc::vec;
use alloc::vec::Vec;
use core::cmp::min;
use smoltcp::iface::SocketHandle;
use smoltcp::socket::{TcpSocket, TcpSocketBuffer};
use smoltcp::wire::IpEndpoint;
use spin::Mutex;
use x86_64::instructions::interrupts::without_interrupts;

/// Used as the number of bytes in each connection's receive
/// buffer.
///
const DEFAULT_RECV_BUFFER_SIZE: usize = 8192;

/// Used as the number of bytes in each connection's send
/// buffer.
///
const DEFAULT_SEND_BUFFER_SIZE: usize = 4096;

/// Contains the set of TCP ports in active use.
///
/// We don't currently remove ports from the list, as the
/// obvious time to remove it would be when the connection
/// is closed, but we don't want to risk confusing the old
/// and the new connection if the port is reused soon after
/// the old connection closed. This would be particularly
/// risky if the FIN packet was lost in transit.
//
// TODO: decide a way to remove used ports from the list.
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

/// The maximum backlog size for any one Listener.
///
/// If [`listen`](ListenConfig::listen) is called
/// with a larger `backlog` than `MAX_BACKLOG`, then
/// `MAX_BACKLOG` is used instead.
///
pub const MAX_BACKLOG: usize = 128;

/// The default backlog size for a Listener.
///
const DEFAULT_BACKLOG: usize = 16;

/// Contains the configuration options for a TCP server.
///
pub struct ListenConfig {
    // If true, calls to send_to and recv_from on new
    // ports will return `Err(Error::NotReady)` if they
    // cannot occur immediately, instead of blocking.
    non_blocking: bool,

    // The max number of pending connections.
    backlog: usize,

    // The receive buffer size for new connections.
    recv_buffer_size: usize,

    // The send buffer size for new connections.
    send_buffer_size: usize,
}

impl Default for ListenConfig {
    /// Returns the default listener configuration.
    ///
    fn default() -> Self {
        ListenConfig {
            non_blocking: false,
            backlog: DEFAULT_BACKLOG,
            recv_buffer_size: DEFAULT_RECV_BUFFER_SIZE,
            send_buffer_size: DEFAULT_SEND_BUFFER_SIZE,
        }
    }
}

impl ListenConfig {
    /// Sets new connections to non-blocking mode.
    ///
    /// If a call to [`accept`](Listener::accept) on a
    /// non-blocking listener, or [`send`](Connection::send)
    /// or [`recv`](Connection::recv) on a non-blocking
    /// connection would otherwise block, it will instead
    /// return [`Error::NotReady`](super::Error::NotReady).
    ///
    /// Repeating the call at a later time may succeed.
    ///
    pub fn set_non_blocking(self) -> Self {
        ListenConfig {
            non_blocking: true,
            ..self
        }
    }

    /// Sets the maximum number of pending connections that can be held
    /// simultaneously.
    ///
    /// Pending connections are completed by calling [`accept`](Listener::accept).
    /// If the backlog is full and another connection attempt is received,
    /// it may be rejected.
    ///
    /// `set_backlog` will panic if given a `backlog` of zero. If the
    /// `backlog` is larger than [`MAX_BACKLOG`], the maximum is used
    /// instead.
    ///
    pub fn set_backlog(self, backlog: usize) -> Self {
        let backlog = min(backlog, MAX_BACKLOG);

        ListenConfig { backlog, ..self }
    }

    /// Sets the size of the receive buffer in new connections.
    ///
    /// `set_recv_buffer_size` will panic if given a `size` of zero.
    ///
    pub fn set_recv_buffer_size(self, size: usize) -> Self {
        ListenConfig {
            recv_buffer_size: size,
            ..self
        }
    }

    /// Sets the size of the send buffer in new connections.
    ///
    /// `set_send_buffer_size` will panic if given a `size` of zero.
    ///
    pub fn set_send_buffer_size(self, size: usize) -> Self {
        ListenConfig {
            send_buffer_size: size,
            ..self
        }
    }

    /// Listen for connections from client peers.
    ///
    /// If the local port is `0`, a random available port will be chosen.
    /// [`local_addr`](Listener::local_addr) can be called to retrieve the
    /// chosen port.
    ///
    pub fn listen<T: Into<IpEndpoint>>(&self, local: T) -> Result<Listener, Error> {
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

        // Decide our backlog size and allocate the
        // vector of pending connections.;
        let mut backlog = Vec::with_capacity(self.backlog);

        let iface_handle = InterfaceHandle::new(0); // TODO: get this properly.

        without_interrupts(|| {
            let mut ifaces = INTERFACES.lock();
            let iface = ifaces
                .get_mut(iface_handle.0)
                .expect("invalid interface handle");

            // Fill the backlog of listening sockets.
            for _ in 0..self.backlog {
                // Create the socket and tell it to start
                // listening on the local address.
                let recv_buffer = TcpSocketBuffer::new(vec![0u8; self.recv_buffer_size]);
                let send_buffer = TcpSocketBuffer::new(vec![0u8; self.send_buffer_size]);
                let mut socket = TcpSocket::new(recv_buffer, send_buffer);
                socket.listen(local)?;

                // Add the socket to the interface.
                let sock_handle = iface.iface.add_socket(socket);

                let conn = Connection {
                    iface: iface_handle,
                    socket: sock_handle,
                    non_blocking: self.non_blocking,
                    local,
                    remote: IpEndpoint::UNSPECIFIED,
                };

                backlog.push(conn);
            }

            let listener = Listener {
                iface: iface_handle,
                local,
                backlog,
                listening: true,
                non_blocking: self.non_blocking,
                recv_buffer_size: self.recv_buffer_size,
                send_buffer_size: self.send_buffer_size,
            };

            Ok(listener)
        })
    }
}

/// Represents a TCP server socket.
///
pub struct Listener {
    // The interface that owns this socket.
    iface: InterfaceHandle,

    // The address of the listening server.
    local: IpEndpoint,

    // The backlog of pending connections.
    backlog: Vec<Connection>,

    // Whether we are still listening.
    listening: bool,

    // Whether the listener's APIs will return
    // an error, rather than blocking.
    non_blocking: bool,

    // The receive buffer size for new connections.
    recv_buffer_size: usize,

    // The send buffer size for new connections.
    send_buffer_size: usize,
}

impl Listener {
    /// Accept the next pending connection.
    ///
    /// `accept` will block until a connection is available.
    /// If the Listener has been closed, then `accept` will
    /// return immediately with `Err(Error::ListenerClosed)`.
    ///
    pub fn accept(&mut self) -> Result<Connection, Error> {
        if !self.listening {
            return Err(Error::ListenerClosed);
        }

        let global_thread_id = current_global_thread_id();

        without_interrupts(|| {
            // Loop until we find a pending connection.
            loop {
                let mut ifaces = INTERFACES.lock();
                let iface = ifaces
                    .get_mut(self.iface.0)
                    .expect("invalid interface handle");

                // See whether any pending sockets are
                // ready yet.
                let mut found = None;
                for (i, conn) in self.backlog.iter().enumerate() {
                    let socket = iface.iface.get_socket::<TcpSocket>(conn.socket);
                    if socket.may_send() || socket.may_recv() {
                        found = Some(i);
                        break;
                    }
                }

                match found {
                    None => {
                        if self.non_blocking {
                            return Err(Error::ConnectionClosed);
                        }

                        // Set a recv waker on each backlog socket
                        // then drop our handles and suspend ourself.
                        //
                        // We use a single waker so hopefully we're
                        // only awoken once.
                        let waker = global_thread_id.waker();
                        for conn in self.backlog.iter() {
                            let socket = iface.iface.get_socket::<TcpSocket>(conn.socket);
                            socket.register_recv_waker(&waker);
                        }

                        drop(ifaces);

                        suspend();

                        continue;
                    }
                    Some(i) => {
                        // Pop the connection we've found, then
                        // replace it with a new one.
                        let mut conn = self.backlog.remove(i);
                        let socket = iface.iface.get_socket::<TcpSocket>(conn.socket);
                        conn.remote = socket.remote_endpoint();

                        // Make a replacement.
                        let recv_buffer = TcpSocketBuffer::new(vec![0u8; self.recv_buffer_size]);
                        let send_buffer = TcpSocketBuffer::new(vec![0u8; self.send_buffer_size]);
                        let mut socket = TcpSocket::new(recv_buffer, send_buffer);
                        socket.listen(self.local)?;

                        // Add the socket to the interface.
                        let sock_handle = iface.iface.add_socket(socket);

                        self.backlog.push(Connection {
                            iface: self.iface,
                            socket: sock_handle,
                            non_blocking: self.non_blocking,
                            local: self.local,
                            remote: IpEndpoint::UNSPECIFIED,
                        });

                        // Return the accepted connection.
                        return Ok(conn);
                    }
                }
            }
        })
    }

    /// Returns the listener's local address.
    ///
    pub fn local_addr(&self) -> IpEndpoint {
        self.local
    }

    /// Close the listener.
    ///
    /// Calling `close`, rejects any future connection attempts, and
    /// any pending connections not yet `accept`ed, but leaves all
    /// connections that have already been `accept`ed intact. This
    /// allows the server to perform a graceful shutdown.
    ///
    pub fn close(&mut self) {
        self.listening = false;
        for conn in self.backlog.iter_mut() {
            conn.close();
        }

        self.backlog.truncate(0);
    }
}

impl Drop for Listener {
    fn drop(&mut self) {
        without_interrupts(|| {
            for conn in self.backlog.iter_mut() {
                conn.close();
            }
        });
    }
}

/// Contains the configuration options for a TCP client.
///
pub struct DialConfig {
    // If true, calls to send and recv on new
    // connections will return `Err(Error::NotReady)` if
    // they cannot occur immediately, instead of blocking.
    non_blocking: bool,

    // The local address that should be used when opening
    // new, outbound, connections.
    local: IpEndpoint,

    // The receive buffer size for new connections.
    recv_buffer_size: usize,

    // The send buffer size for new connections.
    send_buffer_size: usize,
}

impl Default for DialConfig {
    /// Returns the default dialer configuration.
    ///
    fn default() -> Self {
        DialConfig {
            non_blocking: false,
            local: IpEndpoint::UNSPECIFIED,
            recv_buffer_size: DEFAULT_RECV_BUFFER_SIZE,
            send_buffer_size: DEFAULT_SEND_BUFFER_SIZE,
        }
    }
}

impl DialConfig {
    /// Sets new connections to non-blocking mode.
    ///
    /// If a call to [`send`](Connection::send) or [`recv`](Connection::recv)
    /// on a non-blocking connection would otherwise block, it
    /// will instead return [`Error::NotReady`](super::Error::NotReady).
    ///
    /// Repeating the call at a later time may succeed.
    ///
    pub fn set_non_blocking(self) -> Self {
        DialConfig {
            non_blocking: true,
            ..self
        }
    }

    /// Set the local address that should be used when
    /// opening new, outbound, connections.
    ///
    /// The kind of address specified may limit the
    /// connections that can be opened. For example,
    /// setting a local IPv4 address will cause [`dial`](DialConfig::dial)
    /// calls to IPv6 addresses to fail.
    ///
    pub fn set_local_addr<T: Into<IpEndpoint>>(self, addr: T) -> Self {
        DialConfig {
            local: addr.into(),
            ..self
        }
    }

    /// Sets the size of the receive buffer in new connections.
    ///
    /// `set_recv_buffer_size` will panic if given a `size` of zero.
    ///
    pub fn set_recv_buffer_size(self, size: usize) -> Self {
        DialConfig {
            recv_buffer_size: size,
            ..self
        }
    }

    /// Sets the size of the send buffer in new connections.
    ///
    /// `set_send_buffer_size` will panic if given a `size` of zero.
    ///
    pub fn set_send_buffer_size(self, size: usize) -> Self {
        DialConfig {
            send_buffer_size: size,
            ..self
        }
    }

    /// Connect to a remote server.
    ///
    pub fn dial<T: Into<IpEndpoint>>(&self, remote: T) -> Result<Connection, Error> {
        // Realise the arguments.
        let remote = remote.into();
        let local = if self.local.port == 0 {
            IpEndpoint {
                port: ephemeral_port(),
                ..self.local
            }
        } else {
            let mut active = ACTIVE_PORTS.lock();
            if active.contains(&self.local.port) {
                return Err(Error::PortInUse);
            }

            active.insert(self.local.port);

            self.local
        };

        // Create the socket.
        let recv_buffer = TcpSocketBuffer::new(vec![0u8; self.recv_buffer_size]);
        let send_buffer = TcpSocketBuffer::new(vec![0u8; self.send_buffer_size]);
        let socket = TcpSocket::new(recv_buffer, send_buffer);
        let iface_handle = InterfaceHandle::new(0); // TODO: get this properly.
        let global_thread_id = current_global_thread_id();

        without_interrupts(|| {
            let mut ifaces = INTERFACES.lock();
            let iface = ifaces
                .get_mut(iface_handle.0)
                .expect("invalid interface handle");

            // Add the socket to the interface.
            let sock_handle = iface.iface.add_socket(socket);

            // Immediately get the socket back so we can get
            // a context with it. We need the context to use
            // socket.connect.
            let (socket, context) = iface.iface.get_socket_and_context::<TcpSocket>(sock_handle);

            // Connect to the remote server.
            socket.connect(context, remote, local)?;

            // Send the SYN packet.
            iface.poll();

            // Either we're good to go already (somehow), or
            // we need to set a send waker and wait.
            let socket = iface.iface.get_socket::<TcpSocket>(sock_handle);
            if !socket.is_open() {
                return Err(Error::ConnectFailure);
            }

            if socket.may_send() {
                let conn = Connection {
                    iface: iface_handle,
                    socket: sock_handle,
                    non_blocking: self.non_blocking,
                    local,
                    remote,
                };

                return Ok(conn);
            }

            // Set our waker.
            socket.register_send_waker(&global_thread_id.waker());

            // Drop our hold on the interface so we can
            // suspend ourself without causing a deadlock.
            drop(ifaces);

            loop {
                suspend();

                // We've been awoken, but that doesn't
                // necessarily mean we're ready to send
                // yet. We need to regain our access to
                // the socket so we can check its state.
                let mut ifaces = INTERFACES.lock();
                let iface = ifaces
                    .get_mut(iface_handle.0)
                    .expect("invalid interface handle");

                let socket = iface.iface.get_socket::<TcpSocket>(sock_handle);
                if !socket.is_open() {
                    return Err(Error::ConnectFailure);
                }

                if socket.may_send() {
                    let conn = Connection {
                        iface: iface_handle,
                        socket: sock_handle,
                        non_blocking: self.non_blocking,
                        local,
                        remote,
                    };

                    return Ok(conn);
                }

                // Set our waker.
                socket.register_send_waker(&global_thread_id.waker());
            }
        })
    }
}

/// Represents a TCP connection.
///
pub struct Connection {
    // The interface that owns this socket.
    iface: InterfaceHandle,

    // The socket we use to send and receive packets.
    socket: SocketHandle,

    // If true, calls to send and recv on new
    // connections will return `Err(Error::NotReady)`
    // if they cannot occur immediately, instead of
    // blocking.
    non_blocking: bool,

    // The address at our end of the connection.
    local: IpEndpoint,

    // The address at the other end of the connection.
    remote: IpEndpoint,
}

impl Connection {
    /// Close the connection.
    ///
    pub fn close(&self) {
        without_interrupts(|| {
            let mut ifaces = INTERFACES.lock();
            let iface = ifaces
                .get_mut(self.iface.0)
                .expect("invalid interface handle");

            // Close the connection, preparing a FIN.
            iface.iface.get_socket::<TcpSocket>(self.socket).close();

            // Send the packet.
            iface.poll();
        });
    }

    /// Returns the address of this end of the connection.
    ///
    pub fn local_addr(&self) -> IpEndpoint {
        self.local
    }

    /// Returns the address of the peer of this connection.
    ///
    pub fn remote_addr(&self) -> IpEndpoint {
        self.remote
    }

    /// Send the given byte sequence to the peer.
    ///
    /// Returns the number of bytes sent. If no error is
    /// returned, then the number of bytes sent will be
    /// the length of `buf`.
    ///
    pub fn send(&self, buf: &[u8]) -> Result<usize, Error> {
        let mut bytes_sent = 0;
        let global_thread_id = current_global_thread_id();

        without_interrupts(|| {
            // Wait until we're able to send.
            loop {
                let mut ifaces = INTERFACES.lock();
                let iface = ifaces
                    .get_mut(self.iface.0)
                    .expect("invalid interface handle");

                let socket = iface.iface.get_socket::<TcpSocket>(self.socket);
                if !socket.is_open() {
                    return Err(Error::ConnectionClosed);
                }

                if !socket.can_send() {
                    if self.non_blocking {
                        return Err(Error::NotReady);
                    }

                    socket.register_send_waker(&global_thread_id.waker());

                    // Drop our handles to avoid a deadlock.
                    drop(ifaces);

                    // Sleep until the state changes.
                    suspend();

                    continue;
                }

                bytes_sent += socket.send_slice(&buf[bytes_sent..])?;

                // Advance the state machine.
                iface.poll();

                if bytes_sent == buf.len() {
                    return Ok(bytes_sent);
                }
            }
        })
    }

    /// Receive a byte sequence from the peer.
    ///
    /// Returns the number of bytes written to `buf`. If no
    /// error is returned, then the number of bytes will be
    /// non-zero.
    ///
    pub fn recv(&self, buf: &mut [u8]) -> Result<usize, Error> {
        let global_thread_id = current_global_thread_id();

        without_interrupts(|| {
            // Wait until we're able to receive.
            loop {
                let mut ifaces = INTERFACES.lock();
                let iface = ifaces
                    .get_mut(self.iface.0)
                    .expect("invalid interface handle");

                let socket = iface.iface.get_socket::<TcpSocket>(self.socket);
                if !socket.is_open() {
                    return Err(Error::ConnectionClosed);
                }

                if !socket.can_recv() {
                    if self.non_blocking {
                        return Err(Error::NotReady);
                    }

                    socket.register_recv_waker(&global_thread_id.waker());

                    // Drop our handles to avoid a deadlock.
                    drop(ifaces);

                    // Sleep until the state changes.
                    suspend();

                    continue;
                }

                let bytes_read = socket.recv_slice(buf)?;
                if bytes_read > 0 {
                    return Ok(bytes_read);
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

impl Drop for Connection {
    fn drop(&mut self) {
        without_interrupts(|| {
            let mut ifaces = INTERFACES.lock();
            let iface = ifaces
                .get_mut(self.iface.0)
                .expect("invalid interface handle");

            iface.iface.remove_socket(self.socket);
        });
    }
}
