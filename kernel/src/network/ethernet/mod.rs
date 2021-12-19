//! ethernet implements the layer 2 Ethernet protocol.

use core::fmt;
use core::ops::Deref;

/// MAC_SIZE is the size of a MAC address, in bytes.
///
pub const MAC_SIZE: usize = 6;

/// A MAC address.
///
/// MAC addresses are used to identify Ethernet devices.
///
#[derive(Clone, Copy, Eq, PartialEq)]
pub struct Mac([u8; MAC_SIZE]);

impl Mac {
    /// The broadcast MAC address.
    ///
    /// The broadcast MAC address, ff:ff:ff:ff:ff:ff, indicates
    /// that a frame should be received by all receivers,
    /// regardless of their local MAC address.
    ///
    pub const BROADCAST: Mac = Mac([0xff; MAC_SIZE]);

    /// new returns a new MAC address.
    ///
    #[inline]
    pub const fn new(bytes: [u8; MAC_SIZE]) -> Mac {
        Mac(bytes)
    }
}

// Note we only implement Deref, not DerefMut,
// so you can only get read access to the bytes.
//
impl Deref for Mac {
    type Target = [u8; MAC_SIZE];

    fn deref(&self) -> &Self::Target {
        &self.0
    }
}

impl fmt::Display for Mac {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "{:02x}:{:02x}:{:02x}:{:02x}:{:02x}:{:02x}",
            self.0[0], self.0[1], self.0[2], self.0[3], self.0[4], self.0[5]
        )
    }
}
