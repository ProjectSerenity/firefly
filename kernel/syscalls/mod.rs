// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements the kernel's syscalls, allowing user processes to access kernel functionality.

#[allow(clippy::enum_variant_names)]
mod gensyscalls {
    include!(env!("SYSCALLS_RS"));
}

use core::arch::global_asm;
use gensyscalls::{Error, Syscalls};
use memory::VirtAddr;
use multitasking::thread;
use segmentation::with_segment_data;
use serial::{print, println};
use x86_64::registers::model_specific::{Efer, EferFlags, LStar, SFMask, Star};
use x86_64::registers::rflags::RFlags;

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
    match Syscalls::from_u64(syscall_num as u64) {
        Some(Syscalls::ExitThread) => {
            println!("Exiting user thread.");
            thread::exit();
        }
        Some(Syscalls::PrintMessage) => {
            // fn print_message(ptr: *const u8, len: usize) -> (written: usize, error: Error)
            // There are no pointers to pointers
            // so we can consume the arguments
            // straight away.
            //
            // It's a little tacky, but using
            // UNIX shell colours helps us to
            // differentiate user print_message
            // and print_error from the kernel's
            // println.
            let b = unsafe { core::slice::from_raw_parts(arg1 as *const u8, arg2) };
            if let Ok(s) = core::str::from_utf8(b) {
                print!("\x1b[1;32m"); // Green foreground.
                let written = if serial::write_str(s).is_err() {
                    // We handle a failure to write the
                    // message as having written nothing,
                    // rather than returning an error.
                    0
                } else {
                    arg2
                };
                print!("\x1b[0m"); // Reset colour.

                let value = written;
                let error = Error::NoError as usize;

                SyscallResults { value, error }
            } else {
                let value = 0;
                let error = Error::IllegalParameter as usize;

                SyscallResults { value, error }
            }
        }
        Some(Syscalls::PrintError) => {
            // fn print_error(ptr: *const u8, len: usize) -> (written: usize, error: Error)
            // There are no pointers to pointers
            // so we can consume the arguments
            // straight away.
            //
            // It's a little tacky, but using
            // UNIX shell colours helps us to
            // differentiate user print_message
            // and print_error from the kernel's
            // println.
            let b = unsafe { core::slice::from_raw_parts(arg1 as *const u8, arg2) };
            if let Ok(s) = core::str::from_utf8(b) {
                print!("\x1b[1;31m"); // Red foreground.
                let written = if serial::write_str(s).is_err() {
                    // We handle a failure to write the
                    // message as having written nothing,
                    // rather than returning an error.
                    0
                } else {
                    arg2
                };
                print!("\x1b[0m"); // Reset colour.

                let value = written;
                let error = Error::NoError as usize;

                SyscallResults { value, error }
            } else {
                let value = 0;
                let error = Error::IllegalParameter as usize;

                SyscallResults { value, error }
            }
        }
        Some(Syscalls::ReadRandom) => {
            // fn read_random(ptr: *mut u8, len: usize) -> (_: usize, error: Error)
            // There are no pointers to pointers
            // so we can consume the arguments
            // straight away.
            //
            // We return no value (0) and no
            // error.
            let b = unsafe { core::slice::from_raw_parts_mut(arg1 as *mut u8, arg2) };
            random::read(b);

            let value = 0;
            let error = Error::NoError as usize;

            SyscallResults { value, error }
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
#[allow(clippy::missing_panics_doc)] // Will only panic if kernel configuration is broken.
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
    LStar::write(x86_64::VirtAddr::from_ptr(syscall_entry as *const u8));

    // Mask off interrupts so that interrupts are
    // disabled when the interrupt handler starts.
    SFMask::write(RFlags::INTERRUPT_FLAG);

    // Enable the SYSCALL and SYSRET instructions.
    unsafe {
        Efer::update(|flags| *flags |= EferFlags::SYSTEM_CALL_EXTENSIONS);
    }
}
