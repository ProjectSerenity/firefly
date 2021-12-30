//! pci implements the PCI transport mechanism documented in section
//! 4.1 of <https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html>.

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
