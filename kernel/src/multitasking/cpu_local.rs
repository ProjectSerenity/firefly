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

use crate::multitasking::thread::Thread;
use align::align_up_u64;
use alloc::sync::Arc;
use core::mem::size_of;
use core::pin::Pin;
use core::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use memlayout::{VirtAddrRange, CPU_LOCAL};
use physmem;
use segmentation::SegmentData;
use x86_64::instructions::interrupts::without_interrupts;
use x86_64::registers::model_specific::GsBase;
use x86_64::structures::paging::{
    FrameAllocator, Mapper, OffsetPageTable, Page, PageSize, PageTableFlags, Size4KiB,
};
use x86_64::VirtAddr;

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

    // Our global descriptor table and task
    // state segment.
    segment_data: SegmentData,
}

/// Initialise the current CPU's local data using the given CPU
/// ID and stack space.
///
pub fn init(
    cpu_id: CpuId,
    mapper: &mut OffsetPageTable,
    stack_space: &VirtAddrRange,
    current_segment_data: Pin<&mut SegmentData>,
) {
    // Next, work out where we will store our CpuId
    // data. We align up to page size to make paging
    // easier.
    let size = align_up_u64(size_of::<CpuData>() as u64, Size4KiB::SIZE);
    let start = CPU_LOCAL.start() + cpu_id.as_u64() * size;
    let end = start + size;

    // The page addresses should already be aligned,
    // so we shouldn't get any panics here.
    let start_page = Page::from_start_address(start).expect("bad start address");
    let end_page = Page::from_start_address(end).expect("bad end address");

    // Map our per-CPU address space.
    let mut frame_allocator = physmem::ALLOCATOR.lock();
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
            segment_data: SegmentData::new_uninitialised(),
        });
    }

    // Set up the segment data now that everything
    // else is ready.
    without_interrupts(|| {
        let mut segment_data = segment_data();
        segment_data.init();
        segment_data.swap(current_segment_data);
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

/// Returns a static reference to our segment data.
///
/// Note that it's safe to return a static reference,
/// as the segment data doesn't change once CPU-local
/// data is set up.
///
pub fn segment_data() -> Pin<&'static mut SegmentData> {
    Pin::new(unsafe { &mut cpu_data().segment_data })
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

/// Overwrites the currently executing thread.
///
pub fn set_current_thread(thread: Arc<Thread>) {
    let mut data = unsafe { cpu_data() };

    // Save the current thread's user stack pointer.
    data.current_thread.set_user_stack(data.user_stack_pointer);

    // Overwrite the state from the new thread.
    Pin::new(&mut data.segment_data).set_interrupt_stack(thread.interrupt_stack());
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

impl Default for CpuId {
    fn default() -> Self {
        Self::new()
    }
}
