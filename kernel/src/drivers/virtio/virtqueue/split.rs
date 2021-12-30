//! split implements split virtqueues, as described in section
//! 2.6.

use bitflags::bitflags;

bitflags! {
    /// DescriptorFlags represents the set of flags that can
    /// be used in a split virtqueue descriptor's flags field.
    ///
    struct DescriptorFlags: u16 {
        /// NONE indicates that no flags have been set.
        const NONE = 0;

        /// NEXT indicates that the buffer continues into the
        /// descriptor referenced by the current descriptor's next
        /// field.
        const NEXT = 1;

        /// WRITE marks a buffer as write-only for the device.
        /// If WRITE is absent, the descriptor is read-only for the
        /// device.
        const WRITE = 2;

        /// INDIRECT means the buffer contains a list of buffer
        /// descriptors.
        const INDIRECT = 4;
    }
}

/// Descriptor represents a split virtqueue, as described
/// in section 2.6.5.
///
#[repr(C, packed)]
#[derive(Clone, Copy, Debug, Default)]
struct Descriptor {
    // addr is the physical address of the buffer.
    addr: u64,

    // len is the length in bytes of the buffer.
    len: u32,

    // flags indicates the buffer's behaviour.
    flags: u16,

    // next points to another descirptor, if the NEXT flag is present.
    next: u16,
}

impl Descriptor {
    /// flags returns the descriptor's flags.
    ///
    fn flags(&self) -> DescriptorFlags {
        DescriptorFlags::from_bits_truncate(u16::from_le(self.flags))
    }

    /// has_next returns whether more buffer follows
    /// in a descriptor reference by self.next.
    ///
    fn has_next(&self) -> bool {
        self.flags().contains(DescriptorFlags::NEXT)
    }

    /// writable returns whether the device is allowed
    /// to write to the buffer.
    ///
    fn writable(&self) -> bool {
        self.flags().contains(DescriptorFlags::WRITE)
    }

    /// indirect returns whether the buffer contains
    /// a sequence of descriptors.
    ///
    #[allow(dead_code)]
    fn indirect(&self) -> bool {
        self.flags().contains(DescriptorFlags::INDIRECT)
    }
}
