//! pci implements the PCI transport mechanism documented in section
//! 4.1 of <https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html>.

use crate::drivers::pci;
use x86_64::structures::paging::frame::{PhysFrame, PhysFrameRange};

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
