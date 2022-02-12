// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Handles CPU-local data.
//!
//! This module provides two mechanisms for storing and
//! accessing separate data on each CPU, both using the GS
//! segment. Each CPU is allocated a small region of memory,
//! storing the address into the GS base.
//!
//! The first 8 bytes of the region contains the CPU's
//! unique id, which can be used by other modules to index
//! into a vector of per-CPU data specific to that module.
//!
//! The second 8 bytes contains the current thread's syscall
//! stack pointer. This is stored here so it can be accessed
//! easily from the syscall handler's assembly code.
//!
//! The third 8 bytes contains the current thread's user
//! stack pointer. Like the syscall stack pointer above,
//! this is stored here so it can be accessed easily from
//! assembly by dereferencing the GS register.
//!
//! ## Initialisation
//!
//! The initial CPU must call [`init`] to setup the CPU-local
//! memory region. This will identify the maximum number of
//! logical cores so the right sized region can be allocated.
//!
//! After the initial CPU has initialised the CPU-local
//! data, each CPU (including the first) must call [`per_cpu_init`]
//! to initialise that CPU's local data and set the GS base
//! address.
//!
//! Once a CPU has initialised the CPU-local data, it can
//! call [`id`] to retrieve a unique CPU identifier (indexed
//! from 0). Calling [`max_cores`](super::max_cores) will
//! return the maximum number of logical cores, which is larger
//! than the largest value that will be returned by `id`. This
//! can be used to initialise a vector containing separate
//! data for each CPU, which can be indexed by calling `id`.

use core::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use memlayout::CPU_LOCAL;
use physmem::ALLOCATOR;
use virtmem::with_page_tables;
use x86_64::registers::model_specific::GsBase;
use x86_64::structures::paging::{FrameAllocator, Mapper, Page, PageTableFlags};
use x86_64::VirtAddr;

/// The offset into the per-CPU data at which the 8-byte
/// unique CPU id is stored.
///
const CPU_ID_OFFSET: usize = 0;

/// The offset into the per-CPU data at which the 8-byte
/// syscall stack pointer for the current thread is stored.
///
/// We read this when entering the syscall handler to switch
/// to the syscall stack. This is fetched by dereferencing
/// the GS register, as the syscall handler is written in
/// assembly.
///
/// This value must not be changed without also changing it
/// in the syscall handler assembly code.
///
const SYSCALL_STACK_POINTER_OFFSET: usize = 8;

/// The offset into the per-CPU data at which the 8-byte
/// user stack pointer for the current thread is stored.
///
/// We overwrite this when entering the syscall handler and
/// switch to the syscall stack. It is then restored when
/// we return to user space. This is fetched by dereferencing
/// the GS register, as the syscall handler is written in
/// assembly.
///
/// This value must not be changed without also changing it
/// in the syscall handler assembly code.
///
const USER_STACK_POINTER_OFFSET: usize = 16;

/// The number of bytes per CPU in the CPU-local memory
/// region. This is the sum of the fields in the data:
///
/// - CPU id (8 bytes)
/// - Syscall stack pointer (8 bytes)
/// - User stack pointer (8 bytes)
///
const PER_CPU_REGION_SIZE: usize = 8 + 8 + 8;

/// Initialise the CPU-local data.
///
/// # Panics
///
/// Calling `init` more than once will result in a panic.
///
pub fn init() {
    static INITIALISED: AtomicBool = AtomicBool::new(false);
    let prev = INITIALISED.fetch_or(true, Ordering::SeqCst);
    if prev {
        panic!("cpu::init() called more than once");
    }

    // Get the max number of cores so we can map the
    // per-CPU memory region.
    let max_cores = super::max_cores();
    let region_size = max_cores * PER_CPU_REGION_SIZE;
    let start = CPU_LOCAL.start();
    let end = start + region_size;

    let start_page = Page::from_start_address(start).expect("bad start address");
    let last_page = Page::containing_address(end);

    // Map our per-CPU address space.
    with_page_tables(|mapper| {
        let mut frame_allocator = ALLOCATOR.lock();
        for page in Page::range_inclusive(start_page, last_page) {
            let frame = frame_allocator
                .allocate_frame()
                .expect("failed to allocate for per-CPU data");

            let flags = PageTableFlags::PRESENT
                | PageTableFlags::GLOBAL
                | PageTableFlags::WRITABLE
                | PageTableFlags::NO_EXECUTE;
            unsafe {
                mapper
                    .map_to(page, frame, flags, &mut *frame_allocator)
                    .expect("failed to map per-CPU data")
                    .flush()
            };
        }
    });

    // We don't need to actually initialise the region,
    // as it's easier to do in each CPU and avoids us
    // initialising bits of the region that are never
    // used if we use fewer than the maximum number of
    // cores.
}

/// Initialise this CPU's unique memory region.
///
/// # Panics
///
/// `per_cpu_init` will panic if called more than once on
/// any individual CPU.
///
pub fn per_cpu_init() {
    // This tracks the next valid unique CPU id.
    static NEXT_CPU_ID: AtomicUsize = AtomicUsize::new(0);

    if !GsBase::read().is_null() {
        panic!("cpu::per_cpu_init() called for the second time on this CPU.");
    }

    let id = NEXT_CPU_ID.fetch_add(1, Ordering::Relaxed);
    let start = CPU_LOCAL.start() + (id * PER_CPU_REGION_SIZE);
    GsBase::write(start);

    // Initialise the data. It's much simpler and quicker
    // to do this in assembly.
    let cpu = id as u64;
    let sys = 0u64;
    let usr = 0u64;
    unsafe {
        asm!(
            "mov gs:[{cpu_offset}], {cpu}",
            "mov gs:[{sys_offset}], {sys}",
            "mov gs:[{usr_offset}], {usr}",
            cpu_offset = const CPU_ID_OFFSET,
            cpu = in(reg) cpu,
            sys_offset = const SYSCALL_STACK_POINTER_OFFSET,
            sys = in(reg) sys,
            usr_offset = const USER_STACK_POINTER_OFFSET,
            usr = in(reg) usr,
        );
    }
}

/// Returns this CPU's unique identifier.
///
/// The values returned by `id` start at 0.
///
/// # Panics
///
/// Calling `id` will panic if this CPU has not yet called
/// [`init`], or if the GS segment selector has been changed
/// since `init` was called.
///
#[inline(always)]
pub fn id() -> usize {
    let mut id: u64;
    unsafe {
        asm!("mov {id}, gs:[{offset}]", id = out(reg) id, offset = const CPU_ID_OFFSET);
    }

    id as usize
}

/// Returns the current thread's saved syscall stack
/// pointer.
///
#[inline(always)]
pub fn syscall_stack_pointer() -> VirtAddr {
    let mut ptr: u64;
    unsafe {
        asm!("mov {ptr}, gs:[{offset}]", ptr = out(reg) ptr, offset = const SYSCALL_STACK_POINTER_OFFSET);
    }

    VirtAddr::new(ptr)
}

/// Overwrite the current thread's saved syscall stack
/// pointer.
///
#[inline(always)]
pub fn set_syscall_stack_pointer(ptr: VirtAddr) {
    let ptr = ptr.as_u64();
    unsafe {
        asm!("mov gs:[{offset}], {ptr}", ptr = in(reg) ptr, offset = const SYSCALL_STACK_POINTER_OFFSET);
    }
}

/// Returns the current thread's saved user stack
/// pointer.
///
#[inline(always)]
pub fn user_stack_pointer() -> VirtAddr {
    let mut ptr: u64;
    unsafe {
        asm!("mov {ptr}, gs:[{offset}]", ptr = out(reg) ptr, offset = const USER_STACK_POINTER_OFFSET);
    }

    VirtAddr::new(ptr)
}

/// Overwrite the current thread's saved user stack
/// pointer.
///
#[inline(always)]
pub fn set_user_stack_pointer(ptr: VirtAddr) {
    let ptr = ptr.as_u64();
    unsafe {
        asm!("mov gs:[{offset}], {ptr}", ptr = in(reg) ptr, offset = const USER_STACK_POINTER_OFFSET);
    }
}
