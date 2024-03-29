// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements VirtIO [block devices](https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html#x1-2390002).
//!
//! A block device can be used to read and write data in
//! 512-byte segments. Depending on the device, it may be
//! necessary to flush segment caches manually after any
//! writes.

mod cache;

use crate::features::{Block, Reserved};
use crate::{transports, Buffer, InterruptStatus, Transport};
use alloc::boxed::Box;
use alloc::collections::BTreeMap;
use alloc::sync::Arc;
use alloc::vec;
use alloc::vec::Vec;
use interrupts::{register_irq, Irq};
use memory::constants::{PHYSICAL_MEMORY, PHYSICAL_MEMORY_OFFSET};
use memory::{virt_to_phys_addrs, PhysAddr, VirtAddr};
use multitasking::thread::{current_kernel_thread_id, prevent_next_sleep, suspend, KernelThreadId};
use serial::println;
use spin::{lock, Mutex};
use storage::block::{add_device, Device, Error, Operations};
use x86_64::instructions::interrupts::without_interrupts;
use x86_64::structures::idt::InterruptStackFrame;

/// REQUEST_VIRTQUEUE is the sole virtqueue used
/// with a VirtIO block device.
///
const REQUEST_VIRTQUEUE: u16 = 0;

/// Maps IRQs to the drivers that produce them.
///
/// When we receive interrupts, we receive requests
/// from the corresponding drivers.
///
static DRIVERS: Mutex<[Option<Arc<Mutex<crate::Driver>>>; 16]> = {
    const NONE: Option<Arc<Mutex<crate::Driver>>> = Option::None;
    Mutex::new([NONE; 16])
};

/// Maps the first physical address in a request buffer
/// to the thread id of the thread that made the request.
///
/// When we receive an IRQ for a completed request, we
/// identify the first address, use that to look up the
/// thread that made the request, and resume it, removing
/// the entry from the map.
///
static REQUESTS: Mutex<BTreeMap<PhysAddr, KernelThreadId>> = Mutex::new(BTreeMap::new());

/// Receives interrupts for block devices and resumes
/// the corresponding thread.
///
fn interrupt_handler(_stack_frame: InterruptStackFrame, irq: Irq) {
    let drivers = lock!(DRIVERS);
    if let Some(driver) = &drivers[irq.as_u8() as usize] {
        let mut dev = lock!(driver);
        if !dev
            .interrupt_status()
            .contains(InterruptStatus::QUEUE_INTERRUPT)
        {
            // TODO: Handle configuration changes.
            irq.acknowledge();
            return;
        }

        let mut requests = lock!(REQUESTS);
        loop {
            match dev.recv(REQUEST_VIRTQUEUE) {
                None => {
                    irq.acknowledge();
                    return;
                }
                Some(buf) => {
                    let first_addr = match buf.buffers[0] {
                        Buffer::DeviceCanRead { addr, .. } => addr,
                        _ => panic!("invalid buffer type returned by device"),
                    };

                    if let Some(thread_id) = requests.remove(&first_addr) {
                        thread_id.resume();
                    }
                }
            }
        }
    }

    irq.acknowledge();
}

/// Describes a type of operation, used in a
/// request.
///
enum Type {
    // Read data from the device.
    In = 0,

    // Write data to the device.
    Out = 1,

    // Flush the cache.
    Flush = 4,
}

/// Describes the result of a request to the device.
///
#[derive(Debug)]
enum Status {
    // The operation was successful.
    Ok = 0,

    // An error occurred.
    IoErr = 1,

    // The operation is not supported.
    Unsupported = 2,
}

impl Status {
    /// Convert a u8 into the status value, or `None`.
    ///
    pub fn from_u8(status: u8) -> Option<Self> {
        match status {
            0 => Some(Status::Ok),
            1 => Some(Status::IoErr),
            2 => Some(Status::Unsupported),
            _ => None,
        }
    }
}

/// The size of a segment in the block device. Reads
/// and writes to the device must be an exact multiple
/// of the segment size.
///
const BYTES_PER_SEGMENT: usize = 512;

/// Represents a virtio block device, which can be used
/// to read and write data.
///
pub struct Driver {
    // The underlying virtio generic driver.
    driver: Arc<Mutex<crate::Driver>>,

    // Which operations are supported by the device.
    operations: Operations,

    // The device capacity in segments.
    capacity: usize,

    // The set of buffers used for request headers.
    cache: cache::Allocator,
}

impl Driver {
    /// Returns a block device built using the given
    /// VirtIO driver.
    ///
    pub fn new(
        driver: Arc<Mutex<crate::Driver>>,
        operations: Operations,
        capacity: usize,
        cache: cache::Allocator,
    ) -> Self {
        Driver {
            driver,
            operations,
            capacity,
            cache,
        }
    }

    /// Performs the requested operation.
    ///
    /// Note that `buf` must have a length that is an exact
    /// multiple of [`BYTES_PER_SEGMENT`].
    ///
    fn do_op(&mut self, op: Type, segment: usize, buf: &mut [u8]) -> Result<usize, Error> {
        match op {
            Type::In | Type::Out => {
                if buf.len() % BYTES_PER_SEGMENT != 0 {
                    return Err(Error::InvalidBuffer);
                }
            }
            _ => return Err(Error::NotSupported),
        }

        let virt_addr = unsafe { VirtAddr::new_unchecked(buf.as_ptr() as usize) };
        let mut buffers = if PHYSICAL_MEMORY.contains_addr(virt_addr) {
            let addr = PhysAddr::new(virt_addr - PHYSICAL_MEMORY_OFFSET);
            let len = buf.len();

            vec![Buffer::DeviceCanWrite { addr, len }]
        } else {
            virt_to_phys_addrs(VirtAddr::range_exclusive(virt_addr, virt_addr + buf.len()))
                .expect("failed to resolve physical memory region")
                .iter()
                .map(|range| Buffer::DeviceCanWrite {
                    addr: range.start(),
                    len: range.size(),
                })
                .collect::<Vec<Buffer>>()
        };

        // Build the request.
        let (first_addr, header, trailer) = self.cache.allocate(op as u32, segment as u64);
        buffers.insert(0, header);
        buffers.push(trailer);

        // Ensure the interrupt handler will resume
        // us when the request is returned.
        let thread_id = current_kernel_thread_id();
        without_interrupts(|| {
            prevent_next_sleep();
            lock!(REQUESTS).insert(first_addr, thread_id);

            // Send the buffer to be filled and suspend
            // until the request has been processed.
            let mut driver = lock!(self.driver);
            driver.send(REQUEST_VIRTQUEUE, &buffers[..]).unwrap();
            driver.notify(REQUEST_VIRTQUEUE);
            drop(driver);

            suspend();
        });

        // Return the request header to the cache and
        // store the status code.
        let status = self.cache.deallocate(first_addr);

        match Status::from_u8(status) {
            Some(Status::Ok) => Ok(buf.len()),
            Some(Status::IoErr) => Err(Error::DeviceError),
            Some(Status::Unsupported) => Err(Error::NotSupported),
            None => Err(Error::BadResponse),
        }
    }
}

impl Device for Driver {
    /// Returns the number of bytes in each segment.
    ///
    fn segment_size(&self) -> usize {
        BYTES_PER_SEGMENT
    }

    /// Returns the device capacity as a number of
    /// segments.
    //
    fn num_segments(&self) -> usize {
        self.capacity
    }

    /// Returns the device capacity in bytes.
    ///
    fn capacity(&self) -> usize {
        self.capacity * BYTES_PER_SEGMENT
    }

    /// Returns the set of operations supported by the
    /// device.
    ///
    /// If an unsupported operation is attempted, it
    /// will return [`Error::NotSupported`].
    ///
    fn operations(&self) -> Operations {
        self.operations
    }

    /// Populates a byte slice with data from the device.
    ///
    /// `segment` indicates from which segment the data
    /// should be read. For example, `segment` 0 will return
    /// the data at bytes 0 - 511, `segment` 1 will return
    /// the data at bytes 512 - 1023.
    ///
    /// Note that `buf` must have a length that is an exact
    /// multiple of `512`.
    ///
    fn read(&mut self, segment: usize, buf: &mut [u8]) -> Result<usize, Error> {
        if self.operations.contains(Operations::READ) {
            self.do_op(Type::In, segment, buf)
        } else {
            Err(Error::NotSupported)
        }
    }

    /// Writes a byte slice to the device.
    ///
    /// `segment` indicates to which segment the data
    /// should be written. For example, `segment` 0 will write
    /// the data at bytes 0 - 511, `segment` 1 will write
    /// the data at bytes 512 - 1023.
    ///
    /// Note that `buf` must have a length that is an exact
    /// multiple of `512`.
    ///
    fn write(&mut self, segment: usize, buf: &mut [u8]) -> Result<usize, Error> {
        if self.operations.contains(Operations::WRITE) {
            self.do_op(Type::Out, segment, buf)
        } else {
            Err(Error::NotSupported)
        }
    }

    /// Flush the buffered data at the given `segment`.
    ///
    /// `segment` indicates from which segment the data
    /// should be read. For example, `segment` 0 will flush
    /// the data at bytes 0 - 511, `segment` 1 will flush
    /// the data at bytes 512 - 1023.
    ///
    fn flush(&mut self, segment: usize) -> Result<(), Error> {
        if !self.operations.contains(Operations::FLUSH) {
            return Err(Error::NotSupported);
        }

        // Build the request.
        let (first_addr, header, trailer) = self.cache.allocate(Type::Flush as u32, segment as u64);

        // Ensure the interrupt handler will resume
        // us when the request is returned.
        let thread_id = current_kernel_thread_id();
        without_interrupts(|| {
            prevent_next_sleep();
            lock!(REQUESTS).insert(first_addr, thread_id);

            // Send the buffer to be filled and suspend
            // until the request has been processed.
            let mut driver = lock!(self.driver);
            driver.send(REQUEST_VIRTQUEUE, &[header, trailer]).unwrap();
            driver.notify(REQUEST_VIRTQUEUE);
            drop(driver);

            suspend();
        });

        // Return the request header to the cache and
        // store the status code.
        let status = self.cache.deallocate(first_addr);

        match Status::from_u8(status) {
            Some(Status::Ok) => Ok(()),
            Some(Status::IoErr) => Err(Error::DeviceError),
            Some(Status::Unsupported) => Err(Error::NotSupported),
            None => Err(Error::BadResponse),
        }
    }
}

/// Config is a helper type that gives the layout
/// layout of the device-specific config type for
/// block devices, as defined in section 5.2.4.
///
/// ```
/// struct virtio_blk_config {
///     le64 capacity;
///     le32 size_max;
///     le32 seg_max;
///     struct virtio_blk_geometry {
///         le16 cylinders;
///         u8 heads;
///         u8 sectors;
///     } geometry;
///     le32 blk_size;
///     struct virtio_blk_topology {
///         // # of logical blocks per physical block (log2)
///         u8 physical_block_exp;
///         // offset of first aligned logical block
///         u8 alignment_offset;
///         // suggested minimum I/O size in blocks
///         le16 min_io_size;
///         // optimal (suggested maximum) I/O size in blocks
///         le32 opt_io_size;
///     } topology;
///     u8 writeback;
///     u8 unused0[3];
///     le32 max_discard_sectors;
///     le32 max_discard_seg;
///     le32 discard_sector_alignment;
///     le32 max_write_zeroes_sectors;
///     le32 max_write_zeroes_seg;
///     u8 write_zeroes_may_unmap;
///     u8 unused1[3];
/// };
/// ```
///
#[repr(C, packed)]
#[derive(Clone, Copy)]
struct Config {
    capacity: u64,
    size_max: u32,
    seg_max: u32,
    geometry_cylinders: u16,
    geometry_heads: u8,
    geometry_sectors: u8,
    blk_size: u32,
    topology_physical_block_exp: u8,
    topology_alignment_offset: u8,
    topology_min_io_size: u16,
    topology_opt_io_size: u32,
    writeback: u8,
    unused0: [u8; 3],
    max_discard_sectors: u32,
    max_discard_seg: u32,
    discard_sector_alignment: u32,
    max_write_zeroes_sectors: u32,
    max_write_zeroes_seg: u32,
    write_zeroes_may_unmap: u8,
    unused1: [u8; 3],
}

/// Takes ownership of the given modern PCI device to reset and configure
/// a virtio block device.
///
pub fn install_modern_pci_device(device: pci::Device) {
    install_pci_device(device, false)
}

/// Takes ownership of the given legacy PCI device to reset and configure
/// a virtio block device.
///
pub fn install_legacy_pci_device(device: pci::Device) {
    install_pci_device(device, true)
}

/// Takes ownership of the given PCI device to reset and configure
/// a virtio block device.
///
fn install_pci_device(device: pci::Device, legacy: bool) {
    let transport = if legacy {
        match transports::legacy_pci::Transport::new(device) {
            Err(err) => {
                println!(
                    "Ignoring block device: bad legacy PCI transport: {:?}.",
                    err
                );
                return;
            }
            Ok(transport) => Arc::new(transport) as Arc<dyn Transport>,
        }
    } else {
        match transports::pci::Transport::new(device) {
            Err(err) => {
                println!("Ignoring block device: bad PCI transport: {:?}.", err);
                return;
            }
            Ok(transport) => Arc::new(transport) as Arc<dyn Transport>,
        }
    };

    let must_features = if legacy {
        Reserved::empty().bits()
    } else {
        Reserved::VERSION_1.bits()
    };

    let like_features = if legacy {
        (Reserved::RING_EVENT_IDX | Reserved::ANY_LAYOUT).bits() | (Block::RO | Block::FLUSH).bits()
    } else {
        Reserved::RING_EVENT_IDX.bits() | (Block::RO | Block::FLUSH).bits()
    };

    let driver = match crate::Driver::new(transport, must_features, like_features, 1, legacy) {
        Ok(driver) => driver,
        Err(err) => {
            println!("Failed to initialise block device: {:?}.", err);
            return;
        }
    };

    let features = driver.features();
    let block = Block::from_bits_truncate(features);
    let mut operations = Operations::READ;
    if !block.contains(Block::RO) {
        operations |= Operations::WRITE;
    }
    if block.contains(Block::FLUSH) {
        operations |= Operations::FLUSH;
    }

    // Capacity data is at offset 0 in the Config
    // structure documented above.
    let capacity = u64::from_le_bytes([
        driver.read_device_config_u8(0),
        driver.read_device_config_u8(1),
        driver.read_device_config_u8(2),
        driver.read_device_config_u8(3),
        driver.read_device_config_u8(4),
        driver.read_device_config_u8(5),
        driver.read_device_config_u8(6),
        driver.read_device_config_u8(7),
    ]);

    // Prepare our request header cache.
    let cache = cache::Allocator::new();

    // Prepare the block driver.
    let irq = driver.irq();
    let handle = Arc::new(Mutex::new(driver));
    let driver = Driver::new(handle.clone(), operations, capacity as usize, cache);
    without_interrupts(|| {
        let mut dev = lock!(DRIVERS);
        dev[irq.as_usize()] = Some(handle);
    });

    register_irq(irq, interrupt_handler);

    // Register the device.
    add_device(Box::new(driver));
}
