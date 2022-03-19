// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides a bump allocator, which can be used to allocate heap memory.

use crate::Locked;
use alloc::alloc::{GlobalAlloc, Layout};
use core::ptr;
use spin::lock;
use x86_64::VirtAddr;

/// A simple virtual memory allocator, tracking the next free
/// address.
///
/// `BumpAllocator` simply tracks the number of allocations
/// currently active and the next available address. When an
/// allocation is made, the `next` address is incremented by
/// the allocation size and the number of `allocations is
/// incremented by one.
///
/// When memory is de-allocated, `allocations` is decremented.
/// If this result in `allocations` becoming `0`, then we know
/// all memory is free, so the `next` address is reset to the
/// start of the heap memory region.
///
/// Since it is not possible to reuse memory without the entire
/// memory space being de-allocated, `BumpAllocator` is likely
/// to run out of available memory. However, the implementation
/// is very simple.
///
pub struct BumpAllocator {
    heap_start: VirtAddr,
    heap_end: VirtAddr,
    next: VirtAddr,
    allocations: usize,
}

impl BumpAllocator {
    /// Creates a new empty bump allocator.
    ///
    pub const fn new() -> Self {
        BumpAllocator {
            heap_start: VirtAddr::zero(),
            heap_end: VirtAddr::zero(),
            next: VirtAddr::zero(),
            allocations: 0,
        }
    }

    /// Initializes the bump allocator with the given heap bounds.
    ///
    /// # Safety
    ///
    /// This method is unsafe because the caller must ensure that the given
    /// memory range is unused. Also, this method must be called only once.
    ///
    pub unsafe fn init(&mut self, heap_start: VirtAddr, heap_size: usize) {
        self.heap_start = heap_start;
        self.heap_end = heap_start + heap_size;
        self.next = heap_start;
    }

    /// Returns the next available memory region with the given
    /// alignment and size requirements.
    ///
    fn allocate(&mut self, align: usize, size: usize) -> Option<VirtAddr> {
        let alloc_start = self.next.align_up(align as u64);
        let alloc_end = match alloc_start.as_u64().checked_add(size as u64) {
            Some(end) => match VirtAddr::try_new(end) {
                Ok(end) => end,
                Err(_) => return None,
            },
            None => return None,
        };

        if alloc_end > self.heap_end {
            None
        } else {
            self.next = alloc_end;
            self.allocations += 1;
            Some(alloc_start)
        }
    }

    /// Deallocates the given region, marking it as unused and
    /// free for later use.
    ///
    fn deallocate(&mut self, addr: VirtAddr, size: usize) -> Result<(), &'static str> {
        if addr < self.heap_start || self.heap_end <= addr {
            return Err("deallocated region was not allocated by this heap");
        }

        let end = match addr.as_u64().checked_add(size as u64) {
            Some(end) => match VirtAddr::try_new(end) {
                Ok(end) => end,
                Err(_) => return Err("deallocated region is invalid"),
            },
            None => return Err("deallocated region is invalid"),
        };

        if self.heap_end < end {
            Err("deallocated region was not allocated by this heap")
        } else {
            self.allocations -= 1;
            if self.allocations == 0 {
                self.next = self.heap_start;
            }

            Ok(())
        }
    }
}

unsafe impl GlobalAlloc for Locked<BumpAllocator> {
    /// Returns the next available address, or a null
    /// pointer if the heap has been exhausted.
    ///
    unsafe fn alloc(&self, layout: Layout) -> *mut u8 {
        let mut bump = lock!(self.lock); // get a mutable reference

        match bump.allocate(layout.align(), layout.size()) {
            Some(addr) => addr.as_mut_ptr(),
            None => ptr::null_mut(),
        }
    }

    /// Marks the given pointer as unused and free for
    /// later re-use.
    ///
    unsafe fn dealloc(&self, ptr: *mut u8, layout: Layout) {
        let mut bump = lock!(self.lock); // get a mutable reference

        match bump.deallocate(VirtAddr::from_ptr(ptr), layout.size()) {
            Ok(_) => {}
            Err(err) => panic!("{}", err),
        }
    }
}

#[cfg(test)]
mod test {
    use super::*;

    #[test]
    fn test_bump_allocator() {
        let mut heap = BumpAllocator::new();
        unsafe { heap.init(VirtAddr::new(0x1000), 0x1000) };

        // Check that we can make a few successive aligned allocations.
        assert_eq!(heap.allocate(8, 8), Some(VirtAddr::new(0x1000)));
        assert_eq!(heap.next, VirtAddr::new(0x1008));
        assert_eq!(heap.allocate(8, 8), Some(VirtAddr::new(0x1008)));
        assert_eq!(heap.next, VirtAddr::new(0x1010));
        assert_eq!(heap.allocate(8, 8), Some(VirtAddr::new(0x1010)));
        assert_eq!(heap.next, VirtAddr::new(0x1018));

        // Check that we handle alignment correctly.
        assert_eq!(heap.allocate(16, 8), Some(VirtAddr::new(0x1020)));
        assert_eq!(heap.next, VirtAddr::new(0x1028));
        assert_eq!(heap.allocate(16, 8), Some(VirtAddr::new(0x1030)));
        assert_eq!(heap.next, VirtAddr::new(0x1038));

        // Check that deallocating some of the memory doesn't wipe
        // out everything (as that's a defining feature of bump
        // allocators).
        assert_eq!(heap.deallocate(VirtAddr::new(0x1020), 8), Ok(()));
        assert_eq!(heap.next, VirtAddr::new(0x1038));
        assert_eq!(heap.allocations, 4);

        // Check that deallocating all of the memory resets our state.
        assert_eq!(heap.deallocate(VirtAddr::new(0x1000), 8), Ok(()));
        assert_eq!(heap.next, VirtAddr::new(0x1038));
        assert_eq!(heap.allocations, 3);
        assert_eq!(heap.deallocate(VirtAddr::new(0x1008), 8), Ok(()));
        assert_eq!(heap.next, VirtAddr::new(0x1038));
        assert_eq!(heap.allocations, 2);
        assert_eq!(heap.deallocate(VirtAddr::new(0x1010), 8), Ok(()));
        assert_eq!(heap.next, VirtAddr::new(0x1038));
        assert_eq!(heap.allocations, 1);
        assert_eq!(heap.deallocate(VirtAddr::new(0x1030), 8), Ok(()));
        assert_eq!(heap.next, VirtAddr::new(0x1000));
        assert_eq!(heap.allocations, 0);

        // Check that we reject deallocating memory outside our heap.
        assert_eq!(
            heap.deallocate(VirtAddr::new(0x8000), 8),
            Err("deallocated region was not allocated by this heap")
        );

        // Check that we reject deallocating invalid memory regions.
        // End address overflows.
        assert_eq!(
            heap.deallocate(VirtAddr::new(0x1000), 0xffff_ffff_ffff_ffff),
            Err("deallocated region is invalid")
        );
        // End address is non-canonical.
        assert_eq!(
            heap.deallocate(VirtAddr::new(0x1000), 0x1234_8000_0000_0000),
            Err("deallocated region is invalid")
        );
    }
}
