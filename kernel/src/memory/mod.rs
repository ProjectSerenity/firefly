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

use bootloader::BootInfo;
use memlayout::{phys_to_virt_addr, PHYSICAL_MEMORY_OFFSET};
use physmem;
use spin::Mutex;
use virtmem;
use x86_64::registers::control::Cr3;
use x86_64::structures::paging::{OffsetPageTable, PageTable};
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
