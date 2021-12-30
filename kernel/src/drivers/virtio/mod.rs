//! virtio contains drivers for Virtio v1.1, as described in
//! <https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html>.

// Note, all references to 'sections' are relative to the
// Virtio 1.1 specification linked above.

pub mod features;
pub mod network;
pub mod transports;
pub mod virtqueue;

use crate::drivers;
use crate::drivers::pci;
use crate::drivers::virtio::virtqueue::split;
use crate::interrupts::Irq;
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

/// pci_device_supported returns a PciDeviceDriver if the
/// given device is a supported virtio device.
///
pub fn pci_device_supported(device: &pci::Device) -> Option<drivers::PciDeviceDriver> {
    if device.vendor != VENDOR_ID {
        return None;
    }

    if device.device < MIN_DEVICE_ID || MAX_DEVICE_ID < device.device {
        return None;
    }

    match DeviceId::from_pci_device_id(device.device) {
        Some(DeviceId::NetworkCard) => Some(network::install_pci_device),
        Some(_device) => None,
        None => None,
    }
}

/// DeviceId represents a virtio divice id, as described
/// in section 5.
///
#[derive(Clone, Copy, Debug)]
pub enum DeviceId {
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

    /// pci_device_id returns the PCI device id that
    /// corresponds to this virtio device id.
    ///
    pub fn pci_device_id(self) -> u16 {
        self as u16 + 0x1040
    }

    /// transitional_pci_device_id returns the PCI
    /// device id for the legacy representation of
    /// this virtio device id, if applicable.
    ///
    pub fn transitional_pci_device_id(&self) -> Option<u16> {
        match self {
            DeviceId::NetworkCard => Some(0x1000),
            DeviceId::BlockDevice => Some(0x1001),
            DeviceId::MemoryBallooning => Some(0x1002),
            DeviceId::Console => Some(0x1003),
            DeviceId::ScsiHost => Some(0x1004),
            DeviceId::EntropySource => Some(0x1005),
            DeviceId::P9Transport => Some(0x1009),
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
    virtqueues: Vec<Box<dyn virtqueue::Virtqueue + Send>>,
}

impl Driver {
    /// new initialises the device, negotiating
    /// the given required and optional features
    /// and number of virtqueues.
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
            virtqueues.push(Box::new(split::Virtqueue::new(i, transport.clone()))
                as Box<dyn virtqueue::Virtqueue + Send>);
        }

        // Finish initialisation.
        transport.add_status(DeviceStatus::DRIVER_OK);

        Ok(Driver {
            transport,
            features,
            virtqueues,
        })
    }

    /// reset permanently resets the device.
    ///
    pub fn reset(&mut self) {
        self.transport.write_status(DeviceStatus::RESET);
    }

    /// features returns the features that were negotiated
    /// with the device.
    ///
    pub fn features(&self) -> u64 {
        self.features
    }

    /// irq returns this driver's IRQ number.
    ///
    pub fn irq(&self) -> Irq {
        self.transport.read_irq()
    }

    /// interrupt_status returns this driver's interrupt
    /// status.
    ///
    pub fn interrupt_status(&self) -> InterruptStatus {
        self.transport.read_interrupt_status()
    }

    /// read_device_config_u8 returns the device-specific
    /// configuration byte at the given offset.
    ///
    fn read_device_config_u8(&self, offset: u16) -> u8 {
        self.transport.read_device_config_u8(offset)
    }

    /// send enqueues a request to the given virtqueue. A request
    /// consists of a sequence of buffers. The sequence of buffers
    /// should place device-writable buffers after all
    /// device-readable buffers.
    ///
    /// send returns the descriptor index for the head of the chain.
    /// This can be used to identify when the device returns the
    /// buffer chain. If there are not enough descriptors to send
    /// the chain, send returns None.
    ///
    pub fn send(
        &mut self,
        queue_index: u16,
        buffers: &[virtqueue::Buffer],
    ) -> Result<(), virtqueue::Error> {
        self.virtqueues[queue_index as usize].send(buffers)
    }

    /// notify informs the device that descriptors are ready
    /// to use in the given virtqueue.
    ///
    pub fn notify(&self, queue_index: u16) {
        self.virtqueues[queue_index as usize].notify();
    }

    /// num_descriptors returns the number of descriptors in the
    /// given virtqueue.
    ///
    pub fn num_descriptors(&self, queue_index: u16) -> usize {
        self.virtqueues[queue_index as usize].num_descriptors()
    }

    /// recv returns the next set of buffers
    /// returned by the device to the given
    /// queue, or None.
    ///
    pub fn recv(&mut self, queue_index: u16) -> Option<virtqueue::UsedBuffers> {
        self.virtqueues[queue_index as usize].recv()
    }
}
