// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Handles paging, memory management, and enabling the kernel's heap allocator.
//!
//! ## Physical memory management
//!
//! The [`physmem`] crate provides the ability to allocate physical
//! memory frames. This is then used as the foundation for the
//! virtual memory manager.
//!
//! ## Virtual memory management
//!
//! The [`virtmem`] crate provides functionality to analyse and modify
//! page tables, manage the virtual memory address space, and
//! operate the kernel's heap.
//!
//! ## Memory-mapped I/O
//!
//! The [`mmio`] crate provides a way to map physical memory into
//! the virtual address space with safe data accessors and write
//! back page flags.
//!
//! # Memory management
//!
//! This module governs various details of the management of virtual
//! and physical memory.
//!
//! ## Initialising the kernel heap
//!
//! Calling [`init`] initialises the physical and virtual memory
//! managers, then initialises the kernel heap:
//!
//! - The bootstrap physical memory manager is initialised using the memory map included in the boot info we are passed by the bootloader.
//! - The bootstrap allocator is used to initialise the virtual memory management (including for allocating page tables).
//! - The global heap allocator is initialised to set up the kernel's heap.
//! - The heap and the bootstrap physical memory manager are used to initialse the second stage physical memory manager.
//!
//! ## Kernel page tables
//!
//! Calling [`kernel_pml4`] returns a mutable reference to the kernel's
//! level 4 page table. This can be used to inspect or modify the virtual
//! page mappings.
//!
//! ## Virtual memory helpers
//!
//! This module contains various helper functions for physical and virtual
//! memory management.
//!
//! The [`phys_to_virt_addr`] function can be called to return a virtual
//! address that can be used to access the passed physical address. A set
//! of page tables (such as the kernel's level 4 page table returned by
//! [`kernel_pml4`]) can be used with [`virt_to_phys_addrs`](virtmem::virt_to_phys_addrs)
//! to determine the set of physical memory buffers referenced by the
//! given virtual memory buffer.
//!
//! ## Kernel stack management
//!
//! Each kernel thread (including the initial kernel thread, started by
//! the bootloader) has its own stack, which exist within the [`KERNEL_STACK`](memlayout::KERNEL_STACK)
//! memory region. The initial kernel thread is given its stack ([`KERNEL_STACK_0`](memlayout::KERNEL_STACK_0))
//! implicitly by the bootloader. Subsequent kernel threads are allocated
//! by calling [`new_kernel_stack`] and can be de-allocated by calling
//! [`free_kernel_stack`]. De-allocated stacks are reused and can be
//! returned by subsequent calls to [`new_kernel_stack`].

use alloc::vec::Vec;
use bootloader::BootInfo;
use core::sync::atomic::{AtomicU64, Ordering};
use memlayout::{
    phys_to_virt_addr, VirtAddrRange, KERNEL_STACK, KERNEL_STACK_1_START, PHYSICAL_MEMORY_OFFSET,
};
use physmem;
use spin::Mutex;
use virtmem;
use x86_64::registers::control::Cr3;
use x86_64::structures::paging::mapper::MapToError;
use x86_64::structures::paging::page::PageRangeInclusive;
use x86_64::structures::paging::{
    FrameAllocator, Mapper, OffsetPageTable, Page, PageSize, PageTable, PageTableFlags, Size4KiB,
};
use x86_64::VirtAddr;

// PML4 functionality.

/// KERNEL_PML4_ADDRESS contains the virtual address of the kernel's
/// level 4 page table. This enables the kernel_pml4 function to
/// construct the structured data.
///
static KERNEL_PML4_ADDRESS: Mutex<VirtAddr> = Mutex::new(VirtAddr::zero());

/// Initialise the kernel's memory, including setting up the
/// heap.
///
/// - The bootstrap physical memory manager is initialised using the memory map included in the boot info we are passed by the bootloader.
/// - The bootstrap allocator is used to initialise the virtual memory management (including for allocating page tables).
/// - The global heap allocator is initialised to set up the kernel's heap.
/// - The heap and the bootstrap physical memory manager are used to initialse the second stage physical memory manager.
///
/// # Safety
///
/// This function is unsafe because the caller must guarantee
/// that the complete physical memory is mapped to virtual memory
/// at [`PHYSICAL_MEMORY_OFFSET`].
///
/// `init` must be called only once to avoid aliasing &mut
/// references (which is undefined behavior).
///
pub unsafe fn init(boot_info: &'static BootInfo) {
    // Prepare the kernel's PML4.
    let (level_4_table_frame, _) = Cr3::read();
    let phys = level_4_table_frame.start_address();
    *KERNEL_PML4_ADDRESS.lock() = phys_to_virt_addr(phys);

    let mut page_table = kernel_pml4();
    let mut frame_allocator = physmem::bootstrap(&boot_info.memory_map);

    virtmem::init(&mut page_table, &mut frame_allocator).expect("heap initialization failed");

    // Switch over to a more sophisticated physical memory manager.
    physmem::init(frame_allocator);
}

/// Returns a mutable reference to the kernel's level 4 page
/// table.
///
/// # Safety
///
/// The returned page tables must only be used to translate
/// addresses when it is the active level 4 page table.
///
pub unsafe fn kernel_pml4() -> OffsetPageTable<'static> {
    let virt = KERNEL_PML4_ADDRESS.lock();
    let page_table_ptr: *mut PageTable = virt.as_mut_ptr();

    let page_table = &mut *page_table_ptr; // unsafe
    OffsetPageTable::new(page_table, PHYSICAL_MEMORY_OFFSET)
}

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

    let mut mapper = unsafe { kernel_pml4() };
    let mut frame_allocator = physmem::ALLOCATOR.lock();
    for page in Page::range(stack_start, stack_end) {
        let frame = frame_allocator
            .allocate_frame()
            .ok_or(MapToError::FrameAllocationFailed)?;

        let flags = PageTableFlags::PRESENT | PageTableFlags::WRITABLE | PageTableFlags::NO_EXECUTE;
        unsafe {
            mapper
                .map_to(page, frame, flags, &mut *frame_allocator)?
                .flush()
        };
    }

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
