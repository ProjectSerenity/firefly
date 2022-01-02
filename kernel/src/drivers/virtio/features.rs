//! features provides bitflags for each of the different feature flags.

// Virtio negotiates features using specific bits in an arbitrary
// length bit sequence. This is documented in more detail in
// https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html,
// section 2.2, with specific constants defined in other sections.
//
// Since different kinds of devices can have overlapping feature bit
// spaces (with the same bit indicating a different feature in each
// device type), we can't have a single Rust type for all features.
// Instead, we have a separate type for each feature space. This
// includes one for features shared by all device types (Reserved),
// plus another type for each device type (Network, Block, Console,
// Entropy, Ballooning, Scsi, Gpu, DeviceId, and Crypto).
//
// When producing feature bits, simply OR together tbe bits you want
// to use:
//
//     General::RING_EVENT_IDX.bits() | Network::MAC.bits()
//
// When parsing feature bits, simply parse the feature types you care
// about:
//
//     (General::from_bits_truncate(bits), Network::from_bits_truncate(bits))

use crate::print;
use bitflags::bitflags;

/// debug prints flags as a sequence of set bits.
///
pub fn debug(flags: u64) {
    print!("flags(");
    let mut first = true;
    for i in 0..64 {
        let mask = 1 << i;
        if flags & mask == mask {
            if first {
                first = false;
            } else {
                print!(", ");
            }

            print!("{}", i);
        }
    }

    print!(")\n");
}

bitflags! {
    /// Reserved represents the set of reserved virtio
    /// feature flags, as documented in section 6.
    ///
    pub struct Reserved: u64 {
        /// Negotiating this feature indicates that the driver
        /// can use descriptors with the VIRTQ_DESC_F_INDIRECT
        /// flag set, as described in 2.6.5.3 Indirect
        /// Descriptors and 2.7.7 Indirect Flag: Scatter-Gather
        /// Support.
        const RING_INDIRECT_DESC = 1 << 28;

        /// This feature enables the used_event and the
        /// avail_event fields as described in 2.6.7, 2.6.8 and
        /// 2.7.10.
        const RING_EVENT_IDX = 1 << 29;

        /// This indicates compliance with the virtio 1.1
        /// specification, giving a simple way to detect legacy
        /// devices or drivers.
        const VERSION_1 = 1 << 32;

        /// This feature indicates that the device can be used
        /// on a platform where device access to data in memory
        /// is limited and/or translated. E.g. this is the case
        /// if the device can be located behind an IOMMU that
        /// translates bus addresses from the device into
        /// physical addresses in memory, if the device can be
        /// limited to only access certain memory addresses or
        /// if special commands such as a cache flush can be
        /// needed to synchronise data in memory with the device.
        ///
        /// Whether accesses are actually limited or translated
        /// is described by platform-specific means. If this
        /// feature bit is set to 0, then the device has same
        /// access to memory addresses supplied to it as the
        /// driver has.
        ///
        /// In particular, the device will always use physical
        /// addresses matching addresses used by the driver
        /// (typically meaning physical addresses used by the
        /// CPU) and not translated further, and can access any
        /// address supplied to it by the driver.
        ///
        /// When clear, this overrides any platform-specific
        /// description of whether device access is limited or
        /// translated in any way, e.g. whether an IOMMU may be
        /// present.
        const ACCESS_PLATFORM = 1 << 33;

        /// This feature indicates support for the packed
        /// virtqueue layout as described in 2.7 Packed Virtqueues.
        const RING_PACKED = 1 << 34;

        /// This feature indicates that all buffers are used by
        /// the device in the same order in which they have
        /// been made available.
        const IN_ORDER = 1 << 35;

        /// This feature indicates that memory accesses by the
        /// driver and the device are ordered in a way described
        /// by the platform.
        ///
        /// If this feature bit is negotiated, the ordering in
        /// effect for any memory accesses by the driver that
        /// need to be ordered in a specific way with respect
        /// to accesses by the device is the one suitable for
        /// devices described by the platform. This implies that
        /// the driver needs to use memory barriers suitable for
        /// devices described by the platform; e.g. for the PCI
        /// transport in the case of hardware PCI devices.
        ///
        /// If this feature bit is not negotiated, then the
        /// device and driver are assumed to be implemented in
        /// software, that is they can be assumed to run on
        /// identical CPUs in an SMP configuration.
        ///
        /// Thus a weaker form of memory barriers is sufficient
        /// to yield better performance.
        const ORDER_PLATFORM = 1 << 36;

        /// This feature indicates that the device supports
        /// Single Root I/O Virtualization. Currently only PCI
        /// devices support this feature.
        const SR_IOV = 1 << 37;

        /// This feature indicates that the driver passes extra
        /// data (besides identifying the virtqueue) in its
        /// device notifications. See 2.7.23 Driver notifications.
        const NOTIFICATION_DATA = 1 << 38;
    }
}

bitflags! {
    /// Network represents the set of network virtio
    /// feature flags, as documented in section 5.1.3.
    ///
    pub struct Network: u64 {
        /// Device handles packets with partial checksum. This
        /// “checksum offload” is a common feature on modern
        /// network cards.
        const CSUM = 1 << 0;

        /// Driver handles packets with partial checksum.
        const GUEST_CSUM = 1 << 1;

        /// Control channel offloads reconfiguration support.
        const CTRL_GUEST_OFFLOADS = 1 << 2;

        /// Device maximum MTU reporting is supported. If
        /// offered by the device, device advises driver about
        /// the value of its maximum MTU. If negotiated, the
        /// driver uses mtu as the maximum MTU value.
        const MTU = 1 << 3;

        /// Device has given MAC address.
        const MAC = 1 << 5;

        /// Driver can receive TSOv4.
        const GUEST_TSO4 = 1 << 7;

        /// Driver can receive TSOv6.
        const GUEST_TSO6 = 1 << 8;

        /// Driver can receive TSO with ECN.
        const GUEST_ECN = 1 << 9;

        /// Driver can receive UFO.
        const GUEST_UFO = 1 << 10;

        /// Device can receive TSOv4.
        const HOST_TSO4 = 1 << 11;

        /// Device can receive TSOv6.
        const HOST_TSO6 = 1 << 12;

        /// Device can receive TSO with ECN.
        const HOST_ECN = 1 << 13;

        /// Device can receive UFO.
        const HOST_UFO = 1 << 14;

        /// Driver can merge receive buffers.
        const MRG_RXBUF = 1 << 15;

        /// Configuration status field is available.
        const STATUS = 1 << 16;

        /// Control channel is available.
        const CTRL_VQ = 1 << 17;

        /// Control channel RX mode support.
        const CTRL_RX = 1 << 18;

        /// Control channel VLAN filtering.
        const CTRL_VLAN = 1 << 19;

        /// Driver can send gratuitous packets.
        const GUEST_ANNOUNCE = 1 << 21;

        /// Device supports multiqueue with automatic receive
        /// steering.
        const MQ = 1 << 22;

        /// Set MAC address through control channel.
        const CTRL_MAC_ADDR = 1 << 23;

        /// Device can process duplicated ACKs and report
        /// number of coalesced segments and duplicated ACKs.
        const RSC_EXT = 1 << 61;

        /// Device may act as a standby for a primary device
        /// with the same MAC address.
        const STANDBY = 1 << 62;
    }
}

bitflags! {
    /// Block represents the set of block virtio
    /// feature flags, as documented in section 5.2.3.
    ///
    pub struct Block: u64 {
        /// Maximum size of any single segment is in size_max.
        const SIZE_MAX = 1 << 1;

        /// Maximum number of segments in a request is in
        /// seg_max.
        const SEG_MAX = 1 << 2;

        /// Disk-style geometry specified in geometry.
        const GEOMETRY = 1 << 4;

        /// Device is read-only.
        const RO = 1 << 5;

        /// Block size of disk is in blk_size.
        const BLK_SIZE = 1 << 6;

        /// Cache flush command support.
        const FLUSH = 1 << 9;

        /// Device exports information on optimal I/O alignment.
        const TOPOLOGY = 1 << 10;

        /// Device can toggle its cache between writeback and
        /// writethrough modes.
        const CONFIG_WCE = 1 << 11;

        /// Device can support discard command, maximum discard
        /// sectors size in max_discard_sectors and maximum
        /// discard segment number in max_discard_seg.
        const DISCARD = 1 << 13;

        /// Device can support write zeroes command, maximum
        /// write zeroes sectors size in max_write_zeroes_sectors
        /// and maximum write zeroes segment number in
        /// max_write_zeroes_seg.
        const WRITE_ZEROES = 1 << 14;
    }
}

bitflags! {
    /// Console represents the set of console virtio
    /// feature flags, as documented in section 5.3.3.
    ///
    pub struct Console: u64 {
        /// Configuration cols and rows are valid.
        const SIZE = 1 << 0;

        /// Device has support for multiple ports; max_nr_ports
        /// is valid and control virtqueues will be used.
        const MULTIPORT = 1 << 1;

        /// Device has support for emergency write. Configuration
        /// field emerg_wr is valid.
        const EMERG_WRITE = 1 << 2;
    }
}

bitflags! {
    /// Entropy represents the set of entropy virtio
    /// feature flags, as documented in section 5.4.3.
    ///
    pub struct Entropy: u64 {
        // None defined yet.
    }
}

bitflags! {
    /// Ballooning represents the set of memory ballooning
    /// virtio feature flags, as documented in section 5.5.3.
    ///
    pub struct Ballooning: u64 {
        /// Host has to be told before pages from the balloon
        /// are used.
        const MUST_TELL_HOST = 1 << 0;

        /// A virtqueue for reporting guest memory statistics is
        /// present.
        const STATS_VQ = 1 << 1;

        /// Deflate balloon on guest out of memory condition.
        const DEFLATE_ON_OOM = 1 << 2;
    }
}

bitflags! {
    /// Scsi represents the set of SCSI virtio
    /// feature flags, as documented in section 5.6.3.
    ///
    pub struct Scsi: u64 {
        /// A single request can include both device-readable
        /// and device-writable data buffers.
        const INOUT = 1 << 0;

        /// The host SHOULD enable reporting of hot-plug and
        /// hot-unplug events for LUNs and targets on the SCSI
        /// bus. The guest SHOULD handle hot-plug and hot-unplug
        /// events.
        const HOTPLUG = 1 << 1;

        /// The host will report changes to LUN parameters via a
        /// VIRTIO_SCSI_T_PARAM_CHANGE event; the guest SHOULD
        /// handle them.
        const CHANGE = 1 << 2;

        /// The extended fields for T10 protection information
        /// (DIF/DIX) are included in the SCSI request header.
        const T10_PI = 1 << 3;
    }
}

bitflags! {
    /// Gpu represents the set of GPU virtio
    /// feature flags, as documented in section 5.7.3.
    ///
    pub struct Gpu: u64 {
        /// Virgl 3D mode is supported.
        const VIRGL = 1 << 0;

        /// EDID is supported.
        const EDID = 1 << 1;
    }
}

bitflags! {
    /// Input represents the set of input device virtio
    /// feature flags, as documented in section 5.8.3.
    ///
    pub struct Input: u64 {
        // None defined yet.
    }
}

bitflags! {
    /// Crypto represents the set of crypto virtio
    /// feature flags, as documented in section 5.9.3.
    ///
    pub struct Crypto: u64 {
        /// Revision 1. Revision 1 has a specific request format
        /// and other enhancements (which result in some
        /// additional requirements).
        const REVISION_1 = 1 << 0;

        /// Stateless mode requests are supported by the CIPHER
        /// service.
        const CIPHER_STATELESS_MOD = 1 << 1;

        /// Stateless mode requests are supported by the HASH
        /// service.
        const HASH_STATELESS_MODE = 1 << 2;

        /// Stateless mode requests are supported by the MAC
        /// service.
        const MAC_STATELESS_MODE = 1 << 3;

        /// Stateless mode requests are supported by the AEAD
        /// service.
        const AEAD_STATELESS_MODE = 1 << 4;
    }
}
