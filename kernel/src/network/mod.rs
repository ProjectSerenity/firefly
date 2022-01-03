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

// TODO: Make the smoltcp Interface generic over devices, rather than specialising to drivers::virtio::network::Device.

use crate::drivers::virtio::network;
use crate::multitasking::thread::ThreadId;
use crate::{println, time};
use alloc::vec::Vec;
use managed::ManagedSlice;
use smoltcp;
use smoltcp::iface::SocketHandle;
use smoltcp::socket::{Dhcpv4Config, Dhcpv4Event, Dhcpv4Socket};
use smoltcp::time::Instant;
use smoltcp::wire::{IpCidr, Ipv4Address, Ipv4Cidr};
use x86_64::instructions::interrupts;

/// INITIAL_WORKLOADS can be a set of thread ids that
/// should be resumed once we have a DHCP configuration.
///
/// When we next get a DHCP configuration, INITIAL_WORKLOADS
/// is iterated through, wich each thread resumed and
/// then removed from INITIAL_WORKLOADS.
///
static INITIAL_WORKLOADS: spin::Mutex<Vec<ThreadId>> = spin::Mutex::new(Vec::new());

/// Ensures that `thread_id` will be [resumed](crate::multitasking::thread::scheduler::resume)
/// when a DHCP configuration is next negotiated.
///
/// `thread_id` will be resumed at most once.
///
pub fn register_workload(thread_id: ThreadId) {
    interrupts::without_interrupts(|| {
        let mut workloads = INITIAL_WORKLOADS.lock();
        workloads.push(thread_id);
    });
}

/// INTERFACES is the list of network interfaces.
///
static INTERFACES: spin::Mutex<Vec<Interface>> = spin::Mutex::new(Vec::new());

/// Represents a network interface, which can be used to
/// send and receive packets.
///
pub struct Interface {
    // iface is the underlying Smoltcp Interface.
    iface: smoltcp::iface::Interface<'static, network::Device>,

    // dhcp is the socket handle to the DHCP socket.
    dhcp: SocketHandle,

    // config is our current DHCP configuration, if any.
    config: Option<Dhcpv4Config>,
}

impl Interface {
    fn new(iface: smoltcp::iface::Interface<'static, network::Device>, dhcp: SocketHandle) -> Self {
        let config = None;
        Interface {
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
    pub fn poll(&mut self) -> time::Duration {
        let now = Instant::from_micros(time::now().system_micros() as i64);
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

                        if addrs.len() == 0 {
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
        let duration = match next {
            Some(duration) => time::Duration::from_micros(duration.micros()),
            None => time::Duration::from_secs(1),
        };

        duration
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

    /// Returns the current DHCP configuration, if one has been
    /// established.
    ///
    pub fn dhcp_config(&self) -> Option<Dhcpv4Config> {
        interrupts::without_interrupts(|| {
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
    pub fn poll(&self) -> time::Duration {
        interrupts::without_interrupts(|| {
            let mut ifaces = INTERFACES.lock();
            let iface = ifaces.get_mut(self.0).expect("invalid interface handle");
            iface.poll()
        })
    }
}

/// Registers a new network interface, returning a unique
/// interface handle.
///
pub fn register_interface(
    mut iface: smoltcp::iface::Interface<'static, network::Device>,
) -> InterfaceHandle {
    // Enable DHCP.
    let socket = Dhcpv4Socket::new();
    let dhcp = iface.add_socket(socket);

    interrupts::without_interrupts(|| {
        let mut ifaces = INTERFACES.lock();
        let handle = InterfaceHandle::new(ifaces.len());
        ifaces.push(Interface::new(iface, dhcp));

        handle
    })
}
