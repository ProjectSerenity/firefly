// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides a fixed-size block allocator, which can be used to allocate heap memory.

use super::Locked;
use alloc::alloc::{GlobalAlloc, Layout};
use core::ptr::NonNull;
use core::{mem, ptr};
use linked_list_allocator::Heap;
use spin::lock;

/// BLOCK_SIZES contains the block sizes used.
///
/// The sizes must each be power of 2 because they are also used as
/// the block alignment (alignments must be always powers of 2).
///
const BLOCK_SIZES: &[usize] = &[8, 16, 32, 64, 128, 256, 512, 1024, 2048];

/// list_index chooses an appropriate block size for the given layout.
///
/// Returns an index into the `BLOCK_SIZES` array.
///
fn list_index(layout: &Layout) -> Option<usize> {
    let required_block_size = layout.size().max(layout.align());
    BLOCK_SIZES.iter().position(|&s| s >= required_block_size)
}

/// An entry in the linked list of free blocks.
///
struct ListNode {
    next: Option<&'static mut ListNode>,
}

/// A simple virtual memory allocator, tracking lists of fixed
/// size free memory regions.
///
/// `FixedSizeBlockAllocator` tracks a series of linked lists of
/// free memory regions, each of a different fixed size. For
/// each allocation, we find the smallest block size equal to or
/// larger than the requested size. We then return the next block
/// from the corresponding free list.
///
/// If we cannot satisfy an allocation request from the fixed block
/// free list, we fall back to an underlying allocator. This is
/// particularly likely for large allocations that exceed the largest
/// block size.
///
/// When memory is de-allocated, we find the block size again and
/// add the block to the corresponding free list.
///
/// This is similar in behaviour to the [`LinkedListAllocator`](crate::LinkedListAllocator),
/// but has more predictable performance when performing allocations
/// and de-allocations. However, most blocks will include wasted
/// memory, resulting in worse space efficiency.
///
pub struct FixedSizeBlockAllocator {
    list_heads: [Option<&'static mut ListNode>; BLOCK_SIZES.len()],
    fallback_allocator: Heap,
}

impl FixedSizeBlockAllocator {
    /// Creates an empty `FixedSizeBlockAllocator`.
    ///
    pub const fn new() -> Self {
        const EMPTY: Option<&'static mut ListNode> = None;
        FixedSizeBlockAllocator {
            list_heads: [EMPTY; BLOCK_SIZES.len()],
            fallback_allocator: Heap::empty(),
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
        self.fallback_allocator.init(heap_start, heap_size);
    }

    /// Allocates using the fallback allocator.
    ///
    fn fallback_alloc(&mut self, layout: Layout) -> *mut u8 {
        match self.fallback_allocator.allocate_first_fit(layout) {
            Ok(ptr) => ptr.as_ptr(),
            Err(_) => ptr::null_mut(),
        }
    }
}

unsafe impl GlobalAlloc for Locked<FixedSizeBlockAllocator> {
    /// Returns the next available address, or a null
    /// pointer if the heap has been exhausted.
    ///
    unsafe fn alloc(&self, layout: Layout) -> *mut u8 {
        let mut allocator = lock!(self.lock);

        // Find the block size index for this allocation.
        match list_index(&layout) {
            // This request exceeds even the largest fixed
            // size block we support, so we resort to the
            // fallback allocator.
            None => allocator.fallback_alloc(layout),

            // We now have an `index` into the list heads.
            // Now we see whether we have any free blocks
            // of that size.
            Some(index) => {
                match allocator.list_heads[index].take() {
                    // We found a block, so we pop it off
                    // the free list, updating the free
                    // list head, then return the block we
                    // popped.
                    Some(node) => {
                        allocator.list_heads[index] = node.next.take();
                        node as *mut ListNode as *mut u8
                    }

                    // No blocks left, so we allocate one
                    // and return it. When it's de-allocated,
                    // we will add it to the corresponding
                    // free list.
                    None => {
                        let block_size = BLOCK_SIZES[index];
                        // Using the block's size as its alignment
                        // only works if all block sizes are an
                        // exact power of 2.
                        let block_align = block_size;
                        let layout = Layout::from_size_align(block_size, block_align).unwrap();
                        allocator.fallback_alloc(layout)
                    }
                }
            }
        }
    }

    /// Marks the given pointer as unused and free for
    /// later re-use.
    ///
    unsafe fn dealloc(&self, ptr: *mut u8, layout: Layout) {
        let mut allocator = lock!(self.lock);

        // Find the block size index for this allocation.
        match list_index(&layout) {
            // This request exceeds even the largest fixed
            // size block we support, so we return it to
            // the fallback allocator.
            None => {
                let ptr = NonNull::new(ptr).unwrap();
                allocator.fallback_allocator.deallocate(ptr, layout);
            }

            // This fits in a fixed block, so we return the
            // block to the corresponding free list.
            Some(index) => {
                let new_node = ListNode {
                    next: allocator.list_heads[index].take(),
                };

                // Check that the block has the size and alignment
                // required to store a list node.
                assert!(mem::size_of::<ListNode>() <= BLOCK_SIZES[index]);
                assert!(mem::align_of::<ListNode>() <= BLOCK_SIZES[index]);

                // Prepend the block to the free list.
                let new_node_ptr = ptr as *mut ListNode;
                new_node_ptr.write(new_node);
                allocator.list_heads[index] = Some(&mut *new_node_ptr);
            }
        }
    }
}
