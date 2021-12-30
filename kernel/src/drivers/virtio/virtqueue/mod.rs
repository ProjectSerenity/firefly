//! virtqueue implements Virtio Virtqueues, as described in
//! section 2.5 of <https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html>.

use alloc::vec::Vec;
use x86_64::PhysAddr;

/// Buffer represents a contiguous sequence of
/// physical memory, which can either be read
/// or written by the device.
///
pub enum Buffer {
    DeviceCanRead { addr: PhysAddr, len: usize },
    DeviceCanWrite { addr: PhysAddr, len: usize },
}

/// UsedBuffers contains a set of buffers that
/// the device has returned, along with the number
/// of bytes written to them.
///
pub struct UsedBuffers {
    pub buffers: Vec<Buffer>,
    pub written: usize,
}
