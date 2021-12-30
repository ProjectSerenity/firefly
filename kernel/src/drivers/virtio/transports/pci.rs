//! pci implements the PCI transport mechanism documented in section
//! 4.1 of <https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html>.

use crate::drivers::pci;
use crate::memory::mmio;
use x86_64::structures::paging::frame::{PhysFrame, PhysFrameRange};

/// CAPABILITY_ID_VENDOR is the unique identifier
/// for a PCI capability containing vendor-specific
/// data.
///
/// All Virtio configuration data is provided in
/// PCI capabilities with this id.
///
const CAPABILITY_ID_VENDOR: u8 = 0x09;

/// Type represents a virtio configuration type.
///
#[derive(Debug)]
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

/// ConfigError indicates an issue that caused a
/// virtio configuration to be unacceptable.
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

/// Transport implements virtio configuration for
/// the PCI transport, as described in section
/// 4.1.
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
    /// new iterates through the given PCI capabilities,
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

        if common.is_none() {
            Err(ConfigError::NoCommon)
        } else if notify.is_none() || notify_off_multiplier.is_none() {
            Err(ConfigError::NoNotify)
        } else if interrupt.is_none() {
            Err(ConfigError::NoInterrupt)
        } else if device_spec.is_none() {
            Err(ConfigError::NoDevice)
        } else {
            device.enable_bus_master();
            unsafe {
                Ok(Transport {
                    pci: device,
                    common: mmio::Region::map(common.unwrap()),
                    notify: mmio::Region::map(notify.unwrap()),
                    notify_offset_multiplier: notify_off_multiplier.unwrap(),
                    interrupt: mmio::Region::map(interrupt.unwrap()),
                    device: mmio::Region::map(device_spec.unwrap()),
                })
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
