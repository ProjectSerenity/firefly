//! virtio contains drivers for Virtio v1.1, as described in
//! <https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html>.

// Note, all references to 'sections' are relative to the
// Virtio 1.1 specification linked above.

pub mod features;

use crate::drivers;
use crate::drivers::pci;

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
        Some(DeviceId::NetworkCard) => None,
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
