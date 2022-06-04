// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Includes helpers and state for managing the additional stacks
//! used for kernel threads.
//!
//! Each kernel thread (including the initial kernel thread, started by
//! the bootloader) has its own stack, which exist within the [`KERNEL_STACK`](memory::constants::KERNEL_STACK)
//! memory region. The initial kernel thread is given its stack ([`KERNEL_STACK_0`](memory::constants::KERNEL_STACK_0))
//! implicitly by the bootloader. Subsequent kernel threads are allocated
//! by calling [`new_kernel_stack`] and can be de-allocated by calling
//! [`free_kernel_stack`]. De-allocated stacks are reused and can be
//! returned by subsequent calls to [`new_kernel_stack`].

use alloc::vec::Vec;
use core::sync::atomic::{AtomicUsize, Ordering};
use memory::constants::{KERNEL_STACK, KERNEL_STACK_1_START};
use memory::{
    PageMappingError, PageTableFlags, PhysAddr, PhysFrame, PhysFrameSize, VirtAddr, VirtAddrRange,
    VirtPage, VirtPageSize,
};
use spin::{lock, Mutex};
use virtmem::map_pages;

/// Describes the address space used for a kernel stack region.
///
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct StackBounds {
    start: VirtAddr,
    end: VirtAddr,
}

impl StackBounds {
    /// Returns a set of stack bounds consisting of the given
    /// virtual address range.
    ///
    pub fn from(range: &VirtAddrRange) -> Self {
        StackBounds {
            start: range.start(),
            end: range.end(),
        }
    }

    /// Returns a set of stack bounds from the given start
    /// and stop addresses. The top of the stack is `end`
    /// and the bottom is `start`.
    ///
    pub fn from_addrs(start: VirtAddr, end: VirtAddr) -> Self {
        StackBounds { start, end }
    }

    /// Returns the smallest valid address in the stack bounds.
    /// As the stack grows downwards, this is also known as the
    /// bottom of the stack.
    ///
    pub fn start(&self) -> VirtAddr {
        self.start
    }

    /// Returns the largest valid address in the stack bounds.
    /// As the stack grows downwards, this is also known as the
    /// top of the stack.
    ///
    pub fn end(&self) -> VirtAddr {
        self.end
    }

    /// Returns the number of pages included in the bounds.
    ///
    pub fn num_pages(&self) -> usize {
        ((self.end - self.start) + (VirtPageSize::Size4KiB.bytes() - 1))
            / VirtPageSize::Size4KiB.bytes()
    }

    /// Returns whether the stack bounds include the given
    /// virtual address.
    ///
    pub fn contains(&self, addr: VirtAddr) -> bool {
        self.start <= addr && addr <= self.end
    }

    /// Populates the stack with its initial contents,
    /// starting at the top of the stack (the highest
    /// address).
    ///
    /// The resulting stack pointer is returned.
    ///
    /// # Safety
    ///
    /// `populate_kernel` is unsafe, as it writes to
    /// the memory in the stack bounds. This memory
    /// must be mapped in, and must not be being used
    /// for any other purposes.
    ///
    pub unsafe fn populate_kernel(&self, entries: &[u64]) -> usize {
        // Round down the address at the top of the
        // stack to be an exact multiple of 8.
        //
        // This means the next write to rsp will
        // insert the value into the top of the
        // stack.
        let top = self.end.as_usize() & !7;
        let mut rsp = top as *mut u64;
        for entry in entries {
            // Write the entry into the stack,
            // then decrement the stack pointer.
            rsp.write(*entry);
            rsp = rsp.sub(1);
        }

        // Increment the stack pointer so that it
        // points at the last value we inserted.

        rsp.add(1) as usize
    }

    /// Populates the stack with its initial contents,
    /// starting at the top of the stack (the highest
    /// address).
    ///
    /// The resulting stack pointer is returned.
    ///
    /// # Safety
    ///
    /// `populate_user` is unsafe, as it writes to
    /// the underlying physical memory passed to it.
    /// This memory need not be mapped in, but it must
    /// not be being used for any other purposes.
    ///
    pub unsafe fn populate_user(
        &self,
        frames: &[PhysFrame],
        phys_to_virt_addr: fn(PhysAddr) -> VirtAddr,
        entries: &[u64],
    ) -> usize {
        let frame_start = |frame: PhysFrame| {
            let phys = frame.start_address();
            phys_to_virt_addr(phys)
        };

        // Get a virtual address for the top of the
        // stack's physical memory.
        //
        // First, we need to work out the offset
        // of the top of the stack into the final
        // frame.
        let top_page = VirtPage::containing_address(self.end, VirtPageSize::Size4KiB);
        let offset = self.end - top_page.start_address();
        let mut end_idx = frames.len() - 1;
        let mut last_virt = frame_start(frames[end_idx]);
        let mut top_virt = last_virt + offset;

        // Round down the address at the top of the
        // stack to be an exact multiple of 8.
        let mut rsp = (top_virt.as_usize() & !7) as *mut u64;
        for entry in entries {
            // Write the entry into the stack,
            // then decrement the stack pointer.
            rsp.write(*entry);

            // If we've reached the bottom of
            // the current frame, we move to
            // the end of the next one.
            if rsp as usize == last_virt.as_usize() {
                end_idx -= 1;
                last_virt = frame_start(frames[end_idx]);
                top_virt = last_virt + PhysFrameSize::Size4KiB.bytes();
                rsp = top_virt.as_usize() as *mut u64;
            }

            rsp = rsp.sub(1);
        }

        // Now we have to derive the original
        // virtual address that would correspond
        // to the physical address rsp now points
        // to.

        (self.end.as_usize() & !7) - (entries.len() - 1) * 8
    }
}

/// Reserves `num_pages` pages of stack memory for a kernel
/// thread.
///
/// `reserve_kernel_stack` returns the page at the start of
/// the stack (the lowest address).
///
fn reserve_kernel_stack(num_pages: usize) -> VirtPage {
    static STACK_ALLOC_NEXT: AtomicUsize = AtomicUsize::new(KERNEL_STACK_1_START.as_usize());
    let start_addr = VirtAddr::new(STACK_ALLOC_NEXT.fetch_add(
        num_pages * VirtPageSize::Size4KiB.bytes(),
        Ordering::Relaxed,
    ));

    let last_addr = start_addr + (num_pages * VirtPageSize::Size4KiB.bytes()) - 1;
    if !KERNEL_STACK.contains_range(start_addr, last_addr) {
        panic!("cannot reserve kernel stack: kernel stack space exhausted");
    }

    VirtPage::from_start_address(start_addr, VirtPageSize::Size4KiB)
        .expect("`STACK_ALLOC_NEXT` not page aligned")
}

/// DEAD_STACKS is a free list of kernel stacks that
/// have been released by kernel threads that have
/// exited.
///
/// If there is a stack available in DEAD_STACKS
/// when a new thread is created, it is used instead
/// of allocating a new stack. This mitigates the
/// inability to track unused stacks in new_kernel_stack,
/// which would otherwise limit the number of
/// kernel threads that can be created during the
/// lifetime of the kernel. Instead, we're left
/// with just a limit on the number of simultaneous
/// kernel threads.
///
static DEAD_STACKS: Mutex<Vec<StackBounds>> = Mutex::new(Vec::new());

/// Allocates `num_pages` pages of stack memory for a
/// kernel thread and guard page.
///
/// `new_kernel_stack` returns the address space of the
/// allocated stack.
///
pub fn new_kernel_stack(num_pages: usize) -> Result<StackBounds, PageMappingError> {
    // Check whether we can just recycle an old stack.
    // We use an extra scope so we don't hold the lock
    // on DEAD_STACKS for unnecessarily long.
    {
        let mut stacks = lock!(DEAD_STACKS);
        let stack = stacks.pop();
        if let Some(stack) = stack {
            if stack.num_pages() >= num_pages {
                return Ok(stack);
            } else {
                stacks.push(stack);
            }
        }
    }

    let guard_page = reserve_kernel_stack(num_pages + 1);
    let stack_start = guard_page + 1;
    let stack_end = stack_start + num_pages;

    let pages = VirtPage::range_exclusive(stack_start, stack_end);
    let flags = PageTableFlags::PRESENT
        | PageTableFlags::GLOBAL
        | PageTableFlags::WRITABLE
        | PageTableFlags::NO_EXECUTE;

    map_pages(pages, &mut *lock!(physmem::ALLOCATOR), flags)?;

    Ok(StackBounds {
        start: stack_start.start_address(),
        end: stack_end.start_address() - 1,
    })
}

/// Adds the given stack to the dead stacks list, so it
/// can be reused later.
///
pub fn free_kernel_stack(stack_bounds: StackBounds) {
    lock!(DEAD_STACKS).push(stack_bounds);
}

#[cfg(test)]
mod tests {
    use super::*;
    use core::slice;

    #[test]
    fn populate_kernel_stack() {
        let mut stack = [0u64; 4];
        let bounds = StackBounds {
            start: VirtAddr::new((&mut stack[0]) as *mut u64 as usize),
            end: VirtAddr::new((&mut stack[0]) as *mut u64 as usize + 8 * 4 - 1), // Last addr of the buffer.
        };

        let entries = [0x1122_3344_5566_7788, 0x99aa_bbcc_ddee_ff00];

        let want_rsp = (&stack[2]) as *const u64 as usize; // Addr of the last item pushed.
        let want_stack = [0, 0, 0x99aa_bbcc_ddee_ff00, 0x1122_3344_5566_7788]; // Note the order.

        assert_eq!(want_rsp, unsafe { bounds.populate_kernel(&entries) });
        assert_eq!(want_stack, stack);
    }

    #[test]
    fn populate_user_stack() {
        // We start by allocating at least three
        // pages of memory. We do this so that
        // we can make a region of memory within
        // it that covers exactly two pages.
        //
        // We then use that region of two pages
        // to simulate various edge cases.
        const PAGE_SIZE: usize = VirtPageSize::Size4KiB.bytes();
        const TWO_PAGES: usize = 2 * PAGE_SIZE;
        let mut region = [0u8; 3 * PAGE_SIZE];
        let offset = (&region[0]) as *const u8 as usize % PAGE_SIZE;
        let start = PAGE_SIZE - offset;
        let region = &mut region[start..start + TWO_PAGES];
        let frames = [
            PhysFrame::from_start_address(
                PhysAddr::new((&region[0]) as *const u8 as usize),
                PhysFrameSize::Size4KiB,
            )
            .unwrap(),
            PhysFrame::from_start_address(
                PhysAddr::new((&region[PAGE_SIZE]) as *const u8 as usize),
                PhysFrameSize::Size4KiB,
            )
            .unwrap(),
        ];

        // We fake away physical memory by pretending
        // it's identity mapped.
        let phys_to_virt_addr = |phys: PhysAddr| VirtAddr::new(phys.as_usize());

        // Start with the simple case, where all
        // entries fit in the final frame.

        let stack = unsafe {
            slice::from_raw_parts_mut((&mut region[0]) as *mut u8 as *mut u64, TWO_PAGES / 8)
        };
        let bounds = StackBounds {
            start: VirtAddr::new((&mut stack[0]) as *mut u64 as usize),
            end: VirtAddr::new((&mut stack[0]) as *mut u64 as usize + stack.len() * 8 - 1), // Last addr of the buffer.
        };

        let entries = [0x1122_3344_5566_7788, 0x99aa_bbcc_ddee_ff00];

        let want_rsp = (&stack[stack.len() - 2]) as *const u64 as usize; // Addr of the last item pushed.
        let mut want_stack = [0u64; TWO_PAGES / 8];

        // Note that the order is reversed.
        let n = want_stack.len();
        want_stack[n - 2] = 0x99aa_bbcc_ddee_ff00;
        want_stack[n - 1] = 0x1122_3344_5566_7788;

        assert_eq!(want_rsp, unsafe {
            bounds.populate_user(&frames, phys_to_virt_addr, &entries)
        });
        assert_eq!(want_stack, stack);

        // Next, a case where entries are spread
        // over a frame boundary.

        let stack = unsafe {
            slice::from_raw_parts_mut((&mut region[0]) as *mut u8 as *mut u64, (PAGE_SIZE / 8) + 2)
        };
        stack.fill(0); // Reset the stack.
        let bounds = StackBounds {
            start: VirtAddr::new((&mut stack[0]) as *mut u64 as usize),
            end: VirtAddr::new((&mut stack[0]) as *mut u64 as usize + PAGE_SIZE + 15), // Space for 2 entries.
        };

        let entries = [
            0x1122_3344_5566_7788,
            0x99aa_bbcc_ddee_ff00,
            0x3131_4242_7575_8686,
            0xffff_ffff_ffff_ffff,
        ];

        let want_rsp = (&stack[stack.len() - 4]) as *const u64 as usize; // Addr of the last item pushed.
        let mut want_stack = [0u64; (PAGE_SIZE / 8) + 2];

        // Note that the order is reversed.
        let n = want_stack.len();
        want_stack[n - 4] = 0xffff_ffff_ffff_ffff;
        want_stack[n - 3] = 0x3131_4242_7575_8686;
        want_stack[n - 2] = 0x99aa_bbcc_ddee_ff00;
        want_stack[n - 1] = 0x1122_3344_5566_7788;

        assert_eq!(want_rsp, unsafe {
            bounds.populate_user(&frames, phys_to_virt_addr, &entries)
        });
        assert_eq!(want_stack, stack);
    }
}
