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

#![no_std]
#![deny(clippy::float_arithmetic)]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![allow(clippy::panic)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(unused_crate_dependencies)]
#![allow(unsafe_code)]
#![feature(const_btree_new)]

extern crate alloc;

mod device;
pub mod tcp;
pub mod udp;

use self::device::WrappedDevice;
use alloc::boxed::Box;
use alloc::collections::BTreeMap;
use alloc::format;
use alloc::string::String;
use alloc::vec;
use alloc::vec::Vec;
use core::task::Waker;
use managed::ManagedSlice;
use memory::PhysAddr;
use serial::println;
use smoltcp::iface::{InterfaceBuilder, NeighborCache, Routes, SocketHandle, SocketStorage};
use smoltcp::phy::DeviceCapabilities;
use smoltcp::socket::{Dhcpv4Config, Dhcpv4Event, Dhcpv4Socket};
use smoltcp::time::Instant;
use smoltcp::wire::{EthernetAddress, HardwareAddress, IpCidr, Ipv4Address, Ipv4Cidr};
use spin::{lock, Mutex};
use time::{now, Duration};
use x86_64::instructions::interrupts::without_interrupts;

/// INITIAL_WORKLOADS can be a set of thread ids that
/// should be resumed once we have a DHCP configuration.
///
/// When we next get a DHCP configuration, INITIAL_WORKLOADS
/// is iterated through, wich each thread resumed and
/// then removed from INITIAL_WORKLOADS.
///
static INITIAL_WORKLOADS: Mutex<Vec<Waker>> = Mutex::new(Vec::new());

/// Ensures that `waker` will be awoken when a DHCP
/// configuration is next negotiated.
///
pub fn register_workload(waker: Waker) {
    without_interrupts(|| {
        let mut workloads = lock!(INITIAL_WORKLOADS);

        // If we already have a configuration, we
        // can just fire the waker now. We still
        // lock INITIAL_WORKLOADS first to avoid
        // a race condition.
        for iface in lock!(INTERFACES).iter() {
            if iface.config.is_some() {
                waker.wake();
                return;
            }
        }

        workloads.push(waker);
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
    #[allow(clippy::missing_panics_doc)] // Will only panic if the network stack misbehaves.
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
                    let mut workloads = lock!(INITIAL_WORKLOADS);
                    for waker in workloads.drain(..) {
                        waker.wake();
                    }
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
            let ifaces = lock!(INTERFACES);
            let iface = ifaces.get(self.0).expect("invalid interface handle");
            iface.name.clone()
        })
    }

    /// Returns the current DHCP configuration, if one has been
    /// established.
    ///
    pub fn dhcp_config(&self) -> Option<Dhcpv4Config> {
        without_interrupts(|| {
            let ifaces = lock!(INTERFACES);
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
            let mut ifaces = lock!(INTERFACES);
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
        let mut ifaces = lock!(INTERFACES);
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
