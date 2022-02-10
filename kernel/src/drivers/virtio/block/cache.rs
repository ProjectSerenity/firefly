// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implement the caching of buffers we use to store the request header.
//!
//! Block devices are operated by sending request structures, as defined
//! in section 5.2.6. The request structure is as follows:
//!
//! ```
//! struct virtio_blk_req {
//!     le32 type;
//!     le32 reserved;
//!     le64 sector;
//!     u8 data[];
//!     u8 status;
//! }
//! ```
//!
//! The idea is that this should consist of one device-readable
//! buffer containing the first three fields, then a buffer for
//! the data (which is device-writable for reads and device-readable
//! otherwise), then a final device-writable buffer for the status
//! field.
//!
//! We simply proxy the data buffer, passing the buffer passed to
//! us by the filesystem driver, but the filesystem shouldn't need
//! to know about the rest of the request structure. That means we
//! need to have a cache of 16-byte and 1-byte buffers to use for
//! the rest of the structure.
//!
//! There are various ways we could do this very efficiently, but
//! we use a simple and reasonably efficient manner. We start by
//! allocating a frame of physical memory. We don't need to map
//! this into virtual memory, as we can use the existing mapping of
//! all physical memory mapping. It's more convenient to use a
//! physical frame, as we know it's contiguous (which simplifies
//! the construction of the buffers we send to the VirtIO driver).
//!
//! We then use a bitmap to track sequential 17-byte buffers. When
//! we want to send a request, we allocate a 17-byte buffer and use
//! the first 16 bytes for the readable header and the last byte for
//! the writable trailer.
//!
//! One frame gives us 240 concurrent requests, which should be
//! plenty, but if we do run out we just allocate another frame
//! and extend the bitmap accordingly. We don't let our 17-byte
//! buffers span more than one frame, to keep things simple. Note
//! that as we don't map the physical memory, we don't need to
//! worry about polluting the kernel memory space at runtime if we
//! end up needing to allocate another frame.

use crate::drivers::virtio::Buffer;
use alloc::vec;
use alloc::vec::Vec;
use bitmap_index::Bitmap;
use memlayout::phys_to_virt_addr;
use physmem::allocate_frame;
use x86_64::structures::paging::{PageSize, PhysFrame, Size4KiB};
use x86_64::PhysAddr;

/// The number of bytes in the header field.
///
const HEADER_SIZE: usize = (32 + 32 + 64) / 8;

/// The number of bytes in the trailer field.
///
#[allow(clippy::eq_op)]
const TRAILER_SIZE: usize = 8 / 8;

/// The number of bytes per cached buffer.
///
const BUFFER_SIZE: usize = HEADER_SIZE + TRAILER_SIZE;

/// The number of buffers per physical frame.
///
const BUFFERS_PER_FRAME: usize = Size4KiB::SIZE as usize / BUFFER_SIZE;

/// An allocator of 17-byte buffers for use as block device
/// request headers.
///
pub struct Allocator {
    bitmap: Bitmap,
    frames: Vec<PhysFrame>,
}

impl Allocator {
    /// Allocate one physical frame for header buffers.
    ///
    pub fn new() -> Self {
        let frame = allocate_frame().expect("failed to allocate block device request header cache");
        let bitmap = Bitmap::new_set(BUFFERS_PER_FRAME);
        let frames = vec![frame];

        Allocator { bitmap, frames }
    }

    /// Populates and returns the header at the given offset.
    ///
    fn get(&mut self, idx: usize, req_type: u32, sector: u64) -> (PhysAddr, Buffer, Buffer) {
        // Mark the buffer as in use and find its address.
        self.bitmap.unset(idx);
        let frame_idx = idx / BUFFERS_PER_FRAME;
        let frame = self.frames[frame_idx];
        let offset = (idx % BUFFERS_PER_FRAME) * BUFFER_SIZE;
        let phys = frame.start_address() + offset;
        let virt = phys_to_virt_addr(phys);

        // Populate the header.
        unsafe {
            #[allow(clippy::identity_op)]
            (virt + 0u64).as_mut_ptr::<u32>().write(req_type.to_le()); // le32 type;
            (virt + 4u64).as_mut_ptr::<u32>().write(0u32.to_le()); // le32 reserved;
            (virt + 8u64).as_mut_ptr::<u64>().write(sector.to_le()); // le64 sector;
        }

        let header = Buffer::DeviceCanRead {
            addr: phys,
            len: HEADER_SIZE,
        };
        let trailer = Buffer::DeviceCanWrite {
            addr: phys + HEADER_SIZE,
            len: TRAILER_SIZE,
        };

        (phys, header, trailer)
    }

    /// Returns a pair of VirtIO buffers for the given
    /// request type and sector number. The first buffer
    /// is the request header, which should prepend a
    /// request. The second is the status field, which
    /// should append the request.
    ///
    /// `allocate` also returns the physical address of
    /// the start of the buffer. The two VirtIO buffers
    /// are guaranteed to contain seventeen contiguous
    /// bytes, starting at the given physical address.
    ///
    /// Note that `allocate` will ensure the given fields
    /// are stored in the correct endianness, so they
    /// should be passed as natural values.
    ///
    pub fn allocate(&mut self, req_type: u32, sector: u64) -> (PhysAddr, Buffer, Buffer) {
        match self.bitmap.next_set() {
            Some(idx) => self.get(idx, req_type, sector),
            None => {
                // Allocate another frame, then try again.
                let frame =
                    allocate_frame().expect("failed to allocate block device request header cache");
                self.frames.push(frame);
                self.bitmap.add_set(BUFFERS_PER_FRAME);
                let idx = self
                    .bitmap
                    .next_set()
                    .expect("internal error: failed to find space in new frame");

                self.get(idx, req_type, sector)
            }
        }
    }

    /// Adds a used buffer pair to the allocator, returning
    /// the status code in the request trailer.
    ///
    /// # Panics
    ///
    /// `deallocate` will panic if the given buffer pair
    /// was not returned by the same call to [`allocate`](Allocator::allocate).
    ///
    pub fn deallocate(&mut self, buffer: PhysAddr) -> u8 {
        // Store the status code.
        let trailer_virt = phys_to_virt_addr(buffer + HEADER_SIZE);
        let status = unsafe { trailer_virt.as_ptr::<u8>().read() };

        // Work out the offset and deallocate.
        for (i, frame) in self.frames.iter().enumerate() {
            let start = frame.start_address();
            if start > buffer || buffer > (start + frame.size()) {
                continue;
            }

            let frame_idx = i * BUFFERS_PER_FRAME;
            let offset = (buffer - start) as usize / BUFFER_SIZE;
            self.bitmap.set(frame_idx + offset);

            return status;
        }

        // This buffer does not exist in the frames
        // we have allocated.
        panic!("buffers passed to deallocate were not returned by allocate");
    }
}

impl Default for Allocator {
    fn default() -> Self {
        Self::new()
    }
}
