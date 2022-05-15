// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements the kernel's syscalls, allowing user processes to access kernel functionality.

#[allow(clippy::enum_variant_names)]
mod abi {
    include!(env!("SYSCALLS_RS"));
}

use self::abi::{Error, Registers, SavedRegisters, SyscallABI};
use core::arch::global_asm;
use memory::constants::USERSPACE;
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

/// FireflyABI is a unit type that contains the
/// implementation for each syscall.
///
struct FireflyABI;

impl SyscallABI for FireflyABI {
    /// Called when an unsupported syscall is received.
    ///
    #[inline]
    fn bad_syscall(
        registers: *mut SavedRegisters,
        arg1: u64,
        arg2: u64,
        arg3: u64,
        arg4: u64,
        arg5: u64,
        arg6: u64,
        syscall_num: u64,
    ) -> Error {
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

        Error::BadSyscall
    }

    /// Exits the current thread, ceasing execution. `exit_thread`
    /// will not return.
    ///
    #[inline]
    fn exit_thread(_registers: *mut SavedRegisters) -> Error {
        println!("Exiting user thread.");
        thread::exit();
    }

    /// Allows diagnostics of the syscall ABI by userspace.
    /// The full set of registers received by the kernel is
    /// written to the registers structure passed to it.
    ///
    #[inline]
    fn debug_abi_registers(_registers: *mut SavedRegisters, registers: *mut Registers) -> Error {
        // Check that the pointer is in userspace.
        if let Ok(ptr) = VirtAddr::try_new(registers as usize) {
            if !USERSPACE.contains_addr(ptr) {
                return Error::IllegalParameter;
            }
        } else {
            return Error::IllegalParameter;
        }

        unsafe {
            let regs = *_registers;
            let rflags = regs.rflags;
            *registers = Registers {
                rax: regs.rax,
                rbx: regs.rbx,
                rcx: 0, // RCX is not preserved.
                rdx: regs.rdx,
                rsi: regs.rsi,
                rdi: regs.rdi,
                rbp: regs.rbp.as_usize() as u64,
                rip: regs.rip.as_usize() as u64,
                rsp: regs.rsp.as_usize() as u64,
                r8: regs.r8,
                r9: regs.r9,
                r10: regs.r10,
                r11: 0, // R11 is not preserved.
                r12: regs.r12,
                r13: regs.r13,
                r14: regs.r14,
                r15: regs.r15,
                rflags: rflags.bits(),
            };
        }

        Error::NoError
    }

    /// Allows diagnostics of the syscall ABI by userspace.
    /// The error passed to `debug_abi_errors` is returned
    /// as-is.
    ///
    #[inline]
    fn debug_abi_errors(_registers: *mut SavedRegisters, error: Error) -> Error {
        error
    }

    /// Allows diagnostics of the syscall ABI by userspace.
    /// The syscall checks that the passed parameter is in
    /// range and returns an [`Error`] accordingly.
    ///
    #[inline]
    fn debug_abi_range(
        _registers: *mut SavedRegisters,
        _signed_value: i8,
        _unsigned_value: u8,
        _error: Error,
        _pointer: *const u8,
    ) -> Error {
        Error::NoError
    }

    /// Prints a message to teh process's standard output.
    ///
    #[inline]
    fn print_message(
        _registers: *mut SavedRegisters,
        ptr: *const u8,
        size: u64,
    ) -> Result<u64, Error> {
        // Check that the pointer is in userspace.
        if let Ok(ptr) = VirtAddr::try_new(ptr as usize) {
            if !USERSPACE.contains_addr(ptr) {
                return Err(Error::IllegalParameter);
            }
        } else {
            return Err(Error::IllegalParameter);
        }

        // There are no pointers to pointers
        // so we can consume the arguments
        // straight away.
        //
        // It's a little tacky, but using
        // UNIX shell colours helps us to
        // differentiate user print_message
        // and print_error from the kernel's
        // println.
        let b = unsafe { core::slice::from_raw_parts(ptr, size as usize) };
        if let Ok(s) = core::str::from_utf8(b) {
            print!("\x1b[1;32m"); // Green foreground.
            let written = if serial::write_str(s).is_err() {
                // We handle a failure to write the
                // message as having written nothing,
                // rather than returning an error.
                0
            } else {
                size
            };
            print!("\x1b[0m"); // Reset colour.

            Ok(written)
        } else {
            Err(Error::IllegalParameter)
        }
    }

    /// Prints an error message to the process's standard error
    /// output.
    ///
    #[inline]
    fn print_error(
        _registers: *mut SavedRegisters,
        ptr: *const u8,
        size: u64,
    ) -> Result<u64, Error> {
        // Check that the pointer is in userspace.
        if let Ok(ptr) = VirtAddr::try_new(ptr as usize) {
            if !USERSPACE.contains_addr(ptr) {
                return Err(Error::IllegalParameter);
            }
        } else {
            return Err(Error::IllegalParameter);
        }

        // There are no pointers to pointers
        // so we can consume the arguments
        // straight away.
        //
        // It's a little tacky, but using
        // UNIX shell colours helps us to
        // differentiate user print_message
        // and print_error from the kernel's
        // println.
        let b = unsafe { core::slice::from_raw_parts(ptr, size as usize) };
        if let Ok(s) = core::str::from_utf8(b) {
            print!("\x1b[1;31m"); // Red foreground.
            let written = if serial::write_str(s).is_err() {
                // We handle a failure to write the
                // message as having written nothing,
                // rather than returning an error.
                0
            } else {
                size
            };
            print!("\x1b[0m"); // Reset colour.

            Ok(written)
        } else {
            Err(Error::IllegalParameter)
        }
    }

    /// Read cryptographically-secure pseudorandom numbers into
    /// a memory buffer. `read_random` will always succeed and
    /// fill the entire buffer provided.
    ///
    #[inline]
    fn read_random(_registers: *mut SavedRegisters, ptr: *mut u8, size: u64) -> Error {
        // Check that the pointer is in userspace.
        if let Ok(ptr) = VirtAddr::try_new(ptr as usize) {
            if !USERSPACE.contains_addr(ptr) {
                return Error::IllegalParameter;
            }
        } else {
            return Error::IllegalParameter;
        }

        // There are no pointers to pointers
        // so we can consume the arguments
        // straight away.
        let b = unsafe { core::slice::from_raw_parts_mut(ptr, size as usize) };
        random::read(b);

        Error::NoError
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
