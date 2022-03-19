// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides a wrapper around the crate's [`Device`] trait.
//!
//! This ensures the wrapped device implements the
//! [`smoltcp::phy::Device`] trait.

use super::Device;
use alloc::boxed::Box;
use alloc::sync::Arc;
use core::slice;
use memlayout::phys_to_virt_addr;
use smoltcp::phy::{DeviceCapabilities, RxToken, TxToken};
use smoltcp::time::Instant;
use spin::{lock, Mutex};
use x86_64::instructions::interrupts::without_interrupts;
use x86_64::PhysAddr;

/// This is our device wrapper which we use to ensure all
/// our network interfaces are generic over the same type
/// (this one). If we instead allow our device drivers to
/// provide their own type, then we can't have a heterogeneous
/// container for them all.
///
pub struct WrappedDevice {
    dev: Arc<Mutex<Box<dyn Device>>>,
}

impl WrappedDevice {
    /// Wrap the given device.
    ///
    pub fn new(dev: Box<dyn Device>) -> Self {
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
            let mut dev = lock!(self.dev);
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
            let dev = lock!(self.dev);
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
pub struct RecvToken {
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
            let mut dev = lock!(self.dev);
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
pub struct SendToken {
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
            let mut dev = lock!(self.dev);
            dev.get_send_buffer(len)
        })?;

        // Pass our buffer to the callback to
        // receive the packet data.
        let virt_addr = phys_to_virt_addr(phys);
        let buf = unsafe { slice::from_raw_parts_mut(virt_addr.as_mut_ptr(), len) };
        let ret = f(buf)?;

        // Send the packet.
        without_interrupts(|| {
            let mut dev = lock!(self.dev);
            dev.send_packet(phys, len)
        })?;

        Ok(ret)
    }
}
