//! entropy implements virtio entropy devices.

// See section 5.4.

use crate::drivers::virtio::features::Reserved;
use crate::drivers::virtio::{transports, virtqueue};
use crate::drivers::{pci, virtio};
use crate::memory::{kernel_pml4, virt_to_phys_addrs, PHYSICAL_MEMORY, PHYSICAL_MEMORY_OFFSET};
use crate::println;
use alloc::sync::Arc;
use alloc::vec;
use alloc::vec::Vec;
use x86_64::{PhysAddr, VirtAddr};

/// REQUEST_VIRTQUEUE is the sole virtqueue used
/// with a virtio entropy device.
///
const REQUEST_VIRTQUEUE: u16 = 0;

/// Driver represents a virtio entropy device,
/// which can be used to receive random data.
///
pub struct Driver {
    // driver is the underlying virtio generic driver.
    driver: virtio::Driver,
}

impl Driver {
    /// read can be used to populate a byte slice
    /// with entropy. read returns the number of
    /// bytes written.
    ///
    pub fn read(&mut self, buf: &mut [u8]) -> usize {
        let virt_addr = unsafe { VirtAddr::new_unsafe(buf.as_ptr() as u64) };
        let (first_addr, buffers) = if PHYSICAL_MEMORY.contains_addr(virt_addr) {
            let addr = PhysAddr::new(virt_addr - PHYSICAL_MEMORY_OFFSET);
            let len = buf.len();
            let bufs = vec![virtqueue::Buffer::DeviceCanWrite { addr, len }];

            (addr, bufs)
        } else {
            let pml4 = unsafe { kernel_pml4() };
            let bufs = match virt_to_phys_addrs(&pml4, virt_addr, buf.len()) {
                None => panic!("failed to resolve physical memory region"),
                Some(bufs) => bufs,
            };

            let addr = bufs[0].addr;
            let bufs = bufs
                .iter()
                .map(|b| virtqueue::Buffer::DeviceCanWrite {
                    addr: b.addr,
                    len: b.len,
                })
                .collect::<Vec<virtqueue::Buffer>>();

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
                        virtqueue::Buffer::DeviceCanWrite { addr, len: _len } => addr,
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

/// install_device can be used to take ownership of the
/// given PCI device that represents a virtio entropy
/// device.
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
    let mut driver = Driver { driver };

    // Show that it works.
    let mut array = [0u8; 16];
    let buf = &mut array[..];
    println!("RNG before: {} bytes: {:02x?}", buf.len(), buf.to_vec());
    let written = driver.read(buf);
    println!(
        "RNG after:  {} bytes: {:02x?}",
        written,
        buf[0..written].to_vec()
    );
}
