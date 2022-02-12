// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides a bump allocator, which can be used to allocate heap memory.

use crate::Locked;
use align::align_up_usize;
use alloc::alloc::{GlobalAlloc, Layout};
use core::ptr;

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
    heap_start: usize,
    heap_end: usize,
    next: usize,
    allocations: usize,
}

impl BumpAllocator {
    /// Creates a new empty bump allocator.
    ///
    pub const fn new() -> Self {
        BumpAllocator {
            heap_start: 0,
            heap_end: 0,
            next: 0,
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
    pub unsafe fn init(&mut self, heap_start: usize, heap_size: usize) {
        self.heap_start = heap_start;
        self.heap_end = heap_start.saturating_add(heap_size);
        self.next = heap_start;
    }
}

unsafe impl GlobalAlloc for Locked<BumpAllocator> {
    /// Returns the next available address, or a null
    /// pointer if the heap has been exhausted.
    ///
    unsafe fn alloc(&self, layout: Layout) -> *mut u8 {
        let mut bump = self.lock(); // get a mutable reference

        let alloc_start = align_up_usize(bump.next, layout.align());
        let alloc_end = match alloc_start.checked_add(layout.size()) {
            Some(end) => end,
            None => return ptr::null_mut(),
        };

        if alloc_end > bump.heap_end {
            ptr::null_mut() // out of memory
        } else {
            bump.next = alloc_end;
            bump.allocations += 1;
            alloc_start as *mut u8
        }
    }

    /// Marks the given pointer as unused and free for
    /// later re-use.
    ///
    unsafe fn dealloc(&self, _ptr: *mut u8, _layout: Layout) {
        let mut bump = self.lock(); // get a mutable reference

        bump.allocations -= 1;
        if bump.allocations == 0 {
            bump.next = bump.heap_start;
        }
    }
}
