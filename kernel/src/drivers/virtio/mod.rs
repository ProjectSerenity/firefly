// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Partially implements [Virtio v1.1](https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html)
//! for virtual device drivers, plus drivers for network devices and entropy sources.
//!
//! Virtual I/O (Virtio) devices provide a generic abstraction for device drivers.
//! Virtio is used by hypervisors to provide guest virtual machines with efficient
//! access to physical devices. This avoids the security risks of giving guests
//! direct access to physical devices and the engineering and performance overhead
//! of reimplementing device behaviours in software.
//!
//! A Virtio [`Driver`] consists of a [`Transport`] and at least one [`Virtqueue`].
//! A device driver uses a [`Driver`] to exchange memory buffers and notifications
//! with the device. The [`Transport`] is used to discover and configure the device.
//! [`Virtqueue`]s provide a way to exchange memory buffers with the device, both
//! to send data to the device and receive data from it.
//!
//! This module currently supports the [PCI transport](transports::pci::Transport)
//! and the [split virtqueue](virtqueues::split::Virtqueue). There is currently no
//! support for the [MMIO transport](https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html#x1-1440002)
//! or the [packed virtqueue](https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html#x1-610007).
//!
//! We currently have Virtio device drivers for the following device types:
//!
//! - [Entropy source](entropy)
//! - [Network card](network)
//!
//! # Examples
//!
//! A [`Transport`] will be obtained using a transport-specific mechanism. For
//! example, a PCI transport is produced from a [`PCI Device`](crate::drivers::pci::Device)
//! using [`transports::pci::Transport::new`]:
//!
//! ```
//! fn install_pci_device(device: pci::Device) {
//!     let _driver = Transport::new(device).unwrap();
//! }
//! ```
//!
//! The transport can then be used to reset and initialise the device, negotiating
//! features and preparing virtqueues.

pub mod entropy;
pub mod features;
pub mod network;
pub mod transports;
pub mod virtqueues;

use crate::drivers::pci;
use crate::drivers::virtio::virtqueues::split;
use crate::interrupts::Irq;
use crate::println;
use alloc::boxed::Box;
use alloc::sync::Arc;
use alloc::vec::Vec;
use bitflags::bitflags;
use x86_64::PhysAddr;

/// MAX_DESCRIPTORS is the maximum number of
/// descriptors allowed in each virtqueue.
///
pub const MAX_DESCRIPTORS: u16 = 128;

/// VENDOR_ID is the PCI vendor id of a virtio device.
///
const VENDOR_ID: u16 = 0x1af4;

/// MIN_DEVICE_ID is the smallest valid PCI device id
/// of a virtio device.
///
const MIN_DEVICE_ID: u16 = 0x1000;

/// MAX_DEVICE_ID is the largest valid PCI device id
/// of a virtio device.
///
const MAX_DEVICE_ID: u16 = 0x107f;

/// pci_device_supported returns a PCI DeviceDriver if the
/// given device is a supported virtio device.
///
pub fn pci_device_supported(device: &pci::Device) -> Option<pci::DeviceDriver> {
    if device.vendor != VENDOR_ID {
        return None;
    }

    if device.device < MIN_DEVICE_ID || MAX_DEVICE_ID < device.device {
        return None;
    }

    match DeviceId::from_pci_device_id(device.device) {
        Some(DeviceId::EntropySource) => {
            println!("Installing Virtio entropy source.");
            Some(entropy::install_pci_device)
        }
        Some(DeviceId::NetworkCard) => {
            println!("Installing Virtio network card.");
            Some(network::install_pci_device)
        }
        Some(other) => {
            println!("Detected unsupported VirtIO device {:?}.", other);
            None
        }
        _ => None,
    }
}

/// DeviceId represents a virtio divice id, as described
/// in section 5.
///
#[derive(Clone, Copy, Debug)]
enum DeviceId {
    NetworkCard = 1,
    BlockDevice = 2,
    Console = 3,
    EntropySource = 4,
    MemoryBallooning = 5,
    IoMemory = 6,
    Rpmsg = 7,
    ScsiHost = 8,
    P9Transport = 9,
    Mac80211Wan = 10,
    RprocSerial = 11,
    VirtioCaif = 12,
    MemoryBalloon = 13,
    GpuDevice = 16,
    TimerClockDevice = 17,
    InputDevice = 18,
    SocketDevice = 19,
    CryptoDevice = 20,
    SignalDistributionModule = 21,
    PstoreDevice = 22,
    IommuDevice = 23,
    MemoryDevice = 24,
}

impl DeviceId {
    /// from_pci_device_id returns a virtio device
    /// id, if the given PCI device id matches.
    ///
    pub fn from_pci_device_id(device: u16) -> Option<Self> {
        match device {
            0x1041 | 0x1000 => Some(DeviceId::NetworkCard),
            0x1042 | 0x1001 => Some(DeviceId::BlockDevice),
            0x1043 | 0x1003 => Some(DeviceId::Console),
            0x1044 | 0x1005 => Some(DeviceId::EntropySource),
            0x1045 | 0x1002 => Some(DeviceId::MemoryBallooning),
            0x1046 => Some(DeviceId::IoMemory),
            0x1047 => Some(DeviceId::Rpmsg),
            0x1048 | 0x1004 => Some(DeviceId::ScsiHost),
            0x1049 | 0x1009 => Some(DeviceId::P9Transport),
            0x104a => Some(DeviceId::Mac80211Wan),
            0x104b => Some(DeviceId::RprocSerial),
            0x104c => Some(DeviceId::VirtioCaif),
            0x1050 => Some(DeviceId::MemoryBalloon),
            0x1051 => Some(DeviceId::GpuDevice),
            0x1052 => Some(DeviceId::TimerClockDevice),
            0x1053 => Some(DeviceId::InputDevice),
            0x1054 => Some(DeviceId::SocketDevice),
            0x1055 => Some(DeviceId::CryptoDevice),
            0x1056 => Some(DeviceId::SignalDistributionModule),
            0x1057 => Some(DeviceId::PstoreDevice),
            0x1058 => Some(DeviceId::IommuDevice),
            0x1059 => Some(DeviceId::MemoryDevice),
            _ => None,
        }
    }
}

bitflags! {
    /// DeviceStatus represents the status of a virtio device,
    /// as described in section 2.1.
    ///
    pub struct DeviceStatus: u8 {
        /// RESET instructs the device to reset, or indicates that
        /// the device has been reset.
        const RESET = 0;

        /// ACKNOWLEDGE indicates that the guest OS has found
        /// the device and recognized it as a valid virtio device.
        const ACKNOWLEDGE = 1;

        /// DRIVER indicates that the guest OS knows how to drive
        /// the device.
        ///
        /// Note: There could be a significant (or infinite)
        /// delay before setting this bit. For example, under Linux,
        /// drivers can be loadable modules.
        const DRIVER = 2;

        /// FAILED indicates that something went wrong in the guest,
        /// and it has given up on the device. This could be an
        /// internal error, or the driver didn’t like the device for
        /// some reason, or even a fatal error during device operation.
        const FAILED = 128;

        /// FEATURES_OK indicates that the driver has acknowledged
        /// all the features it understands, and feature negotiation
        /// is complete.
        const FEATURES_OK = 8;

        /// DRIVER_OK indicates that the driver is set up and ready
        /// to drive the device.
        const DRIVER_OK = 4;

        /// DEVICE_NEEDS_RESET indicates that the device has
        /// experienced an error from which it can’t recover.
        const DEVICE_NEEDS_RESET = 64;
    }
}

bitflags! {
    /// InterruptStatus records the ISR status capability
    /// values documented in section 4.1.4.5.
    ///
    pub struct InterruptStatus: u8 {
        /// QUEUE_INTERRUPT indicates that a virtqueue has had buffers
        /// returned by the device.
        const QUEUE_INTERRUPT = 1 << 0;

        /// DEVICE_CONFIG_INTERRUPT indicates that the device has made
        /// a configuration change.
        const DEVICE_CONFIG_INTERRUPT = 1 << 1;
    }
}

/// Transport represents a virtio transport
/// mechanism.
///
pub trait Transport: Send + Sync {
    /// read_device_config_u8 returns the device-specific
    /// configuration byte at the given offset.
    ///
    fn read_device_config_u8(&self, offset: u16) -> u8;

    /// read_irq returns the device's interrupt number.
    ///
    fn read_irq(&self) -> Irq;

    /// read_interrupt_status returns the device's current
    /// interrupt status.
    ///
    fn read_interrupt_status(&self) -> InterruptStatus;

    /// read_status returns the device's status.
    ///
    fn read_status(&self) -> DeviceStatus;

    /// write_status sets the device's status.
    ///
    fn write_status(&self, device_status: DeviceStatus);

    /// add_status reads the current device status
    /// and sets the given additional bits.
    ///
    fn add_status(&self, device_status: DeviceStatus);

    /// has_status returns whether the current device
    /// status includes all of the given bits.
    ///
    fn has_status(&self, device_status: DeviceStatus) -> bool;

    /// read_device_features returns the first 64 bits
    /// of the device's features.
    ///
    fn read_device_features(&self) -> u64;

    /// write_driver_features sets the first 64 bits
    /// of the driver's features.
    ///
    fn write_driver_features(&self, features: u64);

    /// read_num_queues returns the maximum number of
    /// virtqueues supported by the device.
    ///
    fn read_num_queues(&self) -> u16;

    /// select_queue sets the current virtqueue.
    ///
    fn select_queue(&self, index: u16);

    /// queue_size returns the maximum number of descriptors
    /// supported by the device in any virtqueue.
    ///
    fn queue_size(&self) -> u16;

    /// set_queue_size notifies the device of the number
    /// of descriptors in the descriptor area of the
    /// current virtqueue (set using select_queue).
    ///
    fn set_queue_size(&self, size: u16);

    /// notify_queue notifies the device that the
    /// virtqueue at the given index has descriptors
    /// ready in the driver area.
    ///
    fn notify_queue(&self, queue_index: u16);

    /// enable_queue notifies the device to use the
    /// current virtqueue (set using select_queue).
    ///
    fn enable_queue(&self);

    /// set_queue_descriptor_area notifies the device of
    /// the physical address of the descriptor area of
    /// the current virtqueue (set using select_queue).
    ///
    fn set_queue_descriptor_area(&self, area: PhysAddr);

    /// set_queue_driver_area notifies the device of the
    /// physical address of the driver area of the current
    /// virtqueue (set using select_queue).
    ///
    fn set_queue_driver_area(&self, area: PhysAddr);

    /// set_queue_device_area notifies the device of the
    /// physical address of the device area of the current
    /// virtqueue (set using select_queue).
    ///
    fn set_queue_device_area(&self, area: PhysAddr);
}

/// Buffer represents a contiguous sequence of
/// physical memory, which can either be read
/// or written by the device.
///
pub enum Buffer {
    DeviceCanRead { addr: PhysAddr, len: usize },
    DeviceCanWrite { addr: PhysAddr, len: usize },
}

/// UsedBuffers contains a set of buffers that
/// the device has returned, along with the number
/// of bytes written to them.
///
pub struct UsedBuffers {
    pub buffers: Vec<Buffer>,
    pub written: usize,
}

/// VirtqueueError represents an error interacting
/// with a virtqueue.
///
#[derive(Clone, Copy, Debug)]
pub enum VirtqueueError {
    /// No descriptors were available for sending
    /// a buffer to the queue.
    NoDescriptors,
}

/// Virtqueue abstracts the implementation details
/// of a virtqueue.
///
pub trait Virtqueue: Send {
    /// send enqueues a request to the device. A request consists of
    /// a sequence of buffers. The sequence of buffers should place
    /// device-writable buffers after all device-readable buffers.
    ///
    /// send returns the descriptor index for the head of the chain.
    /// This can be used to identify when the device returns the
    /// buffer chain. If there are not enough descriptors to send
    /// the chain, send returns None.
    ///
    fn send(&mut self, buffers: &[Buffer]) -> Result<(), VirtqueueError>;

    /// notify informs the device that descriptors are ready
    /// to use in this virtqueue.
    ///
    fn notify(&self);

    /// recv returns the next set of buffers
    /// returned by the device, or None.
    ///
    fn recv(&mut self) -> Option<UsedBuffers>;

    /// num_descriptors returns the number of descriptors
    /// in this queue.
    ///
    fn num_descriptors(&self) -> usize;

    /// disable_notifications requests the device not to send
    /// notifications to this queue.
    ///
    fn disable_notifications(&mut self);

    /// enable_notifications requests the device to send
    /// notifications to this queue.
    ///
    fn enable_notifications(&mut self);
}

/// InitError describes an error initialising
/// a virtio device.
///
#[derive(Debug)]
pub enum InitError {
    /// The device did not indicate support
    /// for a feature required by the driver.
    MissingRequiredFeatures(u64),

    /// The driver attempted to initialise
    /// the device with more virtqueues than
    /// are supported. The device's maximum
    /// number of virtqueues is included.
    TooManyQueues(u16),

    /// The device rejected the feature set
    /// selected by the driver.
    DeviceRefusedFeatures,
}

/// Driver represents a virtio driver.
///
pub struct Driver {
    transport: Arc<dyn Transport>,
    features: u64,
    virtqueues: Vec<Box<dyn Virtqueue + Send>>,
}

impl Driver {
    /// Initialise the device, negotiating the
    /// given required and optional features and
    /// number of virtqueues.
    ///
    pub fn new(
        transport: Arc<dyn Transport>,
        must_features: u64,
        like_features: u64,
        num_queues: u16,
    ) -> Result<Self, InitError> {
        // See section 3.1.1 for the driver initialisation
        // process.
        transport.write_status(DeviceStatus::RESET);
        loop {
            // Section 4.1.4.3.2:
            //   After writing 0 to device_status, the driver MUST
            //   wait for a read of device_status to return 0 before
            //   reinitializing the device.
            if transport.read_status() == DeviceStatus::RESET {
                break;
            }
        }

        transport.add_status(DeviceStatus::ACKNOWLEDGE);
        transport.add_status(DeviceStatus::DRIVER);
        let max_queues = transport.read_num_queues();
        if max_queues < num_queues {
            return Err(InitError::TooManyQueues(max_queues));
        }

        // Read the feature set.
        let device_features = transport.read_device_features();
        if (device_features & must_features) != must_features {
            return Err(InitError::MissingRequiredFeatures(
                must_features & !device_features,
            ));
        }

        // Negotiate our supported features.
        let features = device_features & (must_features | like_features);
        transport.write_driver_features(features);
        transport.add_status(DeviceStatus::FEATURES_OK);
        if !transport.has_status(DeviceStatus::FEATURES_OK) {
            return Err(InitError::DeviceRefusedFeatures);
        }

        // Prepare our virtqueues.
        let mut virtqueues = Vec::new();
        for i in 0..num_queues {
            virtqueues.push(
                Box::new(split::Virtqueue::new(i, transport.clone(), features))
                    as Box<dyn Virtqueue + Send>,
            );
        }

        // Finish initialisation.
        transport.add_status(DeviceStatus::DRIVER_OK);

        Ok(Driver {
            transport,
            features,
            virtqueues,
        })
    }

    /// Permanently reset the device.
    ///
    pub fn reset(&mut self) {
        self.transport.write_status(DeviceStatus::RESET);
    }

    /// Returns the features that were negotiated with
    /// the device.
    ///
    pub fn features(&self) -> u64 {
        self.features
    }

    /// Returns this driver's IRQ number.
    ///
    pub fn irq(&self) -> Irq {
        self.transport.read_irq()
    }

    /// Returns this driver's interrupt status.
    ///
    /// Note that `interrupt_status` must be called in
    /// response to each interrupt so that the nature of
    /// the interrupt can be determined and so that the
    /// device will de-assert the interrupt. Failing to
    /// call `interrupt_status` will lead to the interrupt
    /// handler being called in an infinite loop.
    ///
    pub fn interrupt_status(&self) -> InterruptStatus {
        self.transport.read_interrupt_status()
    }

    /// Returns one byte of the device-specific configuration
    /// at the given offset.
    ///
    fn read_device_config_u8(&self, offset: u16) -> u8 {
        self.transport.read_device_config_u8(offset)
    }

    /// Enqueues a request to the given virtqueue.
    ///
    /// A request consists of a sequence of buffers. The sequence
    /// of buffers should place device-writable buffers after all
    /// device-readable buffers.
    ///
    pub fn send(&mut self, queue_index: u16, buffers: &[Buffer]) -> Result<(), VirtqueueError> {
        self.virtqueues[queue_index as usize].send(buffers)
    }

    /// Notifies the device that descriptors are ready to use
    /// in the given virtqueue.
    ///
    pub fn notify(&self, queue_index: u16) {
        self.virtqueues[queue_index as usize].notify();
    }

    /// Returns the number of descriptors in the given
    /// virtqueue.
    ///
    pub fn num_descriptors(&self, queue_index: u16) -> usize {
        self.virtqueues[queue_index as usize].num_descriptors()
    }

    /// Returns the next set of buffers returned by the device
    /// to the given virtqueue, or None.
    ///
    pub fn recv(&mut self, queue_index: u16) -> Option<UsedBuffers> {
        self.virtqueues[queue_index as usize].recv()
    }

    /// Requests the device not to send notifications to the
    /// given queue.
    ///
    pub fn disable_notifications(&mut self, queue_index: u16) {
        self.virtqueues[queue_index as usize].disable_notifications();
    }

    /// Requests the device to send notifications to the given
    /// queue.
    ///
    pub fn enable_notifications(&mut self, queue_index: u16) {
        self.virtqueues[queue_index as usize].enable_notifications();
    }
}
