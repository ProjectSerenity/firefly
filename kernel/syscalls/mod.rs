// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements the kernel's syscalls, allowing user processes to access kernel functionality.

use core::arch::global_asm;
use multitasking::thread;
use segmentation::with_segment_data;
use serial::println;
use syscalls::{Error, Syscall};
use x86_64::registers::model_specific::{Efer, EferFlags, LStar, SFMask, Star};
use x86_64::registers::rflags::RFlags;
use x86_64::VirtAddr;

global_asm!(include_str!("entry.s"));

// The syscall_entry function is implemented in entry.s.
//
extern "sysv64" {
    /// syscall_entry is the entry point invoked when a
    /// user process uses the SYSCALL instruction.
    ///
    fn syscall_entry();
}

/// The set of saved registers from the user thread that
/// is invoking a syscall.
///
#[repr(C, packed)]
#[derive(Clone, Copy, Debug)]
pub struct SavedRegisters {
    pub rax: usize,
    pub rbx: usize,
    // RCX is not preserved.
    pub rdx: usize,
    pub rsi: usize,
    pub rdi: usize,
    pub rbp: VirtAddr,
    pub rip: VirtAddr, // Return address.
    pub rsp: VirtAddr,
    pub r8: usize,
    pub r9: usize,
    pub r10: usize,
    // R11 is not preserved.
    pub r12: usize,
    pub r13: usize,
    pub r14: usize,
    pub r15: usize,
    pub rflags: RFlags,
}

/// The results structure is used internally to pass
/// the result value and error.
///
#[repr(C)]
struct SyscallResults {
    value: usize,
    error: usize,
}

/// Called by syscall_entry to call the relevant
/// syscall, or return an error if an invalid syscall
/// is invoked.
///
#[no_mangle]
extern "sysv64" fn syscall_handler(
    arg1: usize,
    arg2: usize,
    arg3: usize,
    arg4: usize,
    arg5: usize,
    arg6: usize,
    syscall_num: usize,
    registers: *mut SavedRegisters,
) -> SyscallResults {
    match Syscall::from_usize(syscall_num) {
        Some(Syscall::ExitThread) => {
            println!("Exiting user thread.");
            thread::exit();
        }
        None => {
            println!("Unrecognised syscall {}", syscall_num);
            println!("syscall_handler(");
            println!("    arg1: {:#016x},", arg1);
            println!("    arg2: {:#016x},", arg2);
            println!("    arg3: {:#016x},", arg3);
            println!("    arg4: {:#016x},", arg4);
            println!("    arg5: {:#016x},", arg5);
            println!("    arg6: {:#016x},", arg6);
            println!("    syscall_num: {:#016x},", syscall_num);
            println!("    registers: {:?})", unsafe { &*registers });
            SyscallResults {
                value: 0,
                error: Error::BadSyscall as usize,
            }
        }
    }
}

/// Sets up sycall handling for this CPU.
///
pub fn per_cpu_init() {
    // Set the segment selectors for the kernel
    // and userspace.
    with_segment_data(|segment_data| {
        let (kernel_code_64, kernel_data) = segment_data.kernel_selectors();
        let (_, user_data, user_code_64) = segment_data.user_selectors();
        Star::write(user_code_64, user_data, kernel_code_64, kernel_data).unwrap();
    });

    // Set the kernel's entry point when SYSCALL
    // is invoked.
    LStar::write(VirtAddr::from_ptr(syscall_entry as *const u8));

    // Mask off interrupts so that interrupts are
    // disabled when the interrupt handler starts.
    SFMask::write(RFlags::INTERRUPT_FLAG);

    // Enable the SYSCALL and SYSRET instructions.
    unsafe {
        Efer::update(|flags| *flags |= EferFlags::SYSTEM_CALL_EXTENSIONS);
    }
}
