// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements VirtIO [SCSI host devices](https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html#x1-297001r285).
//!
//! A SCSI host device can be used to send and receive
//! SCSI commands to one or more devices.

use crate::features::Reserved;
use crate::{transports, Buffer, Driver, InterruptStatus, Transport};
use alloc::boxed::Box;
use alloc::collections::BTreeMap;
use alloc::string::String;
use alloc::sync::Arc;
use alloc::vec::Vec;
use core::cmp::min;
use core::{ptr, slice};
use interrupts::{register_irq, Irq};
use memory::constants::{PHYSICAL_MEMORY, PHYSICAL_MEMORY_OFFSET};
use memory::{phys_to_virt_addr, virt_to_phys_addrs, PhysAddr, PhysFrame, PhysFrameSize, VirtAddr};
use multitasking::thread::{current_kernel_thread_id, prevent_next_sleep, suspend, KernelThreadId};
use physmem::{allocate_phys_frame, deallocate_phys_frame};
use scsi::{
    parse_sense_data, AdditionalSense, CommandDescriptorBlock, Inquiry, PageCode,
    PeripheralDeviceType, Read16, ReadCapacity16, ReportLuns, SelectReport, SenseKey,
    TestUnitReady, Write16,
};
use serial::println;
use spin::{lock, Mutex};
use storage::block::{add_device, Device, Error, Operations};
use x86_64::instructions::interrupts::without_interrupts;
use x86_64::structures::idt::InterruptStackFrame;

/// CONTROL_VIRTQUEUE is the virtqueue used
/// for sending SCSI task management commands
/// to a VirtIO SCSI host device.
///
#[allow(dead_code)]
const CONTROL_VIRTQUEUE: u16 = 0;

/// EVENT_VIRTUQUE is the virtqueue used to
/// receive notification events from a VirtIO
/// SCSI host device.
///
#[allow(dead_code)]
const EVENT_VIRTQUEUE: u16 = 1;

/// REQUEST_VIRTQUEUE is the virtqueue used
/// for sending SCSI commands to a VirtIO SCSI
/// host device.
///
const REQUEST_VIRTQUEUE: u16 = 2;

/// Maps IRQs to the drivers that produce them.
///
/// When we receive interrupts, we receive requests
/// from the corresponding drivers.
///
static DRIVERS: Mutex<[Option<Arc<Mutex<Driver>>>; 16]> = {
    const NONE: Option<Arc<Mutex<Driver>>> = Option::None;
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

/// Receives interrupts for SCSI host devices and resumes
/// the corresponding thread.
///
fn interrupt_handler(_stack_frame: InterruptStackFrame, irq: Irq) {
    let drivers = lock!(DRIVERS);
    if let Some(driver) = &drivers[irq.as_u8() as usize] {
        let mut dev = lock!(driver);
        let status = dev.interrupt_status();
        if !status.contains(InterruptStatus::QUEUE_INTERRUPT) {
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

/// Parses a 64-bit logical unit number and
/// verifies that it is a single-level LUN
/// structure, returning the structure as a
/// VirtIO SCSI LUN.
///
fn localise_lun(base: u64, lun: u64) -> Result<u64, VirtioScsiError> {
    // Set the first byte to 1, the
    // second to the target, the
    // third and fourth to the
    // single-level LUN structure,
    // and the remaining four bytes
    // to zero.

    // A single-level LUN has the top two
    // bits of the first byte unset.
    if lun & 0xc000_0000_0000_0000 != 0 {
        Err(VirtioScsiError::InvalidLUN(lun))
    } else {
        let lun = (lun & 0x00ff_0000_0000_0000) >> 16; // Keep only the LUN in byte 2.
        let base = base & 0xffff_0000_ffff_ffff; // Mask off the old LUN in bytes 3 and 4.
        Ok(base | lun)
    }
}

/// A SCSI task attribute.
///
#[allow(dead_code)]
enum TaskAttribute {
    Simple = 0,
    Ordered = 1,
    Head = 2,
    Aca = 3,
}

/// A SCSI priority value.
///
struct Priority(pub u8);

#[allow(dead_code)]
impl Priority {
    const NONE: Self = Priority(0);
    const MAX: Self = Priority(1);
    const MIN: Self = Priority(255);
}

/// The response code.
///
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub enum ResponseCode {
    Ok = 0,
    Overrun = 1,
    Aborted = 2,
    BadTarget = 3,
    Reset = 4,
    Busy = 5,
    TransportFailure = 6,
    TargetFailure = 7,
    NexusFailure = 8,
    Failure = 9,
}

impl ResponseCode {
    fn from_u8(n: u8) -> Option<Self> {
        match n {
            0 => Some(ResponseCode::Ok),
            1 => Some(ResponseCode::Overrun),
            2 => Some(ResponseCode::Aborted),
            3 => Some(ResponseCode::BadTarget),
            4 => Some(ResponseCode::Reset),
            5 => Some(ResponseCode::Busy),
            6 => Some(ResponseCode::TransportFailure),
            7 => Some(ResponseCode::TargetFailure),
            8 => Some(ResponseCode::NexusFailure),
            9 => Some(ResponseCode::Failure),
            _ => None,
        }
    }
}

/// A SCSI status code.
///
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub enum StatusCode {
    Good = 0x0,
    CheckCondition = 0x2,
    ConditionMet = 0x4,
    Busy = 0x8,
    ReservationConflict = 0x18,
    TaskSetFull = 0x28,
    AcaActive = 0x30,
    TaskAborted = 0x40,
}

impl StatusCode {
    fn from_u8(n: u8) -> Option<Self> {
        match n {
            0x0 => Some(StatusCode::Good),
            0x2 => Some(StatusCode::CheckCondition),
            0x4 => Some(StatusCode::ConditionMet),
            0x8 => Some(StatusCode::Busy),
            0x18 => Some(StatusCode::ReservationConflict),
            0x28 => Some(StatusCode::TaskSetFull),
            0x30 => Some(StatusCode::AcaActive),
            0x40 => Some(StatusCode::TaskAborted),
            _ => None,
        }
    }
}

/// An error encountered while communicating
/// with a SCSI host.
///
#[derive(Clone, PartialEq, Eq, Debug)]
enum VirtioScsiError {
    BadResponse(ResponseCode),
    BadStatus {
        status_code: StatusCode,
        status_qualifier: u16,
        sense_key: Option<SenseKey>,
        sense_data: Option<AdditionalSense>,
    },
    InvalidLUN(u64),
}

/// Represents a VirtIO SCSI host.
///
struct Host {
    // The underlying VirtIO generic driver.
    driver: Arc<Mutex<Driver>>,

    // The sense size.
    sense_size: usize,

    // The command descriptor block size.
    cdb_size: usize,

    // The next command identifier.
    id: u64,

    // The physical memory frame used for
    // request headers.
    frame: PhysFrame,
}

impl Host {
    /// Returns a block device built using the given
    /// VirtIO driver.
    ///
    pub fn new(driver: Arc<Mutex<Driver>>, sense_size: u32, cdb_size: u32) -> Self {
        let sense_size = sense_size as usize;
        let cdb_size = cdb_size as usize;
        let id = 1;
        let frame = allocate_phys_frame(PhysFrameSize::Size4KiB).unwrap();
        Host {
            driver,
            sense_size,
            cdb_size,
            id,
            frame,
        }
    }

    /// Sends a SCSI command and returns the response.
    ///
    fn do_cmd(
        &mut self,
        lun: u64,
        cdb: &dyn CommandDescriptorBlock,
        data_out: Option<&[u8]>,
        data_in: Option<&mut [u8]>,
        sync: bool,
    ) -> Result<usize, VirtioScsiError> {
        // Prepare the header frame.
        let phys = self.frame.start_address();
        let virt = phys_to_virt_addr(phys);

        // Zero the frame.
        unsafe { ptr::write_bytes(virt.as_usize() as *mut u8, 0x00, self.frame.size().bytes()) };

        // Device-readable header.
        let len1 = 19 + self.cdb_size;
        let buf1 = unsafe { slice::from_raw_parts_mut(virt.as_usize() as *mut u8, len1) };

        // Device-writable header.
        let len2 = 12 + self.sense_size;
        let buf2 = unsafe { slice::from_raw_parts((virt + len1).as_usize() as *const u8, len2) };

        // Populate the device-readable header.
        self.id += 1;
        buf1[0..8].copy_from_slice(&lun.to_be_bytes()); // LUN.
        buf1[8..16].copy_from_slice(&self.id.to_le_bytes()); // ID.
        buf1[16] = TaskAttribute::Simple as u8; // Task attribute.
        buf1[17] = Priority::NONE.0; // Priority.
        buf1[18] = 0; // CRN.
        cdb.serialise(&mut buf1[19..(19 + self.cdb_size)]); // CDB.

        // Build the request buffers.
        let mut buffers = Vec::with_capacity(4);

        // Add the device-readable header.
        buffers.push(Buffer::DeviceCanRead {
            addr: phys,
            len: len1,
        });

        // Optionally add data_out.
        if let Some(data_out) = &data_out {
            let virt_addr = unsafe { VirtAddr::new_unchecked(data_out.as_ptr() as usize) };
            if PHYSICAL_MEMORY.contains_addr(virt_addr) {
                let addr = PhysAddr::new(virt_addr - PHYSICAL_MEMORY_OFFSET);
                let len = data_out.len();
                buffers.push(Buffer::DeviceCanRead { addr, len });
            } else {
                let bufs = virt_to_phys_addrs(VirtAddr::range_exclusive(
                    virt_addr,
                    virt_addr + data_out.len(),
                ))
                .expect("failed to resolve physical memory region");

                for buf in bufs.iter() {
                    let addr = buf.start();
                    let len = buf.size();
                    buffers.push(Buffer::DeviceCanRead { addr, len });
                }
            };
        }

        // Add the device-writable header.
        buffers.push(Buffer::DeviceCanWrite {
            addr: phys + len1,
            len: len2,
        });

        // Optionally add data_in.
        if let Some(data_in) = &data_in {
            let virt_addr = unsafe { VirtAddr::new_unchecked(data_in.as_ptr() as usize) };
            if PHYSICAL_MEMORY.contains_addr(virt_addr) {
                let addr = PhysAddr::new(virt_addr - PHYSICAL_MEMORY_OFFSET);
                let len = data_in.len();
                buffers.push(Buffer::DeviceCanWrite { addr, len });
            } else {
                let bufs = virt_to_phys_addrs(VirtAddr::range_exclusive(
                    virt_addr,
                    virt_addr + data_in.len(),
                ))
                .expect("failed to resolve physical memory region");

                for buf in bufs.iter() {
                    let addr = buf.start();
                    let len = buf.size();
                    buffers.push(Buffer::DeviceCanWrite { addr, len });
                }
            };
        }

        // Make sure we're woken up correctly if
        // we're doing asynchronous processing.
        //
        // We either go to sleep until the response
        // is ready or we wait in a loop.
        if sync {
            let mut driver = lock!(self.driver);

            // Disable notifications, as we'll just poll.
            driver.disable_notifications(REQUEST_VIRTQUEUE);

            // Send the buffer to be filled.
            driver.send(REQUEST_VIRTQUEUE, &buffers[..]).unwrap();
            driver.notify(REQUEST_VIRTQUEUE);

            // Wait for the device to return it.
            loop {
                match driver.recv(REQUEST_VIRTQUEUE) {
                    None => {
                        // Do a small busy loop so we don't
                        // hammer the MMIO.
                        for _ in 0..1000 {}

                        continue;
                    }
                    Some(bufs) => {
                        // Check we got the right buffer.
                        let got_addr = match bufs.buffers[0] {
                            Buffer::DeviceCanRead { addr, len: _len } => addr,
                            _ => panic!("invalid buffer from scsi device"),
                        };

                        if got_addr != phys {
                            panic!("got unexpected buffer from scsi device");
                        }

                        break;
                    }
                }
            }

            // Re-enable notifications for the next
            // request.
            driver.enable_notifications(REQUEST_VIRTQUEUE);
        } else {
            // Ensure the interrupt handler will resume
            // us when the request is returned.
            let thread_id = current_kernel_thread_id();
            without_interrupts(|| {
                prevent_next_sleep();
                lock!(REQUESTS).insert(phys, thread_id);

                // Send the buffer to be filled.
                let mut driver = lock!(self.driver);
                driver.send(REQUEST_VIRTQUEUE, &buffers[..]).unwrap();
                driver.notify(REQUEST_VIRTQUEUE);
                drop(driver);

                // Suspend while we wait for a response.
                suspend();
            });
        }

        // Read the response.
        let sense_len = u32::from_le_bytes([buf2[0], buf2[1], buf2[2], buf2[3]]);
        let residual = u32::from_le_bytes([buf2[4], buf2[5], buf2[6], buf2[7]]);
        let status_qualifier = u16::from_le_bytes([buf2[8], buf2[9]]);
        let status_code = StatusCode::from_u8(buf2[10]).expect("bad status code");
        let response = ResponseCode::from_u8(buf2[11]).expect("bad response code");
        let sense = &buf2[12..(12 + sense_len as usize)];

        // Calculate data_length so we can subtract
        // residual to determine the number of bytes
        // transferred.
        let data_length = if let Some(data_out) = &data_out {
            data_out.len()
        } else {
            0
        } + if let Some(data_in) = &data_in {
            data_in.len()
        } else {
            0
        };

        if response != ResponseCode::Ok {
            Err(VirtioScsiError::BadResponse(response))
        } else if status_code != StatusCode::Good {
            let (sense_key, sense_data) = parse_sense_data(sense);
            Err(VirtioScsiError::BadStatus {
                status_code,
                status_qualifier,
                sense_key,
                sense_data,
            })
        } else {
            Ok(data_length - residual as usize)
        }
    }

    /// Send a SCSI command with no request data.
    ///
    pub fn recv(
        &mut self,
        lun: u64,
        cdb: &dyn CommandDescriptorBlock,
        data_in: &mut [u8],
        sync: bool,
    ) -> Result<usize, VirtioScsiError> {
        self.do_cmd(lun, cdb, None, Some(data_in), sync)
    }

    /// Send a SCSI command with no response data.
    ///
    pub fn send(
        &mut self,
        lun: u64,
        cdb: &dyn CommandDescriptorBlock,
        data_out: &[u8],
        sync: bool,
    ) -> Result<usize, VirtioScsiError> {
        self.do_cmd(lun, cdb, Some(data_out), None, sync)
    }
}

impl Drop for Host {
    fn drop(&mut self) {
        // Deallocate the frame now we're done with it.
        unsafe { deallocate_phys_frame(self.frame) };
    }
}

/// A SCSI block device.
///
pub struct BlockDevice {
    // The SCSI host that controls
    // this device.
    host: Arc<Mutex<Host>>,

    // The logical unit number for
    // this device.
    lun: u64,

    // The capacity as a number of
    // complete blocks.
    capacity: usize,

    // The block size in bytes.
    block_size: usize,

    // The maximum transfer length.
    max_transfer_length: Option<usize>,

    // The vendor name.
    #[allow(dead_code)]
    vendor: String,

    // The product name.
    #[allow(dead_code)]
    product: String,
}

impl BlockDevice {
    fn new(
        host: Arc<Mutex<Host>>,
        lun: u64,
        capacity: usize,
        block_size: usize,
        max_transfer_length: Option<usize>,
        vendor: String,
        product: String,
    ) -> Self {
        BlockDevice {
            host,
            lun,
            capacity,
            block_size,
            max_transfer_length,
            vendor,
            product,
        }
    }
}

impl Device for BlockDevice {
    /// Returns the number of bytes in each segment.
    ///
    fn segment_size(&self) -> usize {
        self.block_size
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
        self.capacity.saturating_mul(self.block_size)
    }

    /// Returns the set of operations supported by the
    /// device.
    ///
    /// If an unsupported operation is attempted, it
    /// will return [`Error::NotSupported`].
    ///
    fn operations(&self) -> Operations {
        // TODO: Work out the set of supported operations.
        Operations::READ | Operations::WRITE
    }

    /// Populates a byte slice with data from the device.
    ///
    /// `segment` indicates from which segment the data
    /// should be read. The data read will start at the
    /// offset `segment` * [`segment_size`](Self::segment_size).
    ///
    /// Note that `buf` must have a length that is an exact
    /// multiple the [`segment_size`](Self::segment_size).
    ///
    /// `read` returns the number of bytes read.
    ///
    fn read(&mut self, segment: usize, buf: &mut [u8]) -> Result<usize, Error> {
        if buf.len() % self.block_size != 0 {
            return Err(Error::InvalidBuffer);
        }

        let blocks = buf.len() / self.block_size;
        let transfer_length = if let Some(max_transfer_length) = self.max_transfer_length {
            min(blocks, max_transfer_length) as u32
        } else {
            blocks as u32
        };

        let cdb = Read16::new(segment as u64, transfer_length);
        lock!(self.host)
            .recv(self.lun, &cdb, buf, false)
            .map_err(|_| Error::DeviceError)
    }

    /// Writes data from a byte slice to the device.
    ///
    /// `segment` indicates from which segment the data
    /// should be read. The data written will start at the
    /// offset `segment` * [`segment_size`](Self::segment_size).
    ///
    /// Note that `buf` must have a length that is an exact
    /// multiple the [`segment_size`](Self::segment_size).
    ///
    /// `write` returns the number of bytes written.
    ///
    /// If the device is read-only, calls to `write` will
    /// return [`Error::NotSupported`].
    ///
    fn write(&mut self, segment: usize, buf: &mut [u8]) -> Result<usize, Error> {
        if buf.len() % self.block_size != 0 {
            return Err(Error::InvalidBuffer);
        }

        let blocks = buf.len() / self.block_size;
        let transfer_length = if let Some(max_transfer_length) = self.max_transfer_length {
            min(blocks, max_transfer_length) as u32
        } else {
            blocks as u32
        };

        let cdb = Write16::new(segment as u64, transfer_length);
        lock!(self.host)
            .send(self.lun, &cdb, buf, false)
            .map_err(|_| Error::DeviceError)
    }

    /// Flush the buffered data at the given `segment`.
    ///
    /// `segment` indicates from which segment the data
    /// should be flushed. The data flushed will start at the
    /// offset `segment` * [`segment_size`](Self::segment_size).
    ///
    fn flush(&mut self, _segment: usize) -> Result<(), Error> {
        unimplemented!();
    }
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
                println!("Ignoring SCSI device: bad legacy PCI transport: {:?}.", err);
                return;
            }
            Ok(transport) => Arc::new(transport) as Arc<dyn Transport>,
        }
    } else {
        match transports::pci::Transport::new(device) {
            Err(err) => {
                println!("Ignoring SCSI device: bad PCI transport: {:?}.", err);
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
        (Reserved::RING_EVENT_IDX | Reserved::ANY_LAYOUT).bits()
    } else {
        Reserved::RING_EVENT_IDX.bits()
    };

    let driver = match Driver::new(transport, must_features, like_features, 3, legacy) {
        Ok(driver) => driver,
        Err(err) => {
            println!("Failed to initialise SCSI device: {:?}.", err);
            return;
        }
    };

    // Fetch the SCSI host's configuration.

    let sense_size = u32::from_le_bytes([
        driver.read_device_config_u8(20),
        driver.read_device_config_u8(21),
        driver.read_device_config_u8(22),
        driver.read_device_config_u8(23),
    ]);

    let cdb_size = u32::from_le_bytes([
        driver.read_device_config_u8(24),
        driver.read_device_config_u8(25),
        driver.read_device_config_u8(26),
        driver.read_device_config_u8(27),
    ]);

    let max_target = u16::from_le_bytes([
        driver.read_device_config_u8(30),
        driver.read_device_config_u8(31),
    ]);

    let max_lun = u32::from_le_bytes([
        driver.read_device_config_u8(32),
        driver.read_device_config_u8(33),
        driver.read_device_config_u8(34),
        driver.read_device_config_u8(35),
    ]);

    // These values must actually be smaller.
    let max_target = max_target as u8;
    let max_lun = max_lun as u16;
    println!(
        "Found SCSI host with up to {} targets and up to {} LUNs.",
        max_target as usize + 1,
        max_lun as usize + 1
    );

    let irq = driver.irq();
    let handle = Arc::new(Mutex::new(driver));
    let host = Host::new(handle.clone(), sense_size, cdb_size);
    let host_handle = Arc::new(Mutex::new(host));
    let mut host = lock!(host_handle);

    // Scan for the set of logical units.
    // We always scan for the REPORT LUNS
    // well-known logical unit, plus each
    // target up to and including the max
    // target.
    let mut luns = Vec::new();
    let mut bases = [0u64; 1 + 256];
    bases[0] = 0xc101000000000000; // REPORT LUNS well-known logical unit.
    for target in 0..=max_target {
        bases[target as usize + 1] = 0x0100000000000000 | ((target as u64) << 48);
    }

    const DATA_LEN: u32 = 72;
    let mut data = [0u8; DATA_LEN as usize];
    let cdb = ReportLuns::new(SelectReport::All, DATA_LEN);

    for base in bases[0..=(max_target as usize + 1)].iter() {
        let data_len = match host.recv(*base, &cdb, &mut data, true) {
            Ok(data_len) => data_len,
            Err(VirtioScsiError::BadResponse(ResponseCode::BadTarget)) => {
                continue;
            }
            Err(err) => {
                println!("Failed to scan {:x}: {:?}.", base, err);
                continue;
            }
        };

        let res = &data[0..data_len];
        let lun_list_length = u32::from_be_bytes([res[0], res[1], res[2], res[3]]);
        if lun_list_length % 8 != 0 {
            println!(
                "Failed to scan {:x}: got LUN list length {}: not a multiple of 8.",
                base, lun_list_length
            );
            continue;
        }

        for i in (0..(lun_list_length as usize)).step_by(8) {
            let lun = u64::from_be_bytes([
                res[8 + i * 8],
                res[9 + i * 8],
                res[10 + i * 8],
                res[11 + i * 8],
                res[12 + i * 8],
                res[13 + i * 8],
                res[14 + i * 8],
                res[15 + i * 8],
            ]);

            let lun = match localise_lun(*base, lun) {
                Ok(lun) => lun,
                Err(_) => {
                    println!(
                        "Ignoring unacceptable logical unit number {} from scanning {:x}.",
                        lun, base
                    );
                    continue;
                }
            };

            luns.push(lun);
        }
    }

    if luns.is_empty() {
        println!("Failed to identify any devices on SCSI host.");
    } else {
        // Identify the device type for each logical
        // unit we've identified.
        const DATA_LEN: u16 = 96;
        let mut data = [0u8; DATA_LEN as usize];
        let cdb = Inquiry::new(None, DATA_LEN);

        for lun in luns.iter() {
            let data_len = match host.recv(*lun, &cdb, &mut data, true) {
                Ok(data_len) => data_len,
                Err(err) => {
                    println!(
                        "Failed to identify device type for logical unit {:x}: {:?}.",
                        lun, err
                    );
                    continue;
                }
            };

            let res = &data[0..data_len];
            let response_data_format = res[3] & 0b1111;
            if response_data_format != 2 {
                println!("Ignoring SCSI device at {:x} with unrecognised response data format {} (want {})", lun, response_data_format, 2);
                continue;
            }

            // Extract the branding info. We filter
            // for ASCII characters, so we can call
            // String::from_utf8().unwrap() safely.
            let vendor = String::from_utf8(
                res[8..16]
                    .trim_ascii_end()
                    .to_vec()
                    .iter()
                    .filter(|b| b.is_ascii())
                    .copied()
                    .collect(),
            )
            .unwrap();
            let product = String::from_utf8(
                res[16..32]
                    .trim_ascii_end()
                    .to_vec()
                    .iter()
                    .filter(|b| b.is_ascii())
                    .copied()
                    .collect(),
            )
            .unwrap();

            // Switch on the device type.
            match PeripheralDeviceType::from_u8(res[0]) {
                Some(PeripheralDeviceType::DirectAccessBlockDevice) => {
                    // Check that the device is ready.
                    //
                    // Note that we reuse the existing data buffer.
                    let mut error = None;
                    for _i in 0..10 {
                        let cdb = TestUnitReady::new();
                        match host.recv(*lun, &cdb, &mut data, true) {
                            Ok(_) => {
                                error = None;
                                break;
                            }
                            Err(VirtioScsiError::BadStatus {
                                status_code: StatusCode::CheckCondition,
                                status_qualifier: _,
                                sense_key: _,
                                sense_data:
                                    Some(AdditionalSense::PowerOnResetOrBusDeviceResetOccurred),
                            }) => {
                                // We know we're just waiting for
                                // the device to get ready, so we
                                // just wait.
                                continue;
                            }
                            Err(err) => {
                                error = Some(err);
                            }
                        }
                    }

                    if let Some(err) = error {
                        println!(
                            "Giving up on {} {} at {:x}: took too long to become ready. Last error was {:?}",
                            vendor, product, lun, err
                        );
                        continue;
                    }

                    // Determine the device's capacity and logical
                    // block size.
                    //
                    // Note that we reuse the existing data buffer.
                    let cdb = ReadCapacity16::new(DATA_LEN as u32);
                    let data_len = match host.recv(*lun, &cdb, &mut data, true) {
                        Ok(data_len) => data_len,
                        Err(err) => {
                            println!(
                                "Failed to identify device capacity for {} {} at {:x}: {:?}.",
                                vendor, product, lun, err
                            );
                            continue;
                        }
                    };

                    let res = &data[0..data_len];
                    let capacity = u64::from_be_bytes([
                        res[0], res[1], res[2], res[3], res[4], res[5], res[6], res[7],
                    ])
                    .saturating_add(1) as usize;

                    let block_size =
                        u32::from_be_bytes([res[8], res[9], res[10], res[11]]) as usize;

                    // Attempt to determine the block limits.
                    let cbd = Inquiry::new(Some(PageCode::BlockLimits), DATA_LEN);
                    let max_transfer_length = match host.recv(*lun, &cbd, &mut data, true) {
                        Ok(data_len) => {
                            let res = &data[0..data_len];
                            let max_transfer_length =
                                u32::from_be_bytes([res[8], res[9], res[10], res[11]]);

                            Some(max_transfer_length as usize)
                        }
                        Err(_) => None,
                    };

                    println!(
                        "Found SCSI block device {} {} with {} {}-byte blocks.",
                        vendor, product, capacity, block_size
                    );

                    // Register the device.
                    let device = BlockDevice::new(
                        host_handle.clone(),
                        *lun,
                        capacity,
                        block_size,
                        max_transfer_length,
                        vendor,
                        product,
                    );

                    add_device(Box::new(device));
                }
                Some(device) => {
                    println!(
                        "Ignoring {:?} SCSI device {} {} at {:x}.",
                        device, vendor, product, lun
                    );
                    continue;
                }
                None => {
                    println!(
                        "Ignoring unrecognised SCSI device {} {} at {:x}.",
                        vendor, product, lun
                    );
                    continue;
                }
            }
        }
    }

    // Prepare the SCSI driver.
    without_interrupts(|| {
        let mut dev = lock!(DRIVERS);
        dev[irq.as_usize()] = Some(handle);
    });

    if false {
        register_irq(irq, interrupt_handler);
    }
}
