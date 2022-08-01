// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements the legacy [PCI transport mechanism](https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html#x1-1000001).
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

use crate::{DeviceStatus, InterruptStatus};
use interrupts::Irq;
use memory::PhysAddr;
use x86_64::instructions::port::{Port, PortRead, PortWrite};

// Configuration space layout (section 4.1.4.8):
//
// ```
// | Bits       |              32 |              32 |         32 |         16 |           16 |           16 |             8 |          8 |
// | Read/Write |               R |             R+W |        R+W |          R |          R+W |          R+W |           R+W |          R |
// | Purpose    | Device features | Driver features | Queue addr | Queue size | Queue select | Queue notify | Device status | ISR status |
// ```
#[repr(u16)]
#[derive(Clone, Copy, Debug)]
enum Offsets {
    DeviceFeatures = 0,  // u32
    DriverFeatures = 4,  // u32
    QueueAddress = 8,    // u32
    QueueSize = 12,      // u16
    QueueSelect = 14,    // u16
    QueueNotify = 16,    // u16
    DeviceStatus = 18,   // u8
    ISRStatus = 19,      // u8
    DeviceSpecific = 20, // We assume no MSI-X support.
}

impl Offsets {
    fn as_u16(&self) -> u16 {
        *self as u16
    }
}

/// Describes a VirtIO PCI transport that is for
/// some reason unacceptable.
///
#[derive(Debug)]
pub enum ConfigError {
    /// The configuration space was malformed.
    BadConfigurationSpace,
}

/// Implements VirtIO configuration for the PCI transport.
///
pub struct Transport {
    // A handle to the PCI device.
    //
    device: pci::Device,

    // The I/O port for the configuration space.
    port: u16,

    // The offset into the port where device-specific
    // configuration exists.
    device_offset: u16,
}

impl Transport {
    /// Instantiate a PCI transport using the given device.
    ///
    /// `new` iterates through the given PCI capabilities,
    /// parsing the virtio-related structures and returning
    /// them.
    ///
    pub fn new(device: pci::Device) -> Result<Self, ConfigError> {
        let port = if let pci::Bar::IOMapped { port } = device.bar(0) {
            if port > (u16::MAX as u32) {
                return Err(ConfigError::BadConfigurationSpace);
            }

            port as u16
        } else {
            return Err(ConfigError::BadConfigurationSpace);
        };

        let device_offset = Offsets::DeviceSpecific.as_u16();

        device.enable_bus_master();
        Ok(Transport {
            device,
            port,
            device_offset,
        })
    }

    /// Read from the configuration space.
    ///
    #[inline]
    #[track_caller]
    fn read_config<T: PortRead>(&self, offset: u16) -> T {
        let addr = self.port.checked_add(offset).expect("bad I/O port offset");
        unsafe { Port::new(addr).read() }
    }

    /// Write to the configuration space.
    ///
    #[inline]
    #[track_caller]
    fn write_config<T: PortWrite>(&self, offset: u16, value: T) {
        let addr = self.port.checked_add(offset).expect("bad I/O port offset");
        unsafe { Port::new(addr).write(value) }
    }
}

impl crate::Transport for Transport {
    /// read_device_config_u8 returns the device-specific
    /// configuration byte at the given offset.
    ///
    fn read_device_config_u8(&self, offset: u16) -> u8 {
        let addr = self.device_offset.checked_add(offset).unwrap();
        self.read_config(addr)
    }

    /// read_irq returns the device's interrupt number.
    ///
    fn read_irq(&self) -> Irq {
        Irq::new(self.device.interrupt_line).expect("bad IRQ")
    }

    /// read_interrupt_status returns the device's current
    /// interrupt status.
    ///
    fn read_interrupt_status(&self) -> InterruptStatus {
        let status = self.read_config(Offsets::ISRStatus.as_u16());
        InterruptStatus::from_bits_truncate(status)
    }

    /// read_status returns the device's status.
    ///
    fn read_status(&self) -> DeviceStatus {
        let status = self.read_config(Offsets::DeviceStatus.as_u16());
        DeviceStatus::from_bits_truncate(status)
    }

    /// write_status sets the device's status.
    ///
    fn write_status(&self, device_status: DeviceStatus) {
        self.write_config(Offsets::DeviceStatus.as_u16(), device_status.bits());
    }

    /// read_device_features returns the
    /// first 64 of the device's feature bits.
    ///
    fn read_device_features(&self) -> u64 {
        self.read_config::<u32>(Offsets::DeviceFeatures.as_u16()) as u64
    }

    /// write_driver_features sets the
    /// first 64 of the driver's feature bits.
    ///
    fn write_driver_features(&self, features: u64) {
        self.write_config(Offsets::DriverFeatures.as_u16(), features as u32);
    }

    /// read_num_queues returns the maximum number of
    /// virtqueues supported by the device.
    ///
    fn read_num_queues(&self) -> u16 {
        panic!("read_num_queues is not supported for legacy devices.");
    }

    /// select_queue sets the current virtqueue.
    ///
    fn select_queue(&self, index: u16) {
        self.write_config(Offsets::QueueSelect.as_u16(), index);
    }

    /// queue_size returns the maximum number of descriptors
    /// supported by the device in any virtqueue.
    ///
    fn queue_size(&self) -> u16 {
        self.read_config(Offsets::QueueSize.as_u16())
    }

    /// set_queue_size notifies the device of the number
    /// of descriptors in the descriptor area of the
    /// current virtqueue (set using select_queue).
    ///
    fn set_queue_size(&self, _size: u16) {
        panic!("set_queue_size is not supported for legacy devices.");
    }

    /// notify_queue notifies the device that the
    /// virtqueue at the given index has descriptors
    /// ready in the driver area.
    ///
    fn notify_queue(&self, queue_index: u16) {
        self.write_config(Offsets::QueueNotify.as_u16(), queue_index);
    }

    /// enable_queue notifies the device to use the
    /// current virtqueue (set using select_queue).
    ///
    fn enable_queue(&self) {
        panic!("enable_queue is not supported for legacy devices.");
    }

    /// set_queue_descriptor_area notifies the device of
    /// the physical address of the descriptor area of
    /// the current virtqueue (set using select_queue).
    ///
    fn set_queue_descriptor_area(&self, area: PhysAddr) {
        let addr = area.as_usize() / 4096;
        if addr > u32::MAX as usize {
            panic!(
                "queue address / 4096 exceeds max u32: {} > {}",
                addr,
                u32::MAX
            );
        }

        self.write_config(Offsets::QueueAddress.as_u16(), addr as u32);
    }

    /// set_queue_driver_area notifies the device of the
    /// physical address of the driver area of the current
    /// virtqueue (set using select_queue).
    ///
    fn set_queue_driver_area(&self, _area: PhysAddr) {
        panic!("set_queue_driver_area is not supported for legacy devices.");
    }

    /// set_queue_device_area notifies the device of the
    /// physical address of the device area of the current
    /// virtqueue (set using select_queue).
    ///
    fn set_queue_device_area(&self, _area: PhysAddr) {
        panic!("set_queue_device_area is not supported for legacy devices.");
    }
}
