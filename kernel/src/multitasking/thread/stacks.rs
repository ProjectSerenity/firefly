// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Includes helpers and state for managing the additional stacks
//! used for kernel threads.
//!
//! Each kernel thread (including the initial kernel thread, started by
//! the bootloader) has its own stack, which exist within the [`KERNEL_STACK`](memlayout::KERNEL_STACK)
//! memory region. The initial kernel thread is given its stack ([`KERNEL_STACK_0`](memlayout::KERNEL_STACK_0))
//! implicitly by the bootloader. Subsequent kernel threads are allocated
//! by calling [`new_kernel_stack`] and can be de-allocated by calling
//! [`free_kernel_stack`]. De-allocated stacks are reused and can be
//! returned by subsequent calls to [`new_kernel_stack`].

use alloc::vec::Vec;
use core::sync::atomic::{AtomicU64, Ordering};
use memlayout::{VirtAddrRange, KERNEL_STACK, KERNEL_STACK_1_START};
use spin::Mutex;
use virtmem::map_pages;
use x86_64::structures::paging::mapper::MapToError;
use x86_64::structures::paging::page::PageRangeInclusive;
use x86_64::structures::paging::{Page, PageSize, PageTableFlags, Size4KiB};
use x86_64::VirtAddr;

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

    /// Returns a set of stack bounds consisting of the given
    /// virtual page range.
    ///
    pub fn from_page_range(range: PageRangeInclusive) -> Self {
        StackBounds {
            start: range.start.start_address(),
            end: range.end.start_address() + (Size4KiB::SIZE - 1u64),
        }
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
    pub fn num_pages(&self) -> u64 {
        ((self.end - self.start) + (Size4KiB::SIZE - 1)) / Size4KiB::SIZE as u64
    }

    /// Returns whether the stack bounds include the given
    /// virtual address.
    ///
    pub fn contains(&self, addr: VirtAddr) -> bool {
        self.start <= addr && addr <= self.end
    }
}

/// Reserves `num_pages` pages of stack memory for a kernel
/// thread.
///
/// `reserve_kernel_stack` returns the page at the start of
/// the stack (the lowest address).
///
fn reserve_kernel_stack(num_pages: u64) -> Page {
    static STACK_ALLOC_NEXT: AtomicU64 = AtomicU64::new(KERNEL_STACK_1_START.as_u64());
    let start_addr = VirtAddr::new(
        STACK_ALLOC_NEXT.fetch_add(num_pages * Page::<Size4KiB>::SIZE, Ordering::Relaxed),
    );

    let last_addr = start_addr + (num_pages * Page::<Size4KiB>::SIZE) - 1u64;
    if !KERNEL_STACK.contains_range(start_addr, last_addr) {
        panic!("cannot reserve kernel stack: kernel stack space exhausted");
    }

    Page::from_start_address(start_addr).expect("`STACK_ALLOC_NEXT` not page aligned")
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
pub fn new_kernel_stack(num_pages: u64) -> Result<StackBounds, MapToError<Size4KiB>> {
    // Check whether we can just recycle an old stack.
    // We use an extra scope so we don't hold the lock
    // on DEAD_STACKS for unnecessarily long.
    {
        let mut stacks = DEAD_STACKS.lock();
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

    let pages = Page::range(stack_start, stack_end);
    let flags = PageTableFlags::PRESENT | PageTableFlags::WRITABLE | PageTableFlags::NO_EXECUTE;

    map_pages(pages, &mut *physmem::ALLOCATOR.lock(), flags)?;

    Ok(StackBounds {
        start: stack_start.start_address(),
        end: stack_end.start_address() - 1u64,
    })
}

/// Adds the given stack to the dead stacks list, so it
/// can be reused later.
///
pub fn free_kernel_stack(stack_bounds: StackBounds) {
    DEAD_STACKS.lock().push(stack_bounds);
}
