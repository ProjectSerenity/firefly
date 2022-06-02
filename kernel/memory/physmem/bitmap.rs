// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides a bitmap frame allocator, which can be used to allocate
//! and deallocate physical memory frames.

use crate::boot_info::BootInfoFrameAllocator;
use alloc::vec::Vec;
use bitmap_index::Bitmap;
use bootinfo::{MemoryRegion, MemoryRegionType};
use core::slice::Iter;
use memory::{
    PhysAddr, PhysFrame, PhysFrameAllocator, PhysFrameDeallocator, PhysFrameRange, PhysFrameSize,
};
use pretty::Bytes;
use serial::println;

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
    pub num_frames: usize,

    // free_frames is the number of 4 kiB frames in
    // this pool that have not been allocated. There
    // is no guarantee that the free frames will be
    // consecutive.
    //
    pub free_frames: usize,

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
            start_address: PhysAddr::new(region.range.start_addr() as usize),
            last_address: PhysAddr::new(region.range.end_addr() as usize - 1),
            num_frames: num_frames as usize,
            free_frames: num_frames as usize,
            bitmap: Bitmap::new_set(num_frames as usize),
        }
    }

    /// frame_at returns the physical frame at the given
    /// index.
    ///
    fn frame_at(&self, index: usize) -> PhysFrame {
        let size = PhysFrameSize::Size4KiB;
        PhysFrame::from_start_address(self.start_address + index * size.bytes(), size).unwrap()
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

        Some(((addr - self.start_address) as usize) / PhysFrameSize::Size4KiB.bytes())
    }

    /// contains_frame returns whether the pool includes
    /// the given frame.
    ///
    pub fn contains_frame(&self, frame: PhysFrame) -> bool {
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

                self.free_frames -= n;
                let start = self.frame_at(index);
                let end = self.frame_at(index + n);
                Some(PhysFrame::range_exclusive(start, end))
            }
        }
    }

    /// mark_frame_allocated marks the given frame as
    /// allocated.
    ///
    pub fn mark_frame_allocated(&mut self, frame: PhysFrame) {
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

    /// Returns whether the given frame is marked as
    /// allocated in the pool.
    ///
    pub fn is_allocated(&self, frame: PhysFrame) -> bool {
        let start_addr = frame.start_address();
        match self.index_for(start_addr) {
            None => panic!("frame at {:p} not tracked", start_addr),
            Some(i) => !self.bitmap.get(i),
        }
    }

    /// deallocate_frame marks the given frame as free
    /// for use.
    ///
    pub fn deallocate_frame(&mut self, frame: PhysFrame) {
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

    /// deallocate_next_frame returns the next frame
    /// marked as allocated and marks it free, or it
    /// returns None.
    ///
    pub fn deallocate_next_frame(&mut self) -> Option<PhysFrame> {
        if self.free_frames == self.num_frames {
            return None;
        }

        match self.bitmap.next_unset() {
            None => None,
            Some(index) => {
                self.bitmap.set(index);
                self.free_frames += 1;
                Some(self.frame_at(index))
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
    pub num_frames: usize,

    // free_frames is the number of 4 kiB frames in
    // this pool that have not been allocated. There
    // is no guarantee that the free frames will be
    // consecutive.
    //
    pub free_frames: usize,

    // pools contains the bitmap data for each pool
    // of contiguous frames.
    //
    pools: Vec<BitmapPool>,
}

impl BitmapFrameAllocator {
    /// Returns an empty allocator, which can allocate no memory.
    ///
    pub const fn empty() -> Self {
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
        let mut num_frames = 0;
        let mut free_frames = 0;
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

    /// Creates an allocation tracker for the same set of physical frames
    /// as managed by the allocator. All of these frames are initially
    /// marked as free in the returned tracker, so that it will only track
    /// specific allocations chosen by the caller.
    ///
    pub fn new_tracker(&self) -> BitmapFrameTracker {
        let mut pools = Vec::with_capacity(self.pools.len());
        for pool in self.pools.iter() {
            pools.push(BitmapPool {
                start_address: pool.start_address,
                last_address: pool.last_address,
                num_frames: pool.num_frames,

                // All frames start out free.
                free_frames: pool.num_frames,
                bitmap: Bitmap::new_set(pool.num_frames as usize),
            });
        }

        let num_frames = self.num_frames;
        let allocated_frames = 0;

        BitmapFrameTracker {
            num_frames,
            allocated_frames,
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
                self.free_frames -= n;
                return Some(range);
            }
        }

        None
    }

    /// Marks the given frame as already allocated.
    ///
    fn mark_frame_allocated(&mut self, frame: PhysFrame) {
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
            Bytes::from_usize((self.num_frames - self.free_frames) * 4096),
            Bytes::from_usize(self.free_frames * 4096),
            Bytes::from_usize(self.num_frames * 4096)
        );
        for pool in self.pools.iter() {
            println!(
                "{:#011x}-{:#011x} {:5} x {} frame = {:7}, {:5} x free frames = {:7}",
                pool.start_address,
                pool.last_address,
                pool.num_frames,
                Bytes::from_usize(4096),
                Bytes::from_usize(4096 * pool.num_frames),
                pool.free_frames,
                Bytes::from_usize(pool.free_frames * 4096)
            );
        }
    }
}

unsafe impl PhysFrameAllocator for BitmapFrameAllocator {
    /// Returns the next available physical frame, or `None`.
    ///
    fn allocate_phys_frame(&mut self, size: PhysFrameSize) -> Option<PhysFrame> {
        // For now, we only support 4 KiB frames.
        if size != PhysFrameSize::Size4KiB {
            return None;
        }

        for pool in self.pools.iter_mut() {
            if let Some(frame) = pool.allocate_frame() {
                self.free_frames -= 1;
                return Some(frame);
            }
        }

        None
    }
}

impl PhysFrameDeallocator for BitmapFrameAllocator {
    /// Marks the given physical memory frame as unused and returns it to the
    /// list of free frames for later use.
    ///
    /// # Safety
    ///
    /// The caller must ensure that `frame` is unused.
    ///
    unsafe fn deallocate_phys_frame(&mut self, frame: PhysFrame) {
        // For now, we only support 4 KiB, so we
        // won't have allocated anything else.
        if frame.size() != PhysFrameSize::Size4KiB {
            panic!("cannot deallocate {:?}: frame not tracked", frame);
        }

        for pool in self.pools.iter_mut() {
            if pool.contains_frame(frame) {
                pool.deallocate_frame(frame);
                self.free_frames += 1;
                return;
            }
        }

        let start_addr = frame.start_address();
        panic!(
            "cannot deallocate frame at {:p}: frame not tracked",
            start_addr
        );
    }
}

/// A helper for tracking the allocations for a single
/// purpose. This is used alongside a normal allocator,
/// which performs the actual allocations.
///
/// The idea is that in combination with an [`ArenaFrameAllocator`],
/// it becomes easy to deallocate all memory allocated
/// by a single arena, such as a single process.
///
pub struct BitmapFrameTracker {
    // num_frames is the number of 4 kiB frames in
    // this pool.
    //
    pub num_frames: usize,

    // allocated_frames is the number of 4 kiB frames in
    // this arena that have been allocated.
    //
    pub allocated_frames: usize,

    // pools contains the bitmap data for each pool
    // of contiguous frames.
    //
    pools: Vec<BitmapPool>,
}

impl BitmapFrameTracker {
    /// Returns whether the given frame is tracked in this
    /// arena.
    ///
    pub fn is_allocated(&self, frame: PhysFrame) -> bool {
        for pool in self.pools.iter() {
            if pool.contains_frame(frame) {
                return pool.is_allocated(frame);
            }
        }

        false
    }

    /// Mark the given frame as allocated in this arena.
    ///
    /// # Panics
    ///
    /// `allocate_frame` will panic if `frame` is not in
    /// the set of physical memory tracked by this allocator.
    ///
    pub fn allocate_frame(&mut self, frame: PhysFrame) {
        for pool in self.pools.iter_mut() {
            if pool.contains_frame(frame) {
                pool.mark_frame_allocated(frame);
                self.allocated_frames += 1;
                return;
            }
        }

        let start_addr = frame.start_address();
        panic!("cannot mark frame at {:p}: frame not tracked", start_addr);
    }

    /// Mark the given frame as not allocated in this
    /// arena.
    ///
    /// # Panics
    ///
    /// `deallocate_frame` will panic if `frame` was not
    /// returned by this allocator.
    ///
    pub fn deallocate_frame(&mut self, frame: PhysFrame) {
        for pool in self.pools.iter_mut() {
            if pool.contains_frame(frame) {
                pool.deallocate_frame(frame);
                self.allocated_frames -= 1;
                return;
            }
        }

        let start_addr = frame.start_address();
        panic!(
            "cannot dallocate frame at {:p}: frame not tracked",
            start_addr
        );
    }

    /// Returns the first allocated frame and marks
    /// it as not allocated. This is designed to be
    /// used in a loop to deallocate all memory in
    /// the arena
    ///
    unsafe fn deallocate_next_frame(&mut self) -> Option<PhysFrame> {
        for pool in self.pools.iter_mut() {
            if let Some(frame) = pool.deallocate_next_frame() {
                self.allocated_frames -= 1;
                return Some(frame);
            }
        }

        None
    }
}

/// A helper for tracking allocations for a single
/// purpose. This combines a normal allocator with
/// an allocation tracker to perform allocations,
/// keeping a record of those allocations. This can
/// then be used to deallocate all memory used in
/// the arena.
///
pub struct ArenaFrameAllocator<'allocator, 'tracker, A: PhysFrameAllocator + PhysFrameDeallocator> {
    tracker: &'tracker mut BitmapFrameTracker,
    allocator: &'allocator mut A,
}

impl<'allocator, 'tracker, A: PhysFrameAllocator + PhysFrameDeallocator>
    ArenaFrameAllocator<'allocator, 'tracker, A>
{
    /// Returns a new frame allocator, which will
    /// defer to allocator for `allocations`, then
    /// record them in `tracker`.
    ///
    pub fn new(allocator: &'allocator mut A, tracker: &'tracker mut BitmapFrameTracker) -> Self {
        ArenaFrameAllocator { tracker, allocator }
    }

    /// Deallocate all memory allocated for this
    /// arena.
    ///
    /// # Safety
    ///
    /// The caller must ensure that `frame` is unused.
    ///
    pub unsafe fn deallocate_all_frames(&mut self) {
        loop {
            if let Some(frame) = self.tracker.deallocate_next_frame() {
                self.allocator.deallocate_phys_frame(frame);
            } else {
                return;
            }
        }
    }
}

unsafe impl<'allocator, 'tracker, A: PhysFrameAllocator + PhysFrameDeallocator> PhysFrameAllocator
    for ArenaFrameAllocator<'allocator, 'tracker, A>
{
    /// Returns the next available physical frame, or `None`.
    ///
    fn allocate_phys_frame(&mut self, size: PhysFrameSize) -> Option<PhysFrame> {
        if let Some(frame) = self.allocator.allocate_phys_frame(size) {
            self.tracker.allocate_frame(frame);
            Some(frame)
        } else {
            None
        }
    }
}

impl<'allocator, 'tracker, A: PhysFrameAllocator + PhysFrameDeallocator> PhysFrameDeallocator
    for ArenaFrameAllocator<'allocator, 'tracker, A>
{
    /// Marks the given physical memory frame as unused and returns it to the
    /// list of free frames for later use.
    ///
    /// # Safety
    ///
    /// The caller must ensure that `frame` is unused.
    ///
    unsafe fn deallocate_phys_frame(&mut self, frame: PhysFrame) {
        self.tracker.deallocate_frame(frame);
        self.allocator.deallocate_phys_frame(frame);
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_bitmap_frame_allocator() {
        use bootinfo::FrameRange;
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
        assert_eq!(alloc.num_frames, 6);
        assert_eq!(alloc.free_frames, 6);

        // Helper function to speed up making frames.
        fn frame_for(addr: usize) -> PhysFrame {
            let start_addr = PhysAddr::new(addr);
            let size = PhysFrameSize::Size4KiB;
            let frame = PhysFrame::from_start_address(start_addr, size).unwrap();
            frame
        }

        let size = PhysFrameSize::Size4KiB;

        // Do some allocations.
        assert_eq!(alloc.allocate_phys_frame(size), Some(frame_for(0x4000)));
        assert_eq!(alloc.num_frames, 6);
        assert_eq!(alloc.free_frames, 5);
        assert_eq!(alloc.allocate_phys_frame(size), Some(frame_for(0x5000)));
        assert_eq!(alloc.num_frames, 6);
        assert_eq!(alloc.free_frames, 4);

        // Do a free.
        unsafe { alloc.deallocate_phys_frame(frame_for(0x4000)) };
        assert_eq!(alloc.num_frames, 6);
        assert_eq!(alloc.free_frames, 5);

        // Next allocation should return the address we just freed.
        assert_eq!(alloc.allocate_phys_frame(size), Some(frame_for(0x4000)));
        assert_eq!(alloc.num_frames, 6);
        assert_eq!(alloc.free_frames, 4);

        // Check that all remaining allocations are as we expect.
        assert_eq!(alloc.allocate_phys_frame(size), Some(frame_for(0x6000)));
        assert_eq!(alloc.allocate_phys_frame(size), Some(frame_for(0x7000)));
        assert_eq!(alloc.allocate_phys_frame(size), Some(frame_for(0xc000)));
        assert_eq!(alloc.allocate_phys_frame(size), Some(frame_for(0xd000)));
        assert_eq!(alloc.num_frames, 6);
        assert_eq!(alloc.free_frames, 0);

        // Check that we get nothing once we run out of frames.
        assert_eq!(alloc.allocate_phys_frame(size), None);
        assert_eq!(alloc.num_frames, 6);
        assert_eq!(alloc.free_frames, 0);

        // Check that sequential allocations work correctly.

        // Deallocate 2 non-sequential frames, expect None.
        unsafe { alloc.deallocate_phys_frame(frame_for(0x5000)) };
        unsafe { alloc.deallocate_phys_frame(frame_for(0x7000)) };
        assert_eq!(alloc.allocate_n_frames(2), None);

        // Leave 2 sequential frames, check we get the right pair.
        assert_eq!(alloc.allocate_phys_frame(size), Some(frame_for(0x5000)));
        unsafe { alloc.deallocate_phys_frame(frame_for(0x6000)) };
        assert_eq!(
            alloc.allocate_n_frames(2),
            Some(PhysFrame::range_exclusive(
                frame_for(0x6000),
                frame_for(0x8000)
            ))
        );

        // Check that we get nothing once we run out of frames.
        assert_eq!(alloc.num_frames, 6);
        assert_eq!(alloc.free_frames, 0);
    }

    #[test]
    fn test_arena_frame_allocator() {
        // Construct an arena, confirm that it's empty to
        // start with, then perform some allocations and
        // make sure they match up with the allcoator.
        use bootinfo::FrameRange;
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
        assert_eq!(alloc.num_frames, 6);
        assert_eq!(alloc.free_frames, 6);

        // Helper function to speed up making frames.
        fn frame_for(addr: usize) -> PhysFrame {
            let start_addr = PhysAddr::new(addr);
            let size = PhysFrameSize::Size4KiB;
            let frame = PhysFrame::from_start_address(start_addr, size).unwrap();
            frame
        }

        let size = PhysFrameSize::Size4KiB;

        // Do some allocations.
        assert_eq!(alloc.allocate_phys_frame(size), Some(frame_for(0x4000)));
        assert_eq!(alloc.num_frames, 6);
        assert_eq!(alloc.free_frames, 5);
        assert_eq!(alloc.allocate_phys_frame(size), Some(frame_for(0x5000)));
        assert_eq!(alloc.num_frames, 6);
        assert_eq!(alloc.free_frames, 4);

        // Create a tracker, which should start out
        // empty, even though we've already done some
        // allocations.
        let mut tracker = alloc.new_tracker();
        assert_eq!(tracker.num_frames, 6);
        assert_eq!(tracker.allocated_frames, 0);

        // Create an arena allocator using the tracker
        // we just created and the underlying allocator,
        // allowing us to track the allocations.
        let mut arena = ArenaFrameAllocator::new(&mut alloc, &mut tracker);

        // Perform some allocations and confirm they
        // are tracked correctly.
        assert_eq!(arena.allocate_phys_frame(size), Some(frame_for(0x6000)));
        assert_eq!(arena.tracker.num_frames, 6);
        assert_eq!(arena.tracker.allocated_frames, 1);
        assert!(arena.tracker.is_allocated(frame_for(0x6000)));

        // Do a free.
        unsafe { arena.deallocate_phys_frame(frame_for(0x6000)) };
        assert_eq!(arena.tracker.num_frames, 6);
        assert_eq!(arena.tracker.allocated_frames, 0);
        assert!(!arena.tracker.is_allocated(frame_for(0x6000)));

        // Next allocation should return the address we just freed.
        assert_eq!(arena.allocate_phys_frame(size), Some(frame_for(0x6000)));
        assert_eq!(arena.tracker.num_frames, 6);
        assert_eq!(arena.tracker.allocated_frames, 1);
        assert!(arena.tracker.is_allocated(frame_for(0x6000)));

        // Check that all remaining allocations are as we expect.
        assert_eq!(arena.allocate_phys_frame(size), Some(frame_for(0x7000)));
        assert_eq!(arena.allocate_phys_frame(size), Some(frame_for(0xc000)));
        assert_eq!(arena.allocate_phys_frame(size), Some(frame_for(0xd000)));
        assert_eq!(arena.tracker.num_frames, 6);
        assert_eq!(arena.tracker.allocated_frames, 4);

        // Check that we get nothing once we run out of frames.
        assert_eq!(arena.allocate_phys_frame(size), None);
        assert_eq!(arena.tracker.num_frames, 6);
        assert_eq!(arena.tracker.allocated_frames, 4);

        // Check that deallocating everything empties the tracker
        // and is reflected in the allocator.

        // Deallocate everything and check the tracker.
        unsafe { arena.deallocate_all_frames() };
        drop(arena);
        assert_eq!(tracker.num_frames, 6);
        assert_eq!(tracker.allocated_frames, 0);
        assert!(!tracker.is_allocated(frame_for(0x6000)));
        assert!(!tracker.is_allocated(frame_for(0x7000)));
        assert!(!tracker.is_allocated(frame_for(0xc000)));
        assert!(!tracker.is_allocated(frame_for(0xd000)));

        // Check that the allocator agrees.
        let size = PhysFrameSize::Size4KiB;
        assert_eq!(alloc.num_frames, 6);
        assert_eq!(alloc.free_frames, 4);
        assert_eq!(alloc.allocate_phys_frame(size), Some(frame_for(0x6000)));
        assert_eq!(alloc.allocate_phys_frame(size), Some(frame_for(0x7000)));
        assert_eq!(alloc.allocate_phys_frame(size), Some(frame_for(0xc000)));
        assert_eq!(alloc.allocate_phys_frame(size), Some(frame_for(0xd000)));
    }
}
