// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides a linked list allocator, which can be used to allocate heap memory.

use crate::Locked;
use align::align_up_usize;
use alloc::alloc::{GlobalAlloc, Layout};
use core::{mem, ptr};
use spin::lock;

/// Represents a region of memory with the given
/// `size` and an optional pointer to the `next`
/// free region.
///
struct ListNode {
    size: usize,
    next: Option<&'static mut ListNode>,
}

impl ListNode {
    const fn new(size: usize) -> Self {
        ListNode { size, next: None }
    }

    fn start_addr(&self) -> usize {
        self as *const Self as usize
    }

    fn end_addr(&self) -> usize {
        self.start_addr() + self.size
    }
}

/// A simple virtual memory allocator, tracking a list of
/// free regions.
///
/// `LinkedListAllocator` consists of a linked list of free
/// heap memory. When memory is allocated, we find the first
/// free region in the list that meets the allocation's size
/// and alignment requirements, remove it from the list, and
/// return it. If the free region includes more memory than
/// is needed, the remaining memory is added to the free list
/// as a new region.
///
/// When memory is de-allocated, the new region is prepended
/// to the start of the free list.
///
/// This implementation does not re-merge contiguous free
/// regions, so the heap will fragment over time.
///
pub struct LinkedListAllocator {
    head: ListNode,
}

impl LinkedListAllocator {
    /// Creates an empty `LinkedListAllocator`.
    ///
    pub const fn new() -> Self {
        Self {
            head: ListNode::new(0),
        }
    }

    /// Initialise the allocator with the given heap bounds.
    ///
    /// # Safety
    ///
    /// This function is unsafe because the caller must guarantee that the given
    /// heap bounds are valid and that the heap is unused. This method must be
    /// called only once.
    ///
    pub unsafe fn init(&mut self, heap_start: usize, heap_size: usize) {
        self.add_free_region(heap_start, heap_size);
    }

    /// Adds the given memory region to the front of the free list.
    ///
    unsafe fn add_free_region(&mut self, addr: usize, size: usize) {
        // Ensure that the freed region is capable of holding ListNode.
        assert_eq!(align_up_usize(addr, mem::align_of::<ListNode>()), addr);
        assert!(size >= mem::size_of::<ListNode>());

        // Create a new list node and prepend it to the start of the list
        let mut node = ListNode::new(size);
        node.next = self.head.next.take();
        let node_ptr = addr as *mut ListNode;
        node_ptr.write(node);
        self.head.next = Some(&mut *node_ptr)
    }

    /// Looks for a free region with the given size and alignment
    /// and removes it from the list.
    ///
    /// Returns a tuple of the list node and the start address of
    /// the allocation.
    ///
    fn find_region(&mut self, size: usize, align: usize) -> Option<(&'static mut ListNode, usize)> {
        // Reference to current list node, updated for each iteration.
        let mut current = &mut self.head;

        // Look for a large enough memory region in the linked list.
        while let Some(ref mut region) = current.next {
            if let Ok(alloc_start) = Self::alloc_from_region(region, size, align) {
                // The region is suitable for allocation,
                // so remove the node from the list.

                let next = region.next.take();
                let ret = Some((current.next.take().unwrap(), alloc_start));
                current.next = next;

                return ret;
            } else {
                // The region is not suitable, so continue
                // with the next region.
                current = current.next.as_mut().unwrap();
            }
        }

        // No suitable region found.
        None
    }

    /// Tries to use the given region for an allocation with the
    /// given size and alignment.
    ///
    /// Returns the allocation start address on success.
    ///
    fn alloc_from_region(region: &ListNode, size: usize, align: usize) -> Result<usize, ()> {
        let alloc_start = align_up_usize(region.start_addr(), align);
        let alloc_end = alloc_start.checked_add(size).ok_or(())?;

        if alloc_end > region.end_addr() {
            // The region is too small.
            return Err(());
        }

        let excess_size = region.end_addr() - alloc_end;
        if excess_size > 0 && excess_size < mem::size_of::<ListNode>() {
            // The rest of region is too small to hold
            // a ListNode (required because the allocation
            // splits the region into a used and a free
            // part).
            return Err(());
        }

        // The region is suitable for allocation.
        Ok(alloc_start)
    }
}

/// Adjust the given layout so that the resulting allocated memory
/// region is also capable of storing a ListNode.
///
/// Returns the adjusted size and alignment as a (size, align) tuple.
///
fn size_align(layout: Layout) -> (usize, usize) {
    let layout = layout
        .align_to(mem::align_of::<ListNode>())
        .expect("adjusting alignment failed")
        .pad_to_align();

    let size = layout.size().max(mem::size_of::<ListNode>());

    (size, layout.align())
}

unsafe impl GlobalAlloc for Locked<LinkedListAllocator> {
    /// Returns the next available address, or a null
    /// pointer if the heap has been exhausted.
    ///
    unsafe fn alloc(&self, layout: Layout) -> *mut u8 {
        // Perform layout adjustments.
        let (size, align) = size_align(layout);
        let mut allocator = lock!(self.lock);

        if let Some((region, alloc_start)) = allocator.find_region(size, align) {
            let alloc_end = alloc_start.checked_add(size).expect("overflow");
            let excess_size = region.end_addr() - alloc_end;
            if excess_size > 0 {
                allocator.add_free_region(alloc_end, excess_size);
            }

            alloc_start as *mut u8
        } else {
            ptr::null_mut()
        }
    }

    /// Marks the given pointer as unused and free for
    /// later re-use.
    ///
    unsafe fn dealloc(&self, ptr: *mut u8, layout: Layout) {
        // Perform layout adjustments.
        let (size, _) = size_align(layout);

        lock!(self.lock).add_free_region(ptr as usize, size)
    }
}
