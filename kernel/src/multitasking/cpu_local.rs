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
use x86_64::instructions::interrupts;
use x86_64::instructions::segmentation::{Segment, CS, SS};
use x86_64::instructions::tables::load_tss;
use x86_64::registers::model_specific::GsBase;
use x86_64::registers::model_specific::Msr;
use x86_64::structures::gdt::{Descriptor, GlobalDescriptorTable};
use x86_64::structures::paging::{
    FrameAllocator, Mapper, Page, PageSize, PageTableFlags, Size4KiB,
};
use x86_64::structures::tss::TaskStateSegment;
use x86_64::{PrivilegeLevel, VirtAddr};

/// The model-specific register used to provide
/// the user code and stack segment selectors to
/// SYSEXIT.
//
// We define that here, as it's not yet definned
// in [`x86_64::registers::model_specific`].
//
pub const IA32_SYSENTER_CS: Msr = Msr::new(0x174);

/// INITIALSED tracks whether the CPU-local
/// data has been set up on this CPU. It is
/// set in init() and can be checked by
/// calling ready().
///
static INITIALISED: AtomicBool = AtomicBool::new(false);

/// CpuData contains the data specific to an individual CPU core.
///
struct CpuData {
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
            id: cpu_id,
            idle_thread: idle.clone(),
            current_thread: idle,
            tss,
            double_fault_stack,
            gdt,
        });
    }

    // Set up the GDT now that everything
    // else is ready.
    interrupts::without_interrupts(|| {
        let tss_ref = tss_ref();
        let data = unsafe { cpu_data() };
        let kernel_code_selector = data.gdt.add_entry(Descriptor::kernel_code_segment());
        let kernel_stack_selector = data.gdt.add_entry(Descriptor::kernel_data_segment());
        let tss_selector = data.gdt.add_entry(Descriptor::tss_segment(tss_ref));
        let user_code_selector = data.gdt.add_entry(Descriptor::user_code_segment());
        let user_stack_selector = data.gdt.add_entry(Descriptor::user_data_segment());

        // Load the GDT and its selectors.
        data.gdt.load();
        unsafe {
            CS::set_reg(kernel_code_selector);
            SS::set_reg(kernel_stack_selector);
            GsBase::write(start); // Set the GS base again now we've updated GS.
            load_tss(tss_selector);

            // Check that the user code and stack
            // selectors will work correctly with
            // SYSEXIT, where the stack selector
            // is determined by adding 8 to the
            // code selector. See Intel 64 manual,
            // volume 2B, chapter 4, page 684 and
            // volume 3A, section 5.8.7.
            debug_assert_eq!(user_code_selector.0 + 8, user_stack_selector.0);

            // From Intel 64 manual, volume 3A,
            // section 5.8.7.1:
            //
            //     Target code segment - Computed by adding 32 to the value in the IA32_SYSENTER_CS.
            //
            // From volume 2B, chapter 4, page 686:
            //
            //     #GP(0)   If IA32_SYSENTER_CS = 0.
            //
            debug_assert!(user_code_selector.0 > 32);

            #[allow(const_item_mutation)] // Safe, as we don't actually modify the value.
            IA32_SYSENTER_CS.write(user_code_selector.0 as u64 - 32);
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
pub fn set_current_thread(thread: Arc<Thread>, interrupt_stack: VirtAddr) {
    let mut data = unsafe { cpu_data() };
    data.current_thread = thread;
    data.tss.privilege_stack_table[INTERRUPT_KERNEL_STACK_INDEX] = interrupt_stack;
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
