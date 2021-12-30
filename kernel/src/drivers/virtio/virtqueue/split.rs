//! split implements split virtqueues, as described in section
//! 2.6.

use crate::drivers::virtio;
use crate::drivers::virtio::{virtqueue, Transport};
use crate::memory::{mmio, pmm};
use crate::utils::bitmap;
use alloc::sync::Arc;
use alloc::vec::Vec;
use bitflags::bitflags;
use core::mem::size_of;
use core::slice;
use core::sync::atomic::{fence, Ordering};
use x86_64::structures::paging::{PageSize, Size4KiB};
use x86_64::{align_up, PhysAddr};

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
    flags: &'static mut u16,

    // index is the index into ring (modulo the ring's size)
    // at which the next descriptor will be written.
    index: &'static mut u16,

    // ring is the ring buffer containing the descriptor heads
    // returned by the device.
    ring: &'static mut [DeviceElem],

    // send_event is used by the device to indicate to the driver
    // when to send notifications when future descriptors are
    // passed in the driver area.
    send_event: &'static mut u16,
}

/// Virtqueue implements a split virtqueue, as
/// described in section 2.6.
///
pub struct Virtqueue<'a> {
    // queue_index records which virtqueue
    // this is. The first virtqueue shared
    // with the device is number 0 and the
    // rest follow.
    queue_index: u16,

    // transport is the transport mechanism
    // used to configure and notify the
    // device.
    transport: Arc<dyn Transport>,

    // free_list is a bitmap with a bit for
    // each descriptor, indicating whether
    // the descriptor is free or currently
    // passed to the device. A descriptor
    // is free if its bit is set.
    free_list: bitmap::Bitmap,

    // last_used_index stores the most recent
    // value seen in device_area.index.
    last_used_index: u16,

    // descriptors is the list of descriptors
    // in the descriptor area.
    descriptors: &'a mut [Descriptor],

    // driver_area is the area used to pass
    // buffers to the device.
    driver_area: DriverArea,

    // device_area is the area used by the
    // device to return used buffers.
    device_area: DeviceArea,
}

impl<'a> Virtqueue<'a> {
    /// new allocates a new split virtqueue
    /// and uses the transport to configure
    /// the device to use the virtqueue.
    ///
    /// The queue_index field indicates which
    /// virtqueue this is.
    ///
    pub fn new(queue_index: u16, transport: Arc<dyn Transport>) -> Self {
        transport.select_queue(queue_index);

        let num_descriptors = core::cmp::min(transport.queue_size(), virtio::MAX_DESCRIPTORS);
        transport.set_queue_size(num_descriptors);

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

        let queue_size = num_descriptors as u64;
        let descriptors_offset = 0 as u64; // Offset from frame start.
        let descriptors_size = size_of::<Descriptor>() as u64 * queue_size;
        let descriptors_end = descriptors_offset + descriptors_size;
        let driver_offset = align_up(descriptors_end, 2);
        let driver_size = 6u64 + 2u64 * queue_size;
        let driver_end = driver_offset + driver_size;
        let device_offset = align_up(driver_end, 4);
        let device_size = 6u64 + 8 * queue_size;
        let device_end = device_offset + device_size;

        // Allocate the physical memory and map it
        // in the MMIO address space.
        let num_frames = align_up(device_end, Size4KiB::SIZE) / Size4KiB::SIZE;
        let frame_range = pmm::allocate_n_frames(num_frames as usize)
            .expect("failed to allocate physical memory for virtqueue");
        let mmio_region = unsafe { mmio::Region::map(frame_range) };
        let start_phys = frame_range.start.start_address();
        let start_virt = mmio_region.as_mut::<u8>(0).unwrap() as *mut u8;
        let descriptors_phys = start_phys + descriptors_offset;
        let driver_phys = start_phys + driver_offset;
        let device_phys = start_phys + device_offset;
        unsafe { start_virt.write_bytes(0u8, device_end as usize) };

        // Inform the device of the virtqueue.
        transport.set_queue_descriptor_area(descriptors_phys);
        transport.set_queue_driver_area(driver_phys);
        transport.set_queue_device_area(device_phys);
        transport.enable_queue();

        // Prepare the last bits of our state.
        let free_list = bitmap::Bitmap::new_set(num_descriptors as usize);
        let last_used_index = 0;
        let descriptors = unsafe {
            slice::from_raw_parts_mut(start_virt as *mut Descriptor, num_descriptors as usize)
        };

        let driver_area = DriverArea {
            flags: mmio_region.as_mut::<u16>(driver_offset + 0).unwrap(),
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
            flags: mmio_region.as_mut::<u16>(device_offset + 0).unwrap(),
            index: mmio_region.as_mut::<u16>(device_offset + 2).unwrap(),
            ring: unsafe {
                slice::from_raw_parts_mut(
                    mmio_region.as_mut::<DeviceElem>(device_offset + 4).unwrap(),
                    num_descriptors as usize,
                )
            },
            send_event: mmio_region.as_mut::<u16>(device_end - 2).unwrap(),
        };

        Virtqueue {
            queue_index,
            transport,
            free_list,
            last_used_index,
            descriptors,
            driver_area,
            device_area,
        }
    }
}

impl<'a> virtqueue::Virtqueue for Virtqueue<'a> {
    /// send enqueues a request to the device. A request consists of
    /// a sequence of buffers. The sequence of buffers should place
    /// device-writable buffers after all device-readable buffers.
    ///
    fn send(&mut self, buffers: &[virtqueue::Buffer]) -> Result<(), virtqueue::Error> {
        if buffers.is_empty() || self.free_list.num_set() < buffers.len() {
            // Not enough descriptors left.
            return Err(virtqueue::Error::NoDescriptors);
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
                virtqueue::Buffer::DeviceCanRead { addr, len } => {
                    (*addr, *len, DescriptorFlags::NONE)
                }
                virtqueue::Buffer::DeviceCanWrite { addr, len } => {
                    (*addr, *len, DescriptorFlags::WRITE)
                }
            };

            self.descriptors[idx].addr = addr.as_u64().to_le();
            self.descriptors[idx].len = (len as u32).to_le();
            self.descriptors[idx].flags = flags.bits().to_le();
            self.descriptors[idx].next = 0;

            match prev_index {
                Some(prev) => {
                    self.descriptors[prev as usize].flags |= DescriptorFlags::NEXT.bits().to_le();
                    self.descriptors[prev as usize].next = idx as u16;
                }
                _ => {}
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
    fn recv(&mut self) -> Option<virtqueue::UsedBuffers> {
        if self.last_used_index == *self.device_area.index {
            return None;
        }

        // Get the head descriptor index.
        let head =
            self.device_area.ring[self.last_used_index as usize % self.device_area.ring.len()];
        self.last_used_index = self.last_used_index.wrapping_add(1);
        *self.driver_area.recv_event = self.last_used_index;

        let written = head.len as usize;
        let mut buffers = Vec::new();
        let mut next_index = head.index as u16;
        loop {
            let desc = self.descriptors[next_index as usize];
            buffers.push(if desc.writable() {
                virtqueue::Buffer::DeviceCanWrite {
                    addr: PhysAddr::new(desc.addr as u64),
                    len: desc.len as usize,
                }
            } else {
                virtqueue::Buffer::DeviceCanRead {
                    addr: PhysAddr::new(desc.addr as u64),
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

        Some(virtqueue::UsedBuffers { buffers, written })
    }

    /// num_descriptors returns the number of descriptors
    /// in this queue.
    ///
    fn num_descriptors(&self) -> usize {
        self.descriptors.len()
    }
}
