// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements VirtIO [entropy source devices](https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html#x1-2700004).
//!
//! An entropy source can be used to receive cryptographically secure pseudo-random
//! data from the host. This functions using a single virtqueue, which is used to
//! make device-writable buffers available. These buffers are then filled with
//! entropy and returned to the driver.
//!
//! While VirtIO devices normally use notifications, we only use entropy sources
//! infrequently to seed our [CSPRNG](random). This means there should be
//! no need for concurrent requests for entropy. Taking a sequential approach
//! allows us to retrieving entropy allows us to provide a synchronous API, which
//! is easier to use.
//!
//! # Examples
//!
//! ```
//! let driver = Driver::new(virtio_driver);
//! let mut buf = [0u8; 16];
//! let written = driver.read(&mut buf[..]);
//! let _random_data = buf[..written];
//! ```

use crate::drivers::virtio;
use crate::drivers::virtio::features::Reserved;
use crate::drivers::virtio::{transports, Buffer};
use alloc::boxed::Box;
use alloc::sync::Arc;
use alloc::vec;
use alloc::vec::Vec;
use memlayout::{PHYSICAL_MEMORY, PHYSICAL_MEMORY_OFFSET};
use pci;
use random::{register_entropy_source, EntropySource};
use serial::println;
use virtmem::virt_to_phys_addrs;
use x86_64::{PhysAddr, VirtAddr};

/// REQUEST_VIRTQUEUE is the sole virtqueue used
/// with a virtio entropy device.
///
const REQUEST_VIRTQUEUE: u16 = 0;

/// Represents a virtio entropy device, which can be
/// used to receive random data.
///
pub struct Driver {
    // driver is the underlying virtio generic driver.
    driver: virtio::Driver,
}

impl Driver {
    /// Returns an entropy source built using the given
    /// VirtIO driver.
    ///
    pub fn new(driver: virtio::Driver) -> Self {
        Driver { driver }
    }

    /// Populates a byte slice with entropy from the device.
    /// `read` returns the number of bytes written to the slice.
    ///
    /// Note that `read` may return a number of written bytes
    /// smaller than `buf`'s length. That is, `buf` may not be
    /// completely filled with entropy.
    ///
    pub fn read(&mut self, buf: &mut [u8]) -> usize {
        let virt_addr = unsafe { VirtAddr::new_unsafe(buf.as_ptr() as u64) };
        let (first_addr, buffers) = if PHYSICAL_MEMORY.contains_addr(virt_addr) {
            let addr = PhysAddr::new(virt_addr - PHYSICAL_MEMORY_OFFSET);
            let len = buf.len();
            let bufs = vec![Buffer::DeviceCanWrite { addr, len }];

            (addr, bufs)
        } else {
            let bufs = match virt_to_phys_addrs(virt_addr, buf.len()) {
                None => panic!("failed to resolve physical memory region"),
                Some(bufs) => bufs,
            };

            let addr = bufs[0].addr;
            let bufs = bufs
                .iter()
                .map(|b| Buffer::DeviceCanWrite {
                    addr: b.addr,
                    len: b.len,
                })
                .collect::<Vec<Buffer>>();

            (addr, bufs)
        };

        // Send the buffer to be filled.
        self.driver.send(REQUEST_VIRTQUEUE, &buffers[..]).unwrap();
        self.driver.notify(REQUEST_VIRTQUEUE);

        // Wait for the device to return it.
        loop {
            // Do a small busy loop so we don't
            // hammer the MMIO.
            for _ in 0..1000 {}

            match self.driver.recv(REQUEST_VIRTQUEUE) {
                None => continue,
                Some(bufs) => {
                    // Check we got the right buffer.
                    let got_addr = match bufs.buffers[0] {
                        Buffer::DeviceCanWrite { addr, len: _len } => addr,
                        _ => panic!("invalid buffer from entropy device"),
                    };

                    if got_addr != first_addr {
                        panic!("got unexpected buffer from entropy device");
                    }

                    return bufs.written;
                }
            }
        }
    }
}

impl EntropySource for Driver {
    /// Fills the buffer with entropy from the device.
    ///
    /// Note that unlike [`read`](Driver::read),
    /// `get_entropy` will loop until it has filled the
    /// entire buffer.
    ///
    fn get_entropy(&mut self, buf: &mut [u8; 32]) {
        let mut len = buf.len();
        let mut done = 0;
        while len > 0 {
            let written = self.read(&mut buf[done..]);
            len -= written;
            done += written;
        }
    }
}

/// Takes ownership of the given PCI device to reset and configure
/// a virtio entropy device.
///
pub fn install_pci_device(device: pci::Device) {
    let transport = match transports::pci::Transport::new(device) {
        Err(err) => {
            println!("Ignoring invalid device: {:?}.", err);
            return;
        }
        Ok(transport) => Arc::new(transport),
    };

    let must_features = Reserved::VERSION_1.bits();
    let like_features = 0u64;
    let mut driver = match virtio::Driver::new(transport, must_features, like_features, 1) {
        Ok(driver) => driver,
        Err(err) => {
            println!("Failed to initialise entropy device: {:?}.", err);
            return;
        }
    };

    // We don't use notifications, so disable them.
    driver.disable_notifications(REQUEST_VIRTQUEUE);

    // Prepare the entropy driver.
    let driver = Driver::new(driver);

    // Show that it works.
    register_entropy_source(Box::new(driver));
}
