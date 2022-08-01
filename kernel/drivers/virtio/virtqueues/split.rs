// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements the [split Virtqueue](https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html#x1-240006).
//!
//! A split [`Virtqueue`] can be used to exchange buffers with a VirtIO device. A `Virtqueue`
//! is initialised by calling its [`new`](Virtqueue::new) function, which allocates the memory
//! backing the Virtqueue, then uses the passed [`Transport`](crate::Transport) to configure
//! the device to use the Virtqueue.

use crate::features::Reserved;
use crate::{Buffer, Transport, UsedBuffers, VirtqueueError, MAX_DESCRIPTORS};
use align::align_up_usize;
use alloc::sync::Arc;
use alloc::vec::Vec;
use bitflags::bitflags;
use bitmap_index::Bitmap;
use core::mem::size_of;
use core::slice;
use core::sync::atomic::{fence, Ordering};
use memory::{PhysAddr, PhysFrameSize};
use physmem::allocate_n_frames;

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
    fn _indirect(&self) -> bool {
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

bitflags! {
    /// DeviceFlags represents the set of flags that can
    /// be used in a split virtqueue's device area's flags field.
    ///
    struct DeviceFlags: u16 {
        /// NO_NOTIFICATIONS indicates that the driver should not
        /// send notifications to the device after future descriptor
        /// chains are provided in the driver area.
        const NO_NOTIFICATIONS = 1;
    }
}

/// DeviceElem contains a reference to a
/// buffer that the device has finished using,
/// along with the number of bytes written to
/// the buffer.
///
#[repr(C)]
#[derive(Clone, Copy, Debug)]
struct DeviceElem {
    // index indicates the descriptor chain
    // head for the buffer being returned.
    index: u32,

    // len is the minimum number of bytes
    // that have been written to the buffer.
    len: u32,
}

/// DeviceArea represents a split virtqueue's area where
/// the device returns descriptors to the driver, as described
/// in section 2.6.8.
///
#[derive(Debug)]
struct DeviceArea {
    // flags indicates the device's behaviour recommendations
    // to the driver.
    _flags: &'static mut u16,

    // index is the index into ring (modulo the ring's size)
    // at which the next descriptor will be written.
    index: &'static mut u16,

    // ring is the ring buffer containing the descriptor heads
    // returned by the device.
    ring: &'static mut [DeviceElem],

    // send_event is used by the device to indicate to the driver
    // when to send notifications when future descriptors are
    // passed in the driver area.
    _send_event: &'static mut u16,
}

/// Virtqueue implements a split virtqueue, as
/// described in section 2.6.
///
pub struct Virtqueue<'desc> {
    // queue_index records which virtqueue
    // this is. The first virtqueue shared
    // with the device is number 0 and the
    // rest follow.
    queue_index: u16,

    // transport is the transport mechanism
    // used to configure and notify the
    // device.
    transport: Arc<dyn Transport>,

    // features is the set of virtio features
    // that have been negotiated with the
    // device.
    features: u64,

    // free_list is a bitmap with a bit for
    // each descriptor, indicating whether
    // the descriptor is free or currently
    // passed to the device. A descriptor
    // is free if its bit is set.
    free_list: Bitmap,

    // last_used_index stores the most recent
    // value seen in device_area.index.
    last_used_index: u16,

    // update_used_index tracks whether we
    // should keep the used_index up to date
    // as we receive used buffers.
    update_used_index: bool,

    // descriptors is the list of descriptors
    // in the descriptor area.
    descriptors: &'desc mut [Descriptor],

    // driver_area is the area used to pass
    // buffers to the device.
    driver_area: DriverArea,

    // device_area is the area used by the
    // device to return used buffers.
    device_area: DeviceArea,
}

impl<'desc> Virtqueue<'desc> {
    /// Allocates and configures a new split
    /// `Virtqueue`.
    ///
    /// `new` allocates a new split virtqueue
    /// and uses the `transport` to configure
    /// the device to use the virtqueue.
    ///
    /// The `queue_index` field indicates which
    /// Virtqueue this is, indexed from `0`.
    ///
    /// The `features` field should contain the
    /// set of VirtIO [feature flags](https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html#x1-130002)
    /// that have been negotiated with the
    /// device.
    ///
    #[allow(clippy::missing_panics_doc)] // Can only panic if we run out of memory.
    pub fn new(
        queue_index: u16,
        transport: Arc<dyn Transport>,
        features: u64,
        legacy: bool,
    ) -> Self {
        transport.select_queue(queue_index);

        let num_descriptors = if legacy {
            // Legacy devices cannot negotiate the queue
            // size.
            transport.queue_size()
        } else {
            let size = core::cmp::min(transport.queue_size(), MAX_DESCRIPTORS);
            transport.set_queue_size(size);
            size
        };

        // Calculate the size of each section of the
        // virtqueue, with the alignment requirements
        // described in section 2.6:
        //
        //  | Area       | Alignment | Size               |
        //  |------------|-----------|--------------------|
        //  | Descriptor | 16        | 16 * queue_size    |
        //  | Driver     | 2         | 6 + 2 * queue_size |
        //  | Device     | 4         | 6 + 8 * queue_size |
        //
        // For each area, we calculate the offset from
        // the frame start (which has at least KiB
        // alignment), and the size. Each area after
        // the first has an offset of the previous
        // area's offset, plus its size, aligned up
        // if necessary.
        //
        // Legacy devices have extra padding between
        // the driver area and the device area so that
        // the device area is in a different physical
        // frame from the driver area. See section 2.6.2.

        let queue_size = num_descriptors as usize;
        let descriptors_offset = 0; // Offset from frame start.
        let descriptors_size = size_of::<Descriptor>() * queue_size;
        let descriptors_end = descriptors_offset + descriptors_size;
        let driver_offset = align_up_usize(descriptors_end, 2);
        let driver_size = 6 + 2 * queue_size;
        let driver_end = driver_offset + driver_size;
        let device_offset = if legacy {
            align_up_usize(driver_end, 4096)
        } else {
            align_up_usize(driver_end, 4)
        };
        let device_size = 6 + 8 * queue_size;
        let device_end = device_offset + device_size;

        // Allocate the physical memory and map it
        // in the MMIO address space.
        let num_frames = align_up_usize(device_end, PhysFrameSize::Size4KiB.bytes())
            / PhysFrameSize::Size4KiB.bytes();
        let frame_range = allocate_n_frames(num_frames as usize)
            .expect("failed to allocate physical memory for virtqueue");
        let mmio_region = mmio::Region::map(frame_range);
        let start_phys = frame_range.start_address();
        let start_virt = mmio_region.as_mut::<u8>(0).unwrap() as *mut u8;
        let descriptors_phys = start_phys + descriptors_offset;
        let driver_phys = start_phys + driver_offset;
        let device_phys = start_phys + device_offset;
        unsafe { start_virt.write_bytes(0u8, device_end as usize) };

        // Inform the device of the virtqueue.
        if legacy {
            transport.set_queue_descriptor_area(start_phys);
        } else {
            transport.set_queue_descriptor_area(descriptors_phys);
            transport.set_queue_driver_area(driver_phys);
            transport.set_queue_device_area(device_phys);
            transport.enable_queue();
        }

        // Prepare the last bits of our state.
        let free_list = Bitmap::new_set(num_descriptors as usize);
        let last_used_index = 0;
        let descriptors = unsafe {
            slice::from_raw_parts_mut(start_virt as *mut Descriptor, num_descriptors as usize)
        };

        let driver_area = DriverArea {
            flags: mmio_region.as_mut::<u16>(driver_offset).unwrap(),
            index: mmio_region.as_mut::<u16>(driver_offset + 2).unwrap(),
            ring: unsafe {
                slice::from_raw_parts_mut(
                    mmio_region.as_mut::<u16>(driver_offset + 4).unwrap(),
                    num_descriptors as usize,
                )
            },
            recv_event: mmio_region.as_mut::<u16>(driver_end - 2).unwrap(),
        };

        let device_area = DeviceArea {
            _flags: mmio_region.as_mut::<u16>(device_offset).unwrap(),
            index: mmio_region.as_mut::<u16>(device_offset + 2).unwrap(),
            ring: unsafe {
                slice::from_raw_parts_mut(
                    mmio_region.as_mut::<DeviceElem>(device_offset + 4).unwrap(),
                    num_descriptors as usize,
                )
            },
            _send_event: mmio_region.as_mut::<u16>(device_end - 2).unwrap(),
        };

        // Enable used index updates by default if
        // we have negotiated VIRTIO_F_RING_EVENT_IDX.
        //
        // This can be disabled by calling disable_notifications.
        let update_used_index =
            Reserved::from_bits_truncate(features).contains(Reserved::RING_EVENT_IDX);

        Virtqueue {
            queue_index,
            transport,
            features,
            free_list,
            last_used_index,
            update_used_index,
            descriptors,
            driver_area,
            device_area,
        }
    }
}

impl<'desc> crate::Virtqueue for Virtqueue<'desc> {
    /// send enqueues a request to the device. A request consists of
    /// a sequence of buffers. The sequence of buffers should place
    /// device-writable buffers after all device-readable buffers.
    ///
    fn send(&mut self, buffers: &[Buffer]) -> Result<(), VirtqueueError> {
        if buffers.is_empty() || self.free_list.num_set() < buffers.len() {
            // Not enough descriptors left.
            return Err(VirtqueueError::NoDescriptors);
        }

        // TODO: Ensure writable buffers come after readable ones.

        let mut head_index: Option<u16> = None;
        let mut prev_index: Option<u16> = None;
        for (i, buffer) in buffers.iter().enumerate() {
            // Get the next free descriptor. We know we
            // have enough.
            let idx = self.free_list.next_set().unwrap();
            self.free_list.unset(idx);
            if i == 0 {
                head_index = Some(idx as u16);
            }

            let (addr, len, flags) = match buffer {
                Buffer::DeviceCanRead { addr, len } => (*addr, *len, DescriptorFlags::NONE),
                Buffer::DeviceCanWrite { addr, len } => (*addr, *len, DescriptorFlags::WRITE),
            };

            self.descriptors[idx].addr = (addr.as_usize() as u64).to_le();
            self.descriptors[idx].len = (len as u32).to_le();
            self.descriptors[idx].flags = flags.bits().to_le();
            self.descriptors[idx].next = 0;

            if let Some(prev) = prev_index {
                self.descriptors[prev as usize].flags |= DescriptorFlags::NEXT.bits().to_le();
                self.descriptors[prev as usize].next = idx as u16;
            }

            prev_index = Some(idx as u16);
        }

        // Notify the device that descriptors
        // are ready.
        let send_index = (*self.driver_area.index) as usize & (self.num_descriptors() - 1);
        self.driver_area.ring[send_index] = head_index.unwrap();
        fence(Ordering::Release);
        *self.driver_area.index = (*self.driver_area.index).wrapping_add(1);

        Ok(())
    }

    /// notify informs the device that descriptors are ready
    /// to use in this virtqueue.
    ///
    fn notify(&self) {
        fence(Ordering::Release);
        self.transport.notify_queue(self.queue_index);
    }

    /// recv returns the next set of buffers
    /// returned by the device, or None.
    ///
    fn recv(&mut self) -> Option<UsedBuffers> {
        if self.last_used_index == *self.device_area.index {
            return None;
        }

        // Get the head descriptor index.
        let head =
            self.device_area.ring[self.last_used_index as usize % self.device_area.ring.len()];
        self.last_used_index = self.last_used_index.wrapping_add(1);
        if self.update_used_index {
            *self.driver_area.recv_event = self.last_used_index;
        }

        let written = head.len as usize;
        let mut buffers = Vec::new();
        let mut next_index = head.index as u16;
        loop {
            let desc = self.descriptors[next_index as usize];
            buffers.push(if desc.writable() {
                Buffer::DeviceCanWrite {
                    addr: PhysAddr::new(desc.addr as usize),
                    len: desc.len as usize,
                }
            } else {
                Buffer::DeviceCanRead {
                    addr: PhysAddr::new(desc.addr as usize),
                    len: desc.len as usize,
                }
            });

            self.free_list.set(next_index as usize);
            if desc.has_next() {
                next_index = desc.next;
            } else {
                break;
            }
        }

        Some(UsedBuffers { buffers, written })
    }

    /// num_descriptors returns the number of descriptors
    /// in this queue.
    ///
    fn num_descriptors(&self) -> usize {
        self.descriptors.len()
    }

    /// disable_notifications requests the device not to send
    /// notifications to this queue.
    ///
    fn disable_notifications(&mut self) {
        let event_idx =
            Reserved::from_bits_truncate(self.features).contains(Reserved::RING_EVENT_IDX);
        if event_idx {
            // We simply set the used_event to an impossible
            // value.
            *self.driver_area.recv_event = u16::MAX;
            self.update_used_index = false;
        } else {
            *self.driver_area.flags |= DriverFlags::NO_NOTIFICATIONS.bits();
        }
    }

    /// enable_notifications requests the device to send
    /// notifications to this queue.
    ///
    fn enable_notifications(&mut self) {
        let event_idx =
            Reserved::from_bits_truncate(self.features).contains(Reserved::RING_EVENT_IDX);
        if event_idx {
            *self.driver_area.recv_event = self.last_used_index;
            self.update_used_index = true;
        } else {
            *self.driver_area.flags &= !DriverFlags::NO_NOTIFICATIONS.bits();
        }
    }
}
