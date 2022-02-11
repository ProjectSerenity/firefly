// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements the kernel's networking functionality.
//!
//! This module manages the set of network interfaces that have been
//! discovered and installed. Interfaces can be used to send and
//! receive network packets.
//!
//! The network stack also controls the initial workloads, which are
//! started once a network interface receives a DHCP configuration.
//! This allows us to ensure that the network will be available when
//! the initial workload starts.

pub mod tcp;
pub mod udp;

use crate::multitasking::thread::ThreadId;
use alloc::boxed::Box;
use alloc::collections::BTreeMap;
use alloc::format;
use alloc::string::String;
use alloc::sync::Arc;
use alloc::vec;
use alloc::vec::Vec;
use core::slice;
use managed::ManagedSlice;
use memlayout::phys_to_virt_addr;
use serial::println;
use smoltcp;
use smoltcp::iface::{InterfaceBuilder, NeighborCache, Routes, SocketHandle, SocketStorage};
use smoltcp::phy::{DeviceCapabilities, RxToken, TxToken};
use smoltcp::socket::{Dhcpv4Config, Dhcpv4Event, Dhcpv4Socket};
use smoltcp::time::Instant;
use smoltcp::wire::{EthernetAddress, HardwareAddress, IpCidr, Ipv4Address, Ipv4Cidr};
use spin::Mutex;
use time::{now, Duration};
use x86_64::instructions::interrupts::without_interrupts;
use x86_64::PhysAddr;

/// INITIAL_WORKLOADS can be a set of thread ids that
/// should be resumed once we have a DHCP configuration.
///
/// When we next get a DHCP configuration, INITIAL_WORKLOADS
/// is iterated through, wich each thread resumed and
/// then removed from INITIAL_WORKLOADS.
///
static INITIAL_WORKLOADS: Mutex<Vec<ThreadId>> = Mutex::new(Vec::new());

/// Ensures that `thread_id` will be [resumed](crate::multitasking::thread::scheduler::resume)
/// when a DHCP configuration is next negotiated.
///
/// `thread_id` will be resumed at most once.
///
pub fn register_workload(thread_id: ThreadId) {
    without_interrupts(|| {
        let mut workloads = INITIAL_WORKLOADS.lock();
        workloads.push(thread_id);
    });
}

/// INTERFACES is the list of network interfaces.
///
static INTERFACES: Mutex<Vec<Interface>> = Mutex::new(Vec::new());

/// Represents a network interface, which can be used to
/// send and receive packets.
///
pub struct Interface {
    // name is the interface's unique name.
    name: String,

    // iface is the underlying Smoltcp Interface.
    iface: smoltcp::iface::Interface<'static, WrappedDevice>,

    // dhcp is the socket handle to the DHCP socket.
    dhcp: SocketHandle,

    // config is our current DHCP configuration, if any.
    config: Option<Dhcpv4Config>,
}

impl Interface {
    fn new(
        name: String,
        iface: smoltcp::iface::Interface<'static, WrappedDevice>,
        dhcp: SocketHandle,
    ) -> Self {
        let config = None;
        Interface {
            name,
            iface,
            dhcp,
            config,
        }
    }

    /// Instructs the interface to process inbound and outbound
    /// packets.
    ///
    /// `poll` returns the delay before `poll` should next be
    /// called to balance performance and resource usage.
    ///
    pub fn poll(&mut self) -> Duration {
        let now = Instant::from_micros(now().system_micros() as i64);
        loop {
            // Process the next inbound or outbound
            // packet.
            let processed = match self.iface.poll(now) {
                Ok(processed) => processed,
                Err(smoltcp::Error::Unrecognized) => continue, // Ignore bad packets.
                Err(err) => {
                    println!("warn: network poll error: {:?}", err);
                    break;
                }
            };

            // Check whether we've had a DHCP
            // state change.
            let event = self.iface.get_socket::<Dhcpv4Socket>(self.dhcp).poll();
            match event {
                None => {}
                Some(Dhcpv4Event::Configured(config)) => {
                    // We've received a new configuration.
                    println!("Received {}.", config.address);
                    self.iface.update_ip_addrs(|addrs| {
                        let addrs = match addrs {
                            ManagedSlice::Owned(vector) => vector,
                            _ => panic!("unexpected non-vector set of IP addresses"),
                        };

                        if addrs.is_empty() {
                            addrs.push(IpCidr::Ipv4(config.address));
                            return;
                        }

                        let dst = addrs.iter_mut().next().unwrap();
                        *dst = IpCidr::Ipv4(config.address);
                    });

                    if let Some(router) = config.router {
                        println!("Default gateway at {}.", router);
                        self.iface
                            .routes_mut()
                            .add_default_ipv4_route(router)
                            .unwrap();
                    } else {
                        println!("No default gateway.");
                        self.iface.routes_mut().remove_default_ipv4_route();
                    }

                    for (i, srv) in config.dns_servers.iter().enumerate() {
                        if let Some(srv) = srv {
                            println!("DNS server {}: {}.", i, srv);
                        }
                    }

                    self.config = Some(config);

                    // Resume any initial workloads.
                    let mut workloads = INITIAL_WORKLOADS.lock();
                    for thread_id in workloads.iter() {
                        thread_id.resume();
                    }

                    workloads.clear();
                }
                Some(Dhcpv4Event::Deconfigured) => {
                    if self.config.is_some() {
                        println!("Lost DHCP configuration.");
                    }

                    self.iface.update_ip_addrs(|addrs| {
                        if addrs.len() == 0 {
                            return;
                        }

                        let dst = addrs.iter_mut().next().unwrap();
                        *dst = IpCidr::Ipv4(Ipv4Cidr::new(Ipv4Address::UNSPECIFIED, 0));
                    });

                    self.iface.routes_mut().remove_default_ipv4_route();
                    self.config = None;
                }
            }

            // Stop if there are no more packets
            // to process.
            if !processed {
                break;
            }
        }

        // Determine when to poll again.
        let next = self.iface.poll_delay(now);
        match next {
            Some(duration) => Duration::from_micros(duration.micros()),
            None => Duration::from_secs(1),
        }
    }
}

/// Uniquely identifies a network interface.
///
#[derive(Clone, Copy)]
pub struct InterfaceHandle(usize);

impl InterfaceHandle {
    /// Returns a handle with the given index into INTERFACES.
    ///
    fn new(index: usize) -> Self {
        InterfaceHandle(index)
    }

    /// Returns the interface's unique name.
    ///
    pub fn name(&self) -> String {
        without_interrupts(|| {
            let ifaces = INTERFACES.lock();
            let iface = ifaces.get(self.0).expect("invalid interface handle");
            iface.name.clone()
        })
    }

    /// Returns the current DHCP configuration, if one has been
    /// established.
    ///
    pub fn dhcp_config(&self) -> Option<Dhcpv4Config> {
        without_interrupts(|| {
            let ifaces = INTERFACES.lock();
            let iface = ifaces.get(self.0).expect("invalid interface handle");
            iface.config
        })
    }

    /// Instructs the interface to process inbound and outbound
    /// packets.
    ///
    /// `poll` returns the delay before `poll` should next be
    /// called to balance performance and resource usage.
    ///
    pub fn poll(&self) -> Duration {
        without_interrupts(|| {
            let mut ifaces = INTERFACES.lock();
            let iface = ifaces.get_mut(self.0).expect("invalid interface handle");
            iface.poll()
        })
    }
}

/// Represents a network device, which can send and receive
/// packets.
///
pub trait Device: Send {
    /// Called to check whether the device has received any
    /// packets. If so, the next available packet buffer is
    /// returned as a pair of physical address and buffer
    /// length. If not, `None` is returned instead.
    ///
    fn recv_packet(&mut self) -> Option<(PhysAddr, usize)>;

    /// After a device returns a packet buffer from `recv_packet`,
    /// the buffer is returned to the device by calling
    /// `reclaim_recv_buffer`.
    ///
    fn reclaim_recv_buffer(&mut self, addr: PhysAddr, len: usize);

    /// Called when the interface wishes to send a packet of the
    /// given length.
    ///
    fn get_send_buffer(&mut self, len: usize) -> Result<PhysAddr, smoltcp::Error>;

    /// Called to send a packet buffer that was returned by a call
    /// to `get_send_buffer`.
    ///
    fn send_packet(&mut self, addr: PhysAddr, len: usize) -> Result<(), smoltcp::Error>;

    /// Describes this device's capabilities.
    ///
    fn capabilities(&self) -> DeviceCapabilities;
}

/// This is our device wrapper which we use to ensure all
/// our network interfaces are generic over the same type
/// (this one). If we instead allow our device drivers to
/// provide their own type, then we can't have a heterogeneous
/// container for them all.
///
struct WrappedDevice {
    dev: Arc<Mutex<Box<dyn Device>>>,
}

impl WrappedDevice {
    /// Wrap the given device.
    ///
    fn new(dev: Box<dyn Device>) -> Self {
        WrappedDevice {
            dev: Arc::new(Mutex::new(dev)),
        }
    }
}

impl<'a> smoltcp::phy::Device<'a> for WrappedDevice {
    type RxToken = RecvToken;
    type TxToken = SendToken;

    /// receive is called by the interface to check
    /// whether we have any packets to receive. We
    /// pop off the next packet from the receive
    /// queue and return it, or return None if not.
    ///
    fn receive(&'a mut self) -> Option<(Self::RxToken, Self::TxToken)> {
        without_interrupts(|| {
            let mut dev = self.dev.lock();
            if let Some((addr, len)) = dev.recv_packet() {
                let recv = RecvToken {
                    addr,
                    len,
                    dev: self.dev.clone(),
                };
                let send = SendToken {
                    dev: self.dev.clone(),
                };

                Some((recv, send))
            } else {
                None
            }
        })
    }

    /// transmit is called by the interface when
    /// it wants to send a packet.
    fn transmit(&'a mut self) -> Option<Self::TxToken> {
        Some(SendToken {
            dev: self.dev.clone(),
        })
    }

    /// capabilities describes this deivce's
    /// capabilities.
    ///
    fn capabilities(&self) -> DeviceCapabilities {
        without_interrupts(|| {
            let dev = self.dev.lock();
            dev.capabilities()
        })
    }
}

/// Implements RxToken for DeviceWrapper.
///
/// The DeviceWrapper returns a RecvToken when a packet
/// has been received and can be processed by the device.
///
/// This token contains the packet buffer, which we pass
/// to the interface and then return to the device.
///
struct RecvToken {
    addr: PhysAddr,
    len: usize,
    dev: Arc<Mutex<Box<dyn Device>>>,
}

impl<'a> RxToken for RecvToken {
    fn consume<R, F>(self, _timestamp: Instant, f: F) -> smoltcp::Result<R>
    where
        F: FnOnce(&mut [u8]) -> smoltcp::Result<R>,
    {
        // Pass our buffer to the callback to
        // process the packet.
        let virt_addr = phys_to_virt_addr(self.addr);
        let buf = unsafe { slice::from_raw_parts_mut(virt_addr.as_mut_ptr(), self.len) };
        let ret = f(buf);

        // Return the used buffer to the device
        // so it can be used to receive future
        // packets.
        without_interrupts(|| {
            let mut dev = self.dev.lock();
            dev.reclaim_recv_buffer(self.addr, self.len);
        });

        ret
    }
}

/// Implements TxToken for DeviceWrapper.
///
/// The DeviceWrapper returns a SendToken when the
/// interface wishes to send a packet.
///
/// The token contains only a handle to the device,
/// which is then used to send the packet.
///
struct SendToken {
    dev: Arc<Mutex<Box<dyn Device>>>,
}

impl<'a> TxToken for SendToken {
    fn consume<R, F>(self, _timestamp: Instant, len: usize, f: F) -> smoltcp::Result<R>
    where
        F: FnOnce(&mut [u8]) -> smoltcp::Result<R>,
    {
        // Start by getting a send buffer from
        // the device.
        let phys = without_interrupts(|| {
            //
            let mut dev = self.dev.lock();
            dev.get_send_buffer(len)
        })?;

        // Pass our buffer to the callback to
        // receive the packet data.
        let virt_addr = phys_to_virt_addr(phys);
        let buf = unsafe { slice::from_raw_parts_mut(virt_addr.as_mut_ptr(), len) };
        let ret = f(buf)?;

        // Send the packet.
        without_interrupts(|| {
            let mut dev = self.dev.lock();
            dev.send_packet(phys, len)
        })?;

        Ok(ret)
    }
}

/// Registers a new network interface, returning a unique
/// interface handle.
///
pub fn add_interface(device: Box<dyn Device>, mac: EthernetAddress) -> InterfaceHandle {
    // Wrap the device so we can use it
    // in a homogeneous container.
    let wrapped = WrappedDevice::new(device);

    // Create the interface.
    // TODO: Add a random seed.
    let ip_addrs = vec![IpCidr::Ipv4(Ipv4Cidr::new(Ipv4Address::UNSPECIFIED, 0))];
    let routes = Routes::new(BTreeMap::new());
    let neighbours = NeighborCache::new(BTreeMap::new());
    let sockets = Vec::<SocketStorage>::new();
    let mut iface = InterfaceBuilder::new(wrapped, sockets)
        .hardware_addr(HardwareAddress::Ethernet(mac))
        .neighbor_cache(neighbours)
        .ip_addrs(ip_addrs)
        .routes(routes)
        .finalize();

    // Enable DHCP.
    let socket = Dhcpv4Socket::new();
    let dhcp = iface.add_socket(socket);

    without_interrupts(|| {
        let mut ifaces = INTERFACES.lock();
        let name = format!("ethernet{}", ifaces.len());
        let handle = InterfaceHandle::new(ifaces.len());
        println!("New interface {} with MAC address {}.", &name, mac);
        ifaces.push(Interface::new(name, iface, dhcp));

        handle
    })
}

/// The set of errors that can occur when performing
/// network activities.
///
#[derive(Clone, Copy, Debug)]
pub enum Error {
    /// The provided address is not valid.
    InvalidAddress,

    /// The attempted operation is not currently valid.
    ///
    /// For example, this error would be returned if trying
    /// to send packets when a connection is not yet open.
    InvalidOperation,

    /// Failed to establish a connection.
    ConnectFailure,

    /// The connection is already closed.
    ConnectionClosed,

    /// The listener is already closed.
    ListenerClosed,

    /// This port is already being used elsewhere.
    PortInUse,

    /// An operation was cancelled by an expired timeout.
    Timeout,

    /// A non-blocking API has been called, but the action
    /// cannot yet be completed. Repeating the call at a
    /// later time may succeed.
    NotReady,

    /// This error was unexpected.
    ///
    /// Examine the inner error for more details.
    Unknown(smoltcp::Error),
}

impl From<smoltcp::Error> for Error {
    /// Convert the smoltcp error to a Firefly network error.
    ///
    fn from(err: smoltcp::Error) -> Self {
        match err {
            smoltcp::Error::Illegal => Error::InvalidOperation,
            smoltcp::Error::Unaddressable => Error::InvalidAddress,
            smoltcp::Error::Finished => Error::ConnectionClosed,
            _ => Error::Unknown(err),
        }
    }
}
