// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Manages the [Global Descriptor Table](https://en.wikipedia.org/wiki/Global_Descriptor_Table) (GDT).
//!
//! This module governs the GDT, which is currently only used to store the
//! kernel's code segment and the task state segment, which is used to store
//! a safe stack for the double fault handler.
//!
//! We also store a kernel data segment for use with the GS segment to
//! store per-CPU data. See [`cpu_local`](crate::multitasking::cpu_local)
//! for more details.
//!
//! This version of the GDT and TSS are only used to bootstrap the kernel.
//! Once we have created the CPU-local data, we switch over to a per-CPU
//! GDT and TSS. After that point, any changes made to this module will
//! have no effect.

use lazy_static::lazy_static;
use x86_64::instructions::segmentation::{Segment, CS, GS, SS};
use x86_64::instructions::tables::load_tss;
use x86_64::structures::gdt::{Descriptor, GlobalDescriptorTable, SegmentSelector};
use x86_64::structures::tss::TaskStateSegment;
use x86_64::VirtAddr;

/// Installs the global descriptor table,
/// along with the code and task state segments.
///
pub fn init() {
    GDT.0.load();
    unsafe {
        CS::set_reg(GDT.1.kernel_code_selector);
        SS::set_reg(GDT.1.kernel_stack_selector);
        GS::set_reg(GDT.1.cpu_local_selector);
        load_tss(GDT.1.tss_selector);
    }
}

lazy_static! {
    static ref GDT: (GlobalDescriptorTable, Selectors) = {
        let mut gdt = GlobalDescriptorTable::new();
        let kernel_code_selector = gdt.add_entry(Descriptor::kernel_code_segment());
        let kernel_stack_selector = gdt.add_entry(Descriptor::kernel_data_segment());
        let tss_selector = gdt.add_entry(Descriptor::tss_segment(&TSS));
        let cpu_local_selector = gdt.add_entry(Descriptor::kernel_data_segment());
        (
            gdt,
            Selectors {
                kernel_code_selector,
                kernel_stack_selector,
                tss_selector,
                cpu_local_selector,
            },
        )
    };
}

struct Selectors {
    kernel_code_selector: SegmentSelector,
    kernel_stack_selector: SegmentSelector,
    tss_selector: SegmentSelector,
    cpu_local_selector: SegmentSelector,
}

// Set up the task state segment with a safe
// backup stack for the double fault handler.

/// Index into the TSS where the double fault
/// handler stack is stored.
///
/// This ensures that the double fault handler
/// operates with a known-good stack so that
/// a kernel stack overflow does not result in
/// a page fault in the double handler, leading
/// to a triple fault.
///
pub const DOUBLE_FAULT_IST_INDEX: u16 = 0;

lazy_static! {
    static ref TSS: TaskStateSegment = {
        let mut tss = TaskStateSegment::new();
        tss.interrupt_stack_table[DOUBLE_FAULT_IST_INDEX as usize] = {
            const STACK_SIZE: usize = 4096 * 5;
            static mut STACK: [u8; STACK_SIZE] = [0; STACK_SIZE];

            let stack_start = VirtAddr::from_ptr(unsafe { &STACK });
            let stack_end = stack_start + STACK_SIZE;
            stack_end
        };
        tss
    };
}
