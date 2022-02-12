// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Manages segmentation for the kernel, including the [Global Descriptor Table] (GDT).
//!
//! This crate provides a single consistent approach to handling the GDT and TSS, no
//! matter where in memory they are stored. The GDT layout used is as follows:
//!
//! | Index | Descriptor           | Comment                                                   |
//! |-------|----------------------|-----------------------------------------------------------|
//! | 0     | NULL                 | Not usable                                                |
//! | 1     | `kernel_code_64`     | All kernel code, kernel only executes in 64-bit long mode |
//! | 2     | `kernel_data`        | Kernel stacks                                             |
//! | 3 - 4 | `task_state_segment` | Uses up two slots                                         |
//! | 5     | `user_code_32`       | User code when running in 32-bit compatibility mode       |
//! | 6     | `user_data`          | User stacks (both 32 and 64 bit mode)                     |
//! | 7     | `user_code_64`       | User code when running in 64-bit long mode                |
//!
//! [Global Descriptor Table]: https://en.wikipedia.org/wiki/Global_Descriptor_Table

#![no_std]

extern crate alloc;

use alloc::boxed::Box;
use alloc::vec::Vec;
use core::pin::Pin;
use lazy_static::lazy_static;
use spin::Mutex;
use x86_64::instructions::segmentation::{Segment, CS, SS};
use x86_64::instructions::tables::load_tss;
use x86_64::structures::gdt::{
    Descriptor, DescriptorFlags, GlobalDescriptorTable, SegmentSelector,
};
use x86_64::structures::tss::TaskStateSegment;
use x86_64::{PrivilegeLevel, VirtAddr};

/// This is used as the bootstrap segment data until the
/// kernel heap is up and running.
///
/// Although this is a mutable static, it's safe in practice,
/// as it's initialised once and then used once. We check
/// that this is the sequence of events. We don't expose
/// the bootstrap data outside this module.
///
static mut BOOTSTRAP_SEGMENT_DATA: SegmentData = SegmentData::new_uninitialised();

/// Bootstrap the segment data with an initial
/// global set. This should be used until the
/// kernel heap can be set up and the per-CPU
/// segment data can be initialised with [`per_cpu_init`].
///
pub fn bootstrap() {
    let mut pinned = unsafe { Pin::new(&mut BOOTSTRAP_SEGMENT_DATA) };
    pinned.init();
    pinned.activate();
}

lazy_static! {
    /// The segment data for each CPU.
    ///
    static ref PER_CPU: Mutex<Vec<Pin<&'static mut SegmentData>>> = Mutex::new(Vec::new());
}

/// Initialise the per-CPU segment data, which
/// is used for the rest of the kernel's
/// lifetime.
///
pub fn per_cpu_init() {
    // Make sure the PER_CPU vector has enough entries
    // for our CPU id.
    let mut per_cpu = PER_CPU.lock();
    let cpu_id = cpu::id();
    while per_cpu.len() <= cpu_id {
        // We allocate and initialise but don't activate
        // the entries so they're only activated once by
        // their owning CPU.
        let segment_data = Box::new(SegmentData::new_uninitialised()); // Allocate on the heap.
        let segment_data = Box::leak(segment_data); // Leak as a &'static mut.
        let mut segment_data = Pin::new(segment_data); // Pin so it can't move.
        segment_data.init();
        per_cpu.push(segment_data);
    }

    unsafe { per_cpu[cpu_id].swap(Pin::new(&mut BOOTSTRAP_SEGMENT_DATA)) };
}

/// Invoke a callback acting on the segment data.
///
pub fn with_segment_data<F: FnOnce(&mut Pin<&mut SegmentData>)>(f: F) {
    let mut per_cpu = PER_CPU.lock();
    if let Some(segment_data) = per_cpu.get_mut(cpu::id()) {
        f(segment_data);
    } else {
        panic!("segmentation::with_segment_data() called before being initialised.");
    }
}

/// Index into each TSS where the double fault
/// handler stack is stored.
///
/// This ensures that the double fault handler
/// operates with a known-good stack so that
/// a kernel stack overflow does not result in
/// a page fault in the double handler, leading
/// to a triple fault.
///
pub const DOUBLE_FAULT_IST_INDEX: u16 = 0;

/// The size of the extra stack reserved for the double fault
/// handler.
///
/// If we are unable to use our current stack when entering a
/// CPU exception handler, such as if we have a stack overflow,
/// then the resulting double fault will immediately become a
/// triple fault and reset the CPU. This makes debugging hard,
/// as we have no way to capture what went wrong.
///
/// To prevent this from happening, we allocate a separate stack
/// just for the double fault handler, so we can be sure we have
/// a safe stack to use. That extra stack is the following size
/// in bytes.
///
const DOUBLE_FAULT_STACK_SIZE: usize = 4096 * 5; // 20 KiB.

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

/// Contains the global descriptor table and task state segment.
///
/// This can be loaded into the CPU to activate both structures.
///
pub struct SegmentData {
    // Our descriptor table.
    gdt: GlobalDescriptorTable,

    // Our task state segment.
    tss: TaskStateSegment,

    // Our segment selectors.
    kernel_code_64: SegmentSelector,
    kernel_data: SegmentSelector,
    task_state: SegmentSelector,
    user_code_32: SegmentSelector,
    user_data: SegmentSelector,
    user_code_64: SegmentSelector,

    // Whether the structures are set up
    // and currently in use.
    is_initialised: bool,
    is_active: bool,

    // The stack we reserve for the double
    // fault handler. We only store this
    // so it doesn't get dropped.
    //
    // Placed last in the struct to minimise
    // padding.
    #[allow(dead_code)]
    double_fault_stack: [u8; DOUBLE_FAULT_STACK_SIZE],
}

impl SegmentData {
    /// Returns an uninitialised GDT.
    ///
    /// It's necessary to create and initialise the GDT in two
    /// separate steps so that the initialisation takes place
    /// on the final address.
    ///
    const fn new_uninitialised() -> Self {
        SegmentData {
            gdt: GlobalDescriptorTable::new(),
            tss: TaskStateSegment::new(),

            kernel_code_64: SegmentSelector(0),
            kernel_data: SegmentSelector(0),
            task_state: SegmentSelector(0),
            user_code_32: SegmentSelector(0),
            user_data: SegmentSelector(0),
            user_code_64: SegmentSelector(0),

            is_initialised: false,
            is_active: false,

            double_fault_stack: [0u8; DOUBLE_FAULT_STACK_SIZE],
        }
    }

    /// Returns a static, immutable reference to the TSS. This is
    /// necessary, due to the [`tss_segment`] API.
    ///
    /// Note that although we use unsafe and cheat the type system,
    /// this is actually safe, as we check that the segment data is
    /// no longer active when it drops. This means we can safely
    /// use a static reference from a shorter-lived object, as the
    /// static reference will drop before the host object, or it
    /// will panic.
    ///
    /// This doesn't need the inner reference to be mutable, but if
    /// it isn't, we can't call it from [`init`](Self::init).
    ///
    /// [`tss_segment`]: https://docs.rs/x86_64/0.14.7/x86_64/structures/gdt/enum.Descriptor.html#method.tss_segment
    ///
    fn tss_ref(self: &Pin<&mut Self>) -> &'static TaskStateSegment {
        let ptr = (&self.tss) as *const TaskStateSegment;
        unsafe { &*ptr }
    }

    /// Initialise the segment data, making it ready to make
    /// active with [`activate`](Self::activate) or [`swap`](Self::swap).
    ///
    /// # Panics
    ///
    /// `init` will panic if the data has already been initialised.
    ///
    fn init(self: &mut Pin<&mut Self>) {
        if self.is_initialised {
            panic!("SegmentData is being initialised a second time");
        }

        // Set up the TSS.
        let stack_bottom = VirtAddr::from_ptr(&self.double_fault_stack);
        let stack_top = stack_bottom + self.double_fault_stack.len();
        self.tss.interrupt_stack_table[DOUBLE_FAULT_IST_INDEX as usize] = stack_top;

        // Then create the segment selectors.
        let tss_ref = self.tss_ref();
        self.kernel_code_64 = self.gdt.add_entry(Descriptor::kernel_code_segment());
        self.kernel_data = self.gdt.add_entry(Descriptor::kernel_data_segment());
        self.task_state = self.gdt.add_entry(Descriptor::tss_segment(tss_ref));
        self.user_code_32 = self
            .gdt
            .add_entry(Descriptor::UserSegment(DescriptorFlags::USER_CODE32.bits()));
        self.user_data = self.gdt.add_entry(Descriptor::user_data_segment());
        self.user_code_64 = self.gdt.add_entry(Descriptor::user_code_segment());

        self.is_initialised = true;
    }

    /// Activate the segment data, loading it into the CPU.
    ///
    /// Note that this is designed for cases where there is
    /// currently no segment data in the CPU, or the code
    /// that managed the current segment data was implemented
    /// separately. To swap one set of segment data with another,
    /// use [`swap`](Self::swap) instead.
    ///
    /// # Panics
    ///
    /// `activate` will panic if the data has already been
    /// activated, or if the data has not been initialised.
    ///
    /// If another set of segment data is currently active and
    /// `activate` is used to swap, rather than [`swap`](Self::swap),
    /// the other segment data will panic when it drops.
    ///
    fn activate(self: &mut Pin<&mut Self>) {
        if !self.is_initialised {
            panic!("SegmentData is being activated before being initialised");
        }

        if self.is_active {
            panic!("SegmentData is being activated a second time");
        }

        unsafe {
            // This is safe in practice, as we enforce the
            // lifetime constraint by panicking if the segment
            // data is dropped while active.
            self.gdt.load_unsafe();

            CS::set_reg(self.kernel_code_64);
            SS::set_reg(self.kernel_data);

            load_tss(self.task_state);
        }

        self.is_active = true;
    }

    /// Activate the segment data, loading it into the CPU
    /// and replacing `previous`.
    ///
    /// If there is currently no segment data loaded, or if
    /// it was loaded by another implementation, use [`activate`](Self::activate)
    /// instead.
    ///
    /// # Panics
    ///
    /// `swap` will panic if the data has already been
    /// activated, or if `previous` is not currently loaded.
    ///
    fn swap(self: &mut Pin<&mut Self>, mut previous: Pin<&mut Self>) {
        if !previous.is_active {
            panic!("previous SegmentData is not currently active");
        }

        self.activate();

        previous.is_active = false;
    }

    /// Sets the stack used for handling interrupts.
    ///
    /// This should be set whenever the current user thread
    /// changes, so that there is no risk of stack corruption.
    ///
    /// The passed address should be the address of the top
    /// of the stack, or zero.
    ///
    pub fn set_interrupt_stack(self: &mut Pin<&mut Self>, stack_top: VirtAddr) {
        self.tss.privilege_stack_table[INTERRUPT_KERNEL_STACK_INDEX] = stack_top;
    }

    /// Returns the kernel's 64-bit code and data segment
    /// selectors.
    ///
    pub fn kernel_selectors(self: &Pin<&mut Self>) -> (SegmentSelector, SegmentSelector) {
        (self.kernel_code_64, self.kernel_data)
    }

    /// Returns the user 32-bit code, data, and 64-bit code
    /// segment selectors.
    ///
    pub fn user_selectors(
        self: &Pin<&mut Self>,
    ) -> (SegmentSelector, SegmentSelector, SegmentSelector) {
        (self.user_code_32, self.user_data, self.user_code_64)
    }
}

impl Drop for SegmentData {
    fn drop(&mut self) {
        if self.is_active {
            panic!("SegmentData was dropped while still active in the CPU");
        }
    }
}
