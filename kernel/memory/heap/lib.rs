// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Manages the kernel's heap, along with the underlying allocator.
//!
//! ## Heap initialisation
//!
//! The [`init`] function starts by mapping the entirety of the kernel heap
//! address space ([`KERNEL_HEAP`](memory::constants::KERNEL_HEAP)) using the physical
//! frame allocator provided. This virtual memory is then used to initialise
//! the heap allocator.
//!
//! With the heap initialised, `init` enables global page mappings and the
//! no-execute permission bit and then remaps virtual memory. This ensures
//! that unexpected page mappings are removed and the remaining page mappings
//! have the correct flags. For example, the kernel stack is mapped with the
//! no-execute permission bit set.

#![no_std]
#![deny(clippy::float_arithmetic)]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![deny(clippy::panic)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(unused_crate_dependencies)]
#![allow(unsafe_code)]
#![feature(const_mut_refs)]

extern crate alloc;

mod fixed_size_block;

use memory::constants::KERNEL_HEAP;
use memory::{PageMappingError, PageTableFlags, PhysFrameAllocator, VirtPage, VirtPageSize};
use spin::Mutex;
use virtmem::map_pages;
use x86_64::registers::control::{Cr4, Cr4Flags};
use x86_64::registers::model_specific::{Efer, EferFlags};

#[cfg(not(test))]
#[global_allocator]
static ALLOCATOR: Locked<fixed_size_block::FixedSizeBlockAllocator> =
    Locked::new(fixed_size_block::FixedSizeBlockAllocator::new());

/// Initialise the static global allocator, enabling the kernel heap.
///
/// The given physical memory frame allocator is used to map the
/// entirety of the kernel heap address space ([`KERNEL_HEAP`]).
///
/// With the heap initialised, `init` enables global page mappings and the
/// no-execute permission bit and then remaps virtual memory. This ensures
/// that unexpected page mappings are removed and the remaining page mappings
/// have the correct flags. For example, the kernel stack is mapped with the
/// no-execute permission bit set.
///
pub fn init(frame_allocator: &mut impl PhysFrameAllocator) -> Result<(), PageMappingError> {
    let page_range = {
        let heap_end = KERNEL_HEAP.end();
        let heap_start_page =
            VirtPage::containing_address(KERNEL_HEAP.start(), VirtPageSize::Size4KiB);
        let heap_end_page = VirtPage::containing_address(heap_end, VirtPageSize::Size4KiB);
        VirtPage::range_inclusive(heap_start_page, heap_end_page)
    };

    let flags = PageTableFlags::PRESENT | PageTableFlags::WRITABLE | PageTableFlags::NO_EXECUTE;

    map_pages(page_range, frame_allocator, flags).expect("failed to map kernel heap");

    #[cfg(not(test))]
    unsafe {
        use spin::lock;
        lock!(ALLOCATOR.lock).init(KERNEL_HEAP.start().as_usize(), KERNEL_HEAP.size());
    }

    // Set the CR4 fields, so we can then use the global
    // page flag when we remap the kernel.
    unsafe { Cr4::update(|flags| *flags |= Cr4Flags::PAGE_GLOBAL) }; // Enable the global flag in page tables.

    // Set the EFER fields, so we can use the no-execute
    // page flag when we remap the kernel.
    let mut flags = Efer::read();
    flags |= EferFlags::NO_EXECUTE_ENABLE; // Enable the no-execute flag in page tables.
    unsafe { Efer::write(flags) };

    Ok(())
}

/// Wrap a type in a [`spin::Mutex`] so we can
/// implement traits on a locked type.
///
struct Locked<A> {
    lock: Mutex<A>,
}

impl<A> Locked<A> {
    #[allow(dead_code)]
    pub const fn new(inner: A) -> Self {
        Locked {
            lock: Mutex::new(inner),
        }
    }
}
