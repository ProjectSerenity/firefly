//! network implements virtio network cards.

use bitflags::bitflags;

/// Header represents the header data that must preceed
/// each buffer sent to the device.
///
#[repr(C, packed)]
#[derive(Clone, Copy)]
struct Header {
    flags: u8,
    gso_type: u8,
    header_len: u16,
    gso_size: u16,
    checksum_start: u16,
    checksum_offset: u16,
    num_buffers: u16,
}

bitflags! {
    /// HeaderFlags describes the flags that can be
    /// passed in the Header type's flags field.
    ///
    struct HeaderFlags: u8 {
        /// NONE indicates no flags.
        const NONE = 0;

        /// NEEDS_CHECKSUM indicates that the device
        /// should populate a checksum field in the
        /// buffer.
        const NEEDS_CHECKSUM = 1;

        /// DATA_VALID indicates that the device has
        /// checked the received packet's checksum
        /// and confirms that it is valid.
        const DATA_VALID = 2;

        /// RSC_INFO indicates that the device has
        /// included information on the number of
        /// coalesced TCP segments.
        const RSC_INFO = 4;
    }
}

/// GsoType represents the generic segmentation offload
/// types used in the gos_type field of the Header type.
///
#[derive(Clone, Copy, Debug)]
enum GsoType {
    None = 0,
    TcpV4 = 1,
    Udp = 3,
    TcpV6 = 4,
    Ecn = 0x80,
}

/// Config is a helper type that gives the layout
/// layout of the device-specific config type for
/// network card devices, as defined in section
/// 5.1.4:
///
/// ```
/// struct virtio_net_config {
///     u8 mac[6];
///     le16 status;
///     le16 max_virtqueue_pairs;
///     le16 mtu;
/// }
/// ```
///
#[repr(C, packed)]
#[derive(Clone, Copy)]
struct Config {
    mac: [u8; 6],
    status: u16,
    max_virtqueue_pairs: u16,
    mtu: u16,
}
