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

bitflags! {
    /// DriverFlags represents the set of flags that can
    /// be used in a split virtqueue's driver area's flags field.
    ///
    struct DriverFlags: u16 {
        /// NO_NOTIFICATIONS indicates that the device should not
        /// send notifications to the driver after the descriptor
        /// chain is returned in the device area.
        const NO_NOTIFICATIONS = 1;
    }
}

/// DriverArea represents a split virtqueue's area where
/// the driver provides descriptors to the device, as described
/// in section 2.6.6.
///
#[derive(Debug)]
struct DriverArea {
    // flags indicates the driver's behaviour recommendations
    // to the device.
    flags: &'static mut u16,

    // index is the index into ring (modulo the ring's size)
    // at which the next descriptor will be written.
    index: &'static mut u16,

    // ring is the ring buffer containing the descriptor heads
    // passed to the device.
    ring: &'static mut [u16],

    // recv_event is used by the driver to indicate to the device
    // when to send notifications when descriptors are returned
    // in the device area.
    recv_event: &'static mut u16,
}
