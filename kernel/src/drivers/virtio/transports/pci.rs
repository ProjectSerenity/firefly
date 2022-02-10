// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements the [PCI transport mechanism](https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html#x1-1000001).
//!
//! A PCI [`Device`](pci::Device) can be used to instantiate a PCI
//! [`Transport`], which can then be used to reset and configure a
//! VirtIO device.
//!
//! # Examples
//!
//! ```
//! fn install_pci_device(device: pci::Device) {
//!     let _driver = Transport::new(device).unwrap();
//! }
//! ```

use crate::drivers::virtio;
use crate::drivers::virtio::{DeviceStatus, InterruptStatus};
use crate::interrupts::Irq;
use crate::memory::mmio;
use crate::memory::mmio::{read_volatile, write_volatile};
use pci;
use x86_64::structures::paging::frame::{PhysFrame, PhysFrameRange};
use x86_64::PhysAddr;

/// CAPABILITY_ID_VENDOR is the unique identifier
/// for a PCI capability containing vendor-specific
/// data.
///
/// All VirtIO configuration data is provided in
/// PCI capabilities with this id.
///
const CAPABILITY_ID_VENDOR: u8 = 0x09;

/// Type represents a virtio configuration type.
///
#[derive(Debug)]
#[allow(clippy::upper_case_acronyms)]
enum Type {
    // Common indicates the configuration common
    // to all virtio devices.
    Common = 1,

    // Notify indicates notification configuration.
    Notify = 2,

    // Interrupt indicates the insterrupt handler
    // status configuration.
    Interrupt = 3,

    // Device indicates device-specific configuration.
    Device = 4,

    // PCI indicates PCI access configuration.
    PCI = 5,
}

impl Type {
    /// from_u8 returns the matching config type,
    /// if any.
    ///
    pub fn from_u8(v: u8) -> Option<Self> {
        match v {
            1 => Some(Self::Common),
            2 => Some(Self::Notify),
            3 => Some(Self::Interrupt),
            4 => Some(Self::Device),
            5 => Some(Self::PCI),
            _ => None,
        }
    }
}

/// bar_frame_range returns the PhysFrameRange
/// that describes the physical address space
/// at the offset and size from the chosen base
/// address register.
///
fn bar_frame_range(
    device: &pci::Device,
    bar: u8,
    offset: u32,
    length: u32,
) -> Option<PhysFrameRange> {
    // We pick out the BAR, then determine the first
    // address at the given offset and the first address
    // after the range. Both addresses should be on
    // frame boundaries, as that means we can be sure
    // that there's no risk over overlapping with
    // another region.
    if length == 0 || bar as usize >= device.base_address_registers.len() {
        return None;
    }

    let base = match device.bar(bar as usize) {
        pci::Bar::MemoryMapped { addr } => addr,
        _ => {
            return None;
        }
    };

    let start_addr = base + offset as u64;
    let start_frame = PhysFrame::from_start_address(start_addr);
    if start_frame.is_err() {
        return None;
    }

    let end_addr = start_addr + length as u64;
    let end_frame = PhysFrame::from_start_address(end_addr);
    if end_frame.is_err() {
        return None;
    }

    Some(PhysFrame::range(start_frame.unwrap(), end_frame.unwrap()))
}

/// Describes a VirtIO PCI transport that is for
/// some reason unacceptable.
///
#[derive(Debug)]
pub enum ConfigError {
    /// NoCommon indicates there was no common
    /// configuration.
    NoCommon,

    /// NoNotify indicates there was no notify
    /// configuration.
    NoNotify,

    /// NoInterrupt indicates there was no interrupt
    /// configuration.
    NoInterrupt,

    /// NoDevice indicates there was no device
    /// configuration.
    NoDevice,
}

/// CommonConfig is a helper type that gives the
/// layout of the common config type defined in
/// section 4.1.4.3:
///
/// ```
/// struct virtio_pci_common_cfg {
///     /* About the whole device. */
///     le32 device_feature_select;     /* read-write */
///     le32 device_feature;            /* read-only for driver */
///     le32 driver_feature_select;     /* read-write */
///     le32 driver_feature;            /* read-write */
///     le16 msix_config;               /* read-write */
///     le16 num_queues;                /* read-only for driver */
///     u8 device_status;               /* read-write */
///     u8 config_generation;           /* read-only for driver */
///
///     /* About a specific virtqueue. */
///     le16 queue_select;              /* read-write */
///     le16 queue_size;                /* read-write */
///     le16 queue_msix_vector;         /* read-write */
///     le16 queue_enable;              /* read-write */
///     le16 queue_notify_off;          /* read-only for driver */
///     le64 queue_desc;                /* read-write */
///     le64 queue_driver;              /* read-write */
///     le64 queue_device;              /* read-write */
/// };
/// ```
///
#[repr(C, packed)]
#[derive(Clone, Copy)]
struct CommonConfig {
    device_feature_select: u32,
    device_feature: u32,
    driver_feature_select: u32,
    driver_feature: u32,
    msix_config: u16,
    num_queues: u16,
    device_status: u8,
    config_generation: u8,

    queue_select: u16,
    queue_size: u16,
    queue_msix_vector: u16,
    queue_enable: u16,
    queue_notify_off: u16,
    queue_desc_lo: u32,
    queue_desc_hi: u32,
    queue_driver_lo: u32,
    queue_driver_hi: u32,
    queue_device_lo: u32,
    queue_device_hi: u32,
}

/// Implements VirtIO configuration for the PCI transport.
///
pub struct Transport {
    // pci is a handle to the PCI device.
    //
    pci: pci::Device,

    // common implements the configuration to all
    // virtio devices.
    common: mmio::Region,

    // notify implements notification configuratin.
    notify: mmio::Region,
    notify_offset_multiplier: u64,

    // interrupt implements the interrupt handler
    // status configuration.
    interrupt: mmio::Region,

    // device implements device-specific
    // configuration.
    pub device: mmio::Region,
}

impl Transport {
    /// Instantiate a PCI transport using the given device.
    ///
    /// `new` iterates through the given PCI capabilities,
    /// parsing the virtio-related structures and returning
    /// them.
    ///
    pub fn new(device: pci::Device) -> Result<Self, ConfigError> {
        let mut common: Option<PhysFrameRange> = None;
        let mut notify: Option<PhysFrameRange> = None;
        let mut notify_off_multiplier: Option<u64> = None;
        let mut interrupt: Option<PhysFrameRange> = None;
        let mut device_spec: Option<PhysFrameRange> = None;

        // For now, we just take the first instance of
        // each configuration type we see. In future,
        // we should do proper feature detection to
        // check we only choose configurations we can
        // support.
        for capability in device.capabilities.iter() {
            if capability.id != CAPABILITY_ID_VENDOR {
                continue;
            }

            if capability.data.len() < 13 {
                continue;
            }

            let cfg_type = capability.data[0];
            let bar = capability.data[1];
            if bar > 5 {
                // Invalid.
                continue;
            }

            let offset = u32::from_le_bytes([
                capability.data[5],
                capability.data[6],
                capability.data[7],
                capability.data[8],
            ]);
            let length = u32::from_le_bytes([
                capability.data[9],
                capability.data[10],
                capability.data[11],
                capability.data[12],
            ]);

            match Type::from_u8(cfg_type) {
                Some(Type::Common) => {
                    common = bar_frame_range(&device, bar, offset, length);
                }
                Some(Type::Notify) => {
                    notify = bar_frame_range(&device, bar, offset, length);
                    if notify.is_some() && capability.data.len() >= 17 {
                        notify_off_multiplier = Some(u32::from_le_bytes([
                            capability.data[13],
                            capability.data[14],
                            capability.data[15],
                            capability.data[16],
                        ]) as u64);
                    }
                }
                Some(Type::Interrupt) => {
                    interrupt = bar_frame_range(&device, bar, offset, length);
                }
                Some(Type::Device) => {
                    device_spec = bar_frame_range(&device, bar, offset, length);
                }
                Some(Type::PCI) => {
                    // We don't support this yet.
                }
                None => {}
            }
        }

        match (
            common,
            notify,
            notify_off_multiplier,
            interrupt,
            device_spec,
        ) {
            (None, _, _, _, _) => Err(ConfigError::NoCommon),
            (_, None, _, _, _) => Err(ConfigError::NoNotify),
            (_, _, None, _, _) => Err(ConfigError::NoNotify),
            (_, _, _, None, _) => Err(ConfigError::NoInterrupt),
            (_, _, _, _, None) => Err(ConfigError::NoDevice),
            (
                Some(common),
                Some(notify),
                Some(notify_off_multiplier),
                Some(interrupt),
                Some(device_spec),
            ) => {
                device.enable_bus_master();
                unsafe {
                    Ok(Transport {
                        pci: device,
                        common: mmio::Region::map(common),
                        notify: mmio::Region::map(notify),
                        notify_offset_multiplier: notify_off_multiplier,
                        interrupt: mmio::Region::map(interrupt),
                        device: mmio::Region::map(device_spec),
                    })
                }
            }
        }
    }

    // common returns the mutable common config, which
    // can be used to read or write the configuration.
    //
    fn common(&self) -> &'static mut CommonConfig {
        self.common
            .as_mut::<CommonConfig>(0)
            .expect("invalid config address space")
    }
}

impl virtio::Transport for Transport {
    /// read_device_config_u8 returns the device-specific
    /// configuration byte at the given offset.
    ///
    fn read_device_config_u8(&self, offset: u16) -> u8 {
        self.device
            .read(offset as u64)
            .expect("failed to read device configuration")
    }

    /// read_irq returns the device's interrupt number.
    ///
    fn read_irq(&self) -> Irq {
        Irq::new(self.pci.interrupt_line).expect("bad IRQ")
    }

    /// read_interrupt_status returns the device's current
    /// interrupt status.
    ///
    fn read_interrupt_status(&self) -> InterruptStatus {
        InterruptStatus::from_bits_truncate(self.interrupt.read::<u8>(0).unwrap())
    }

    /// read_status returns the device's status.
    ///
    fn read_status(&self) -> DeviceStatus {
        let common = self.common();

        DeviceStatus::from_bits_truncate(unsafe { read_volatile!(common.device_status) })
    }

    /// write_status sets the device's status.
    ///
    fn write_status(&self, device_status: DeviceStatus) {
        let common = self.common();

        unsafe { write_volatile!(common.device_status, device_status.bits()) };
    }

    /// add_status reads the current device status
    /// and sets the given additional bits.
    ///
    fn add_status(&self, device_status: DeviceStatus) {
        let current = self.read_status();
        self.write_status(current | device_status);
    }

    /// has_status returns whether the current device
    /// status includes the given bits.
    ///
    fn has_status(&self, device_status: DeviceStatus) -> bool {
        let current = self.read_status();
        current.contains(device_status)
    }

    /// read_device_features returns the
    /// first 64 of the device's feature bits.
    ///
    fn read_device_features(&self) -> u64 {
        let common = self.common();

        let low;
        let high;
        unsafe {
            write_volatile!(common.device_feature_select, 0u32.to_le());
            low = u32::from_le(read_volatile!(common.device_feature));
            write_volatile!(common.device_feature_select, 1u32.to_le());
            high = u32::from_le(read_volatile!(common.device_feature));
        }

        ((high as u64) << 32) | (low as u64)
    }

    /// write_driver_features sets the
    /// first 64 of the driver's feature bits.
    ///
    fn write_driver_features(&self, features: u64) {
        let common = self.common();

        unsafe {
            write_volatile!(common.driver_feature_select, 0u32.to_le());
            write_volatile!(common.driver_feature, features.to_le() as u32);
            write_volatile!(common.driver_feature_select, 1u32.to_le());
            write_volatile!(common.driver_feature, (features.to_le() >> 32) as u32);
        }
    }

    /// read_num_queues returns the maximum number of
    /// virtqueues supported by the device.
    ///
    fn read_num_queues(&self) -> u16 {
        let common = self.common();

        unsafe { u16::from_le(read_volatile!(common.num_queues)) }
    }

    /// select_queue sets the current virtqueue.
    ///
    fn select_queue(&self, index: u16) {
        let common = self.common();

        unsafe {
            write_volatile!(common.queue_select, index.to_le());
        }
    }

    /// queue_size returns the maximum number of descriptors
    /// supported by the device in any virtqueue.
    ///
    fn queue_size(&self) -> u16 {
        let common = self.common();

        unsafe { u16::from_le(read_volatile!(common.queue_size)) }
    }

    /// set_queue_size notifies the device of the number
    /// of descriptors in the descriptor area of the
    /// current virtqueue (set using select_queue).
    ///
    fn set_queue_size(&self, size: u16) {
        let common = self.common();

        unsafe {
            write_volatile!(common.queue_size, size.to_le());
        }
    }

    /// notify_queue notifies the device that the
    /// virtqueue at the given index has descriptors
    /// ready in the driver area.
    ///
    fn notify_queue(&self, queue_index: u16) {
        let common = self.common();

        unsafe {
            let offset = self.notify_offset_multiplier
                * u16::from_le(read_volatile!(common.queue_notify_off)) as u64;
            *self.notify.as_mut(offset).unwrap() = queue_index;
        }
    }

    /// enable_queue notifies the device to use the
    /// current virtqueue (set using select_queue).
    ///
    fn enable_queue(&self) {
        let common = self.common();

        unsafe {
            write_volatile!(common.queue_enable, 1u16.to_le());
        }
    }

    /// set_queue_descriptor_area notifies the device of
    /// the physical address of the descriptor area of
    /// the current virtqueue (set using select_queue).
    ///
    fn set_queue_descriptor_area(&self, area: PhysAddr) {
        let common = self.common();
        let value = area.as_u64().to_le();

        unsafe {
            write_volatile!(common.queue_desc_lo, value as u32);
            write_volatile!(common.queue_desc_hi, (value >> 32) as u32);
        }
    }

    /// set_queue_driver_area notifies the device of the
    /// physical address of the driver area of the current
    /// virtqueue (set using select_queue).
    ///
    fn set_queue_driver_area(&self, area: PhysAddr) {
        let common = self.common();
        let value = area.as_u64().to_le();

        unsafe {
            write_volatile!(common.queue_driver_lo, value as u32);
            write_volatile!(common.queue_driver_hi, (value >> 32) as u32);
        }
    }

    /// set_queue_device_area notifies the device of the
    /// physical address of the device area of the current
    /// virtqueue (set using select_queue).
    ///
    fn set_queue_device_area(&self, area: PhysAddr) {
        let common = self.common();
        let value = area.as_u64().to_le();

        unsafe {
            write_volatile!(common.queue_device_lo, value as u32);
            write_volatile!(common.queue_device_hi, (value >> 32) as u32);
        }
    }
}
