// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides functionality to access a different copy of the same
//! structure on each CPU.
//!
//! This module allows each CPU to track data like the CPU ID and
//! the currently-executing thread.
//!
//! For now, we take a simple but slightly inefficient approach, where we
//! allocate a copy of the CpuData struct in the CPU-local address space
//! and store a pointer to it in the GS base. To access the data, we use
//! a wrapper function to retrieve the pointer from the GS base, casting
//! it to the right type, then access the data as usual.
//!
//! This is less efficient than using offsets from the GS base directly
//! in assembly, as described [here], but it's much simpler to implement.
//! If [rust-osdev/x86_64#257](https://github.com/rust-osdev/x86_64/pull/257)
//! is merged, that will probably be used to replace this module.
//!
//! [here]: https://github.com/rust-osdev/x86_64/pull/257#issuecomment-849514649

use crate::gdt::DOUBLE_FAULT_IST_INDEX;
use crate::memory::{kernel_pml4, pmm, VirtAddrRange, CPU_LOCAL};
use crate::multitasking::thread::Thread;
use alloc::sync::Arc;
use alloc::vec;
use alloc::vec::Vec;
use core::mem::size_of;
use core::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use x86_64::addr::align_up;
use x86_64::instructions::interrupts::without_interrupts;
use x86_64::instructions::tables::load_tss;
use x86_64::registers::model_specific::GsBase;
use x86_64::structures::gdt::{
    Descriptor, DescriptorFlags, GlobalDescriptorTable, SegmentSelector,
};
use x86_64::structures::paging::{
    FrameAllocator, Mapper, Page, PageSize, PageTableFlags, Size4KiB,
};
use x86_64::structures::tss::TaskStateSegment;
use x86_64::{PrivilegeLevel, VirtAddr};

/// INITIALSED tracks whether the CPU-local
/// data has been set up on this CPU. It is
/// set in init() and can be checked by
/// calling ready().
///
static INITIALISED: AtomicBool = AtomicBool::new(false);

/// CpuData contains the data specific to an individual CPU core.
///
struct CpuData {
    // The current thread's syscall stack
    // pointer. We read this when entering
    // the syscall handler to switch to
    // the syscall stack. This is fetched
    // by dereferencing the GS register,
    // as the syscall handler is written
    // in assembly. It's important that
    // the location of thia value within
    // CpuData not be changed without also
    // changing it in the syscall handler.
    syscall_stack_pointer: VirtAddr,

    // The current thread's user stack
    // pointer. This is overwritten when
    // we enter the syscall handler and
    // switch to the syscall stack, and
    // restored when we return to user
    // space. As above, do not move within
    // CpuData without updating the syscall
    // handler too.
    user_stack_pointer: VirtAddr,

    // This CPU's unique ID.
    id: CpuId,

    // This CPU's idle thread.
    idle_thread: Arc<Thread>,

    // The currently executing thread on
    // this CPU.
    current_thread: Arc<Thread>,

    // Our task state segment.
    tss: TaskStateSegment,

    // The stack we reserve for the double
    // fault handler. We only store this
    // so it doesn't get dropped.
    #[allow(dead_code)]
    double_fault_stack: Vec<u8>,

    // Our descriptor table.
    gdt: GlobalDescriptorTable,

    kernel_code_64: SegmentSelector,
    kernel_data: SegmentSelector,
    user_code_32: SegmentSelector,
    user_data: SegmentSelector,
    user_code_64: SegmentSelector,
}

/// Initialise the current CPU's local data using the given CPU
/// ID and stack space.
///
pub fn init(cpu_id: CpuId, stack_space: &VirtAddrRange) {
    // Next, work out where we will store our CpuId
    // data. We align up to page size to make paging
    // easier.
    let size = align_up(size_of::<CpuId>() as u64, Size4KiB::SIZE);
    let start = CPU_LOCAL.start() + cpu_id.as_u64() * size;
    let end = start + size;

    // The page addresses should already be aligned,
    // so we shouldn't get any panics here.
    let start_page = Page::from_start_address(start).expect("bad start address");
    let end_page = Page::from_start_address(end).expect("bad end address");

    // Map our per-CPU address space.
    let mut mapper = unsafe { kernel_pml4() };
    let mut frame_allocator = pmm::ALLOCATOR.lock();
    for page in Page::range(start_page, end_page) {
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

    // Store the pointer to the CpuData in the GS base.
    GsBase::write(start);

    // Create our idle thread.
    let idle = Thread::new_idle_thread(stack_space);

    // Set up our TSS. We set up the GDT
    // once our per-CPU data is ready, as
    // we need to access its address to
    // add the TSS segment.
    let mut tss = TaskStateSegment::new();
    const STACK_SIZE: usize = 4096 * 5;
    let double_fault_stack = vec![0u8; STACK_SIZE];
    let stack_start = VirtAddr::from_ptr(double_fault_stack.as_ptr());
    let stack_end = stack_start + STACK_SIZE;
    tss.interrupt_stack_table[DOUBLE_FAULT_IST_INDEX as usize] = stack_end;
    let gdt = GlobalDescriptorTable::new();

    // Initialise the CpuData from a pointer at the
    // start of the address space.
    let cpu_local_data = start.as_mut_ptr() as *mut CpuData;
    unsafe {
        cpu_local_data.write(CpuData {
            syscall_stack_pointer: VirtAddr::zero(),
            user_stack_pointer: VirtAddr::zero(),
            id: cpu_id,
            idle_thread: idle.clone(),
            current_thread: idle,
            tss,
            double_fault_stack,
            gdt,
            // Placeholder values until we set up
            // the GDT below.
            kernel_code_64: SegmentSelector(0),
            kernel_data: SegmentSelector(0),
            user_code_32: SegmentSelector(0),
            user_data: SegmentSelector(0),
            user_code_64: SegmentSelector(0),
        });
    }

    // Set up the GDT now that everything
    // else is ready.
    without_interrupts(|| {
        let tss_ref = tss_ref();
        let data = unsafe { cpu_data() };
        let kernel_code_64 = data.gdt.add_entry(Descriptor::kernel_code_segment());
        let kernel_data = data.gdt.add_entry(Descriptor::kernel_data_segment());
        let tss_selector = data.gdt.add_entry(Descriptor::tss_segment(tss_ref));
        let user_code_32 = data
            .gdt
            .add_entry(Descriptor::UserSegment(DescriptorFlags::USER_CODE32.bits()));
        let user_data = data.gdt.add_entry(Descriptor::user_data_segment());
        let user_code_64 = data.gdt.add_entry(Descriptor::user_code_segment());

        // Load the GDT and its selectors.
        data.gdt.load();
        unsafe {
            load_tss(tss_selector);

            // Store our segment selectors.
            let mut data = &mut *cpu_local_data;
            data.kernel_code_64 = kernel_code_64;
            data.kernel_data = kernel_data;
            data.user_code_32 = user_code_32;
            data.user_data = user_data;
            data.user_code_64 = user_code_64;
        }
    });

    INITIALISED.store(true, Ordering::Relaxed);
}

/// Returns whether the CPU-local data has been initialised
/// on this CPU.
///
pub fn ready() -> bool {
    INITIALISED.load(Ordering::Relaxed)
}

// Helper functions to expose the CPU data.

/// cpu_data is our helper function to get
/// the pointer to the CPU data from the
/// GS register.
///
unsafe fn cpu_data() -> &'static mut CpuData {
    let ptr = GsBase::read();
    if ptr.is_null() {
        panic!("CPU-local data fetched but GS base is 0");
    }

    &mut *(ptr.as_mut_ptr() as *mut CpuData)
}

/// Fetches a static reference to our TSS.
///
fn tss_ref() -> &'static TaskStateSegment {
    unsafe { &cpu_data().tss }
}

/// Returns this CPU's unique ID.
///
pub fn cpu_id() -> CpuId {
    unsafe { cpu_data() }.id
}

/// Returns this CPU's idle thread.
///
pub fn idle_thread() -> Arc<Thread> {
    unsafe { cpu_data() }.idle_thread.clone()
}

/// Returns the code and stack segment selectors
/// for the kernel in this CPU.
///
pub fn kernel_selectors() -> (SegmentSelector, SegmentSelector) {
    let data = unsafe { cpu_data() };
    (data.kernel_code_64, data.kernel_data)
}

/// Returns the code and stack segment selectors
/// for 32-bit user code in this CPU.
///
pub fn user_selectors() -> (SegmentSelector, SegmentSelector, SegmentSelector) {
    let data = unsafe { cpu_data() };
    (data.user_code_32, data.user_data, data.user_code_64)
}

/// Returns the currently executing thread.
///
pub fn current_thread() -> Arc<Thread> {
    unsafe { cpu_data() }.current_thread.clone()
}

/// Index into the TSS where the userland interrupt
/// handler's stack is stored.
///
/// User threads have a stack space in ring 3,
/// so we cannot use that thread for handling
/// interrupts. As a result, each user thread
/// has an additional stack, allocated in
/// kernel space. Kernel threads don't need
/// the extra stack, so they use their existing
/// stack.
///
/// Each time we switch thread, we update this
/// index with the new thread's kernel stack
/// top (or 0 for kernel threads).
///
const INTERRUPT_KERNEL_STACK_INDEX: usize = PrivilegeLevel::Ring0 as usize;

/// Overwrites the currently executing thread.
///
pub fn set_current_thread(thread: Arc<Thread>) {
    let mut data = unsafe { cpu_data() };

    // Save the current thread's user stack pointer.
    data.current_thread.set_user_stack(data.user_stack_pointer);

    // Overwrite the state from the new thread.
    data.tss.privilege_stack_table[INTERRUPT_KERNEL_STACK_INDEX] = thread.interrupt_stack();
    data.syscall_stack_pointer = thread.syscall_stack();
    data.user_stack_pointer = thread.user_stack();
    data.current_thread = thread;
}

/// Uniquely identifies a CPU core.
///
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub struct CpuId(u64);

impl CpuId {
    /// Allocates and returns the next available CpuId.
    ///
    pub fn new() -> Self {
        static NEXT_CPU_ID: AtomicU64 = AtomicU64::new(0);
        CpuId(NEXT_CPU_ID.fetch_add(1, Ordering::Relaxed))
    }

    /// Returns a numerical representation for the CPU
    /// ID.
    ///
    pub const fn as_u64(&self) -> u64 {
        self.0
    }
}
