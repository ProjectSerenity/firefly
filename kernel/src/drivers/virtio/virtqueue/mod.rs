//! virtqueue implements Virtio Virtqueues, as described in
//! section 2.5 of <https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html>.

pub mod split;

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

/// Error represents an error interacting with a
/// virtqueue.
///
#[derive(Clone, Copy, Debug)]
pub enum Error {
    /// No descriptors were available for sending
    /// a buffer to the queue.
    NoDescriptors,
}

/// Virtqueue abstracts the implementation details
/// of a virtqueue.
///
pub trait Virtqueue: Send {
    /// send enqueues a request to the device. A request consists of
    /// a sequence of buffers. The sequence of buffers should place
    /// device-writable buffers after all device-readable buffers.
    ///
    /// send returns the descriptor index for the head of the chain.
    /// This can be used to identify when the device returns the
    /// buffer chain. If there are not enough descriptors to send
    /// the chain, send returns None.
    ///
    fn send(&mut self, buffers: &[Buffer]) -> Result<(), Error>;

    /// notify informs the device that descriptors are ready
    /// to use in this virtqueue.
    ///
    fn notify(&self);

    /// recv returns the next set of buffers
    /// returned by the device, or None.
    ///
    fn recv(&mut self) -> Option<UsedBuffers>;

    /// num_descriptors returns the number of descriptors
    /// in this queue.
    ///
    fn num_descriptors(&self) -> usize;

    /// disable_notifications requests the device not to send
    /// notifications to this queue.
    ///
    fn disable_notifications(&mut self);

    /// enable_notifications requests the device to send
    /// notifications to this queue.
    ///
    fn enable_notifications(&mut self);
}
