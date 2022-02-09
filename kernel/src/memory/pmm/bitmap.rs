// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides a bitmap frame allocator, which can be used to allocate
//! and deallocate physical memory frames.

use crate::memory::pmm::boot_info::BootInfoFrameAllocator;
use crate::println;
use crate::utils::pretty::Bytes;
use alloc::vec::Vec;
use bitmap_index::Bitmap;
use bootloader::bootinfo::{MemoryRegion, MemoryRegionType};
use core::slice::Iter;
use x86_64::structures::paging::frame::PhysFrameRange;
use x86_64::structures::paging::{FrameAllocator, FrameDeallocator, PhysFrame, Size4KiB};
use x86_64::PhysAddr;

/// FRAME_SIZE is the size of a single frame of
/// physical memory.
///
const FRAME_SIZE: usize = 4096;

/// A single contiguous chunk of physical memory, which is
/// tracked using a bitmap.
///
struct BitmapPool {
    // start_address is the address of the first
    // frame in the pool. start_address is guaranteed
    // to be frame-aligned.
    //
    pub start_address: PhysAddr,

    // last_address is the largest address that is
    // within the pool.
    //
    pub last_address: PhysAddr,

    // num_frames is the number of 4 kiB frames in
    // this pool.
    //
    pub num_frames: u64,

    // free_frames is the number of 4 kiB frames in
    // this pool that have not been allocated. There
    // is no guarantee that the free frames will be
    // consecutive.
    //
    pub free_frames: u64,

    // bitmap is a compact representation of the frames
    // in this pool and whether each is free. For frame
    // n (where the frame with the starting address at
    // self.start_address is frame 0), the nth most
    // significant bit in bitmap will be 1 if the frame
    // is free and 0 if the frame has been allocated.
    //
    bitmap: Bitmap,
}

impl BitmapPool {
    /// new returns a BitmapPool representing the given
    /// memory region.
    ///
    pub fn new(region: &MemoryRegion) -> Self {
        if region.region_type != MemoryRegionType::Usable {
            panic!(
                "cannot make new frame pool from memory region with non-Usable type {:?}",
                region.region_type
            );
        }

        let num_frames = region.range.end_frame_number - region.range.start_frame_number;
        BitmapPool {
            start_address: PhysAddr::new(region.range.start_addr()),
            last_address: PhysAddr::new(region.range.end_addr() - 1),
            num_frames,
            free_frames: num_frames,
            bitmap: Bitmap::new_set(num_frames as usize),
        }
    }

    /// frame_at returns the physical frame at the given
    /// index.
    ///
    fn frame_at(&self, index: usize) -> PhysFrame {
        PhysFrame::from_start_address(self.start_address + index * FRAME_SIZE).unwrap()
    }

    /// index_for returns the index at which the given
    /// physical address exists, or None.
    ///
    fn index_for(&self, addr: PhysAddr) -> Option<usize> {
        if addr < self.start_address || self.last_address < addr {
            return None;
        }

        if addr == self.start_address {
            return Some(0);
        }

        Some(((addr - self.start_address) as usize) / FRAME_SIZE)
    }

    /// contains_frame returns whether the pool includes
    /// the given frame.
    ///
    pub fn contains_frame(&self, frame: PhysFrame<Size4KiB>) -> bool {
        let start_addr = frame.start_address();
        self.start_address <= start_addr && start_addr < self.last_address
    }

    /// allocate_frame returns the next free frame,
    /// or None.
    ///
    pub fn allocate_frame(&mut self) -> Option<PhysFrame> {
        if self.free_frames == 0 {
            return None;
        }

        match self.bitmap.next_set() {
            None => None,
            Some(index) => {
                self.bitmap.unset(index);
                self.free_frames -= 1;
                Some(self.frame_at(index))
            }
        }
    }

    /// allocate_n_frames returns n sequential free frames,
    /// or None.
    ///
    pub fn allocate_n_frames(&mut self, n: usize) -> Option<PhysFrameRange> {
        if n == 0 || self.free_frames == 0 {
            return None;
        }

        match self.bitmap.next_n_set(n) {
            None => None,
            Some(index) => {
                for i in 0..n {
                    self.bitmap.unset(index + i);
                }

                self.free_frames -= n as u64;
                let start = self.frame_at(index);
                let end = self.frame_at(index + n);
                Some(PhysFrame::range(start, end))
            }
        }
    }

    /// mark_frame_allocated marks the given frame as
    /// allocated.
    ///
    pub fn mark_frame_allocated(&mut self, frame: PhysFrame<Size4KiB>) {
        let start_addr = frame.start_address();
        match self.index_for(start_addr) {
            None => panic!("cannot mark frame at {:p}: frame not tracked", start_addr),
            Some(i) => {
                if !self.bitmap.get(i) {
                    panic!(
                        "cannot mark frame at {:p}: frame already marked allocated",
                        start_addr
                    );
                }

                self.bitmap.unset(i);
                self.free_frames -= 1;
            }
        }
    }

    /// deallocate_frame marks the given frame as free
    /// for use.
    ///
    pub fn deallocate_frame(&mut self, frame: PhysFrame<Size4KiB>) {
        let start_addr = frame.start_address();
        match self.index_for(start_addr) {
            None => panic!(
                "cannot deallocate frame at {:p}: frame not tracked",
                start_addr
            ),
            Some(i) => {
                if self.bitmap.get(i) {
                    panic!(
                        "cannot deallocate frame at {:p}: frame already free",
                        start_addr
                    );
                }

                self.bitmap.set(i);
                self.free_frames += 1;
            }
        }
    }
}

/// A more sophisticated physical memory allocator.
///
/// `BitmapFrameAllocator` takes over from the [`BootInfoFrameAllocator`](super::BootInfoFrameAllocator)
/// once the kernel's heap has been initialised.
///
pub struct BitmapFrameAllocator {
    // num_frames is the number of 4 kiB frames in
    // this pool.
    //
    pub num_frames: u64,

    // free_frames is the number of 4 kiB frames in
    // this pool that have not been allocated. There
    // is no guarantee that the free frames will be
    // consecutive.
    //
    pub free_frames: u64,

    // pools contains the bitmap data for each pool
    // of contiguous frames.
    //
    pools: Vec<BitmapPool>,
}

impl BitmapFrameAllocator {
    /// Returns an empty allocator, which can allocate no memory.
    ///
    pub fn empty() -> Self {
        BitmapFrameAllocator {
            num_frames: 0,
            free_frames: 0,
            pools: Vec::new(),
        }
    }

    /// Creates a BitmapFrameAllocator from the passed memory map.
    ///
    /// # Safety
    ///
    /// This function is unsafe because the caller must guarantee that the
    /// memory map is valid and complete. All frames that are marked as `USABLE`
    /// in the memory map must be unused.
    ///
    pub unsafe fn new(regions: Iter<MemoryRegion>) -> Self {
        // Start out by determining the set of
        // available pools.
        let usable_regions = regions.filter(|r| {
            r.region_type == MemoryRegionType::Usable
                && r.range.start_frame_number < r.range.end_frame_number
        });

        let pools: Vec<BitmapPool> = usable_regions.map(BitmapPool::new).collect();
        let mut num_frames = 0u64;
        let mut free_frames = 0u64;
        for pool in pools.iter() {
            num_frames += pool.num_frames;
            free_frames += pool.free_frames;
        }

        BitmapFrameAllocator {
            num_frames,
            free_frames,
            pools,
        }
    }

    /// Returns `n` sequential free frames, or `None`.
    ///
    /// It's possible that `n` frames may be available, but `allocate_n_frames`
    /// still return `None`. The bitmap allocator must be able to return `n`
    /// frames in a single contiguous sequence for it to succeed.
    ///
    pub fn allocate_n_frames(&mut self, n: usize) -> Option<PhysFrameRange> {
        for pool in self.pools.iter_mut() {
            if let Some(range) = pool.allocate_n_frames(n) {
                self.free_frames -= n as u64;
                return Some(range);
            }
        }

        None
    }

    /// Marks the given frame as already allocated.
    ///
    fn mark_frame_allocated(&mut self, frame: PhysFrame<Size4KiB>) {
        for pool in self.pools.iter_mut() {
            if pool.contains_frame(frame) {
                pool.mark_frame_allocated(frame);
                self.free_frames -= 1;
                return;
            }
        }

        let start_addr = frame.start_address();
        panic!("cannot mark frame at {:p}: frame not tracked", start_addr);
    }

    /// Takes ownership of the given [`BootInfoFrameAllocator`](super::BootInfoFrameAllocator),
    /// along with any frames it has already allocated, allowing them to be freed using
    /// `deallocate_frame`.
    ///
    /// # Safety
    ///
    /// This function is unsafe because the caller must guarantee that the
    /// memory map is valid and complete. All frames that are marked as `USABLE`
    /// in the memory map must be unused.
    ///
    pub unsafe fn repossess(&mut self, alloc: BootInfoFrameAllocator) {
        for frame in alloc.used_frames() {
            self.mark_frame_allocated(frame);
        }
    }

    /// Prints debug information about the allocator's state.
    ///
    pub fn debug(&self) {
        println!(
            "Physical memory manager: {}/{} frames available.",
            self.free_frames, self.num_frames
        );
        println!(
            "{} used, {} free, {} total",
            Bytes::from_u64((self.num_frames - self.free_frames) * 4096),
            Bytes::from_u64(self.free_frames * 4096),
            Bytes::from_u64(self.num_frames * 4096)
        );
        for pool in self.pools.iter() {
            println!(
                "{:#011x}-{:#011x} {:5} x {} frame = {:7}, {:5} x free frames = {:7}",
                pool.start_address,
                pool.last_address,
                pool.num_frames,
                Bytes::from_u64(4096),
                Bytes::from_u64(4096 * pool.num_frames),
                pool.free_frames,
                Bytes::from_u64(pool.free_frames * 4096)
            );
        }
    }
}

unsafe impl FrameAllocator<Size4KiB> for BitmapFrameAllocator {
    /// Returns the next available physical frame, or `None`.
    ///
    fn allocate_frame(&mut self) -> Option<PhysFrame> {
        for pool in self.pools.iter_mut() {
            if let Some(frame) = pool.allocate_frame() {
                self.free_frames -= 1;
                return Some(frame);
            }
        }

        None
    }
}

impl FrameDeallocator<Size4KiB> for BitmapFrameAllocator {
    /// Marks the given physical memory frame as unused and returns it to the
    /// list of free frames for later use.
    ///
    /// # Safety
    ///
    /// The caller must ensure that `frame` is unused.
    ///
    unsafe fn deallocate_frame(&mut self, frame: PhysFrame<Size4KiB>) {
        for pool in self.pools.iter_mut() {
            if pool.contains_frame(frame) {
                pool.deallocate_frame(frame);
                self.free_frames += 1;
                return;
            }
        }

        let start_addr = frame.start_address();
        panic!(
            "cannot dallocate frame at {:p}: frame not tracked",
            start_addr
        );
    }
}

#[test_case]
fn bitmap_frame_allocator() {
    use bootloader::bootinfo::FrameRange;
    let regions = [
        MemoryRegion {
            range: FrameRange {
                start_frame_number: 0u64,
                end_frame_number: 1u64,
            },
            region_type: MemoryRegionType::FrameZero,
        },
        MemoryRegion {
            range: FrameRange {
                start_frame_number: 1u64,
                end_frame_number: 4u64,
            },
            region_type: MemoryRegionType::Reserved,
        },
        MemoryRegion {
            range: FrameRange {
                start_frame_number: 4u64,
                end_frame_number: 8u64,
            },
            region_type: MemoryRegionType::Usable,
        },
        MemoryRegion {
            range: FrameRange {
                start_frame_number: 8u64,
                end_frame_number: 12u64,
            },
            region_type: MemoryRegionType::Reserved,
        },
        MemoryRegion {
            range: FrameRange {
                start_frame_number: 12u64,
                end_frame_number: 14u64,
            },
            region_type: MemoryRegionType::Usable,
        },
    ];

    let mut alloc = unsafe { BitmapFrameAllocator::new(regions.iter()) };
    assert_eq!(alloc.num_frames, 6u64);
    assert_eq!(alloc.free_frames, 6u64);

    // Helper function to speed up making frames.
    fn frame_for(addr: u64) -> PhysFrame {
        let start_addr = PhysAddr::new(addr);
        let frame = PhysFrame::from_start_address(start_addr).unwrap();
        frame
    }

    // Do some allocations.
    assert_eq!(alloc.allocate_frame(), Some(frame_for(0x4000)));
    assert_eq!(alloc.num_frames, 6u64);
    assert_eq!(alloc.free_frames, 5u64);
    assert_eq!(alloc.allocate_frame(), Some(frame_for(0x5000)));
    assert_eq!(alloc.num_frames, 6u64);
    assert_eq!(alloc.free_frames, 4u64);

    // Do a free.
    unsafe { alloc.deallocate_frame(frame_for(0x4000)) };
    assert_eq!(alloc.num_frames, 6u64);
    assert_eq!(alloc.free_frames, 5u64);

    // Next allocation should return the address we just freed.
    assert_eq!(alloc.allocate_frame(), Some(frame_for(0x4000)));
    assert_eq!(alloc.num_frames, 6u64);
    assert_eq!(alloc.free_frames, 4u64);

    // Check that all remaining allocations are as we expect.
    assert_eq!(alloc.allocate_frame(), Some(frame_for(0x6000)));
    assert_eq!(alloc.allocate_frame(), Some(frame_for(0x7000)));
    assert_eq!(alloc.allocate_frame(), Some(frame_for(0xc000)));
    assert_eq!(alloc.allocate_frame(), Some(frame_for(0xd000)));
    assert_eq!(alloc.num_frames, 6u64);
    assert_eq!(alloc.free_frames, 0u64);

    // Check that we get nothing once we run out of frames.
    assert_eq!(alloc.allocate_frame(), None);
    assert_eq!(alloc.num_frames, 6u64);
    assert_eq!(alloc.free_frames, 0u64);

    // Check that sequential allocations work correctly.

    // Deallocate 2 non-sequential frames, expect None.
    unsafe { alloc.deallocate_frame(frame_for(0x5000)) };
    unsafe { alloc.deallocate_frame(frame_for(0x7000)) };
    assert_eq!(alloc.allocate_n_frames(2), None);

    // Leave 2 sequential frames, check we get the right pair.
    // Note: we use PhysFrameRange, not PhysFrameRangeInclusive.
    assert_eq!(alloc.allocate_frame(), Some(frame_for(0x5000)));
    unsafe { alloc.deallocate_frame(frame_for(0x6000)) };
    assert_eq!(
        alloc.allocate_n_frames(2),
        Some(PhysFrameRange {
            start: frame_for(0x6000),
            end: frame_for(0x8000) // exclusive
        })
    );

    // Check that we get nothing once we run out of frames.
    assert_eq!(alloc.num_frames, 6u64);
    assert_eq!(alloc.free_frames, 0u64);
}
