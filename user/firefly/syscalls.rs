// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides helper functions for calling Firefly syscalls.

use core::arch::asm;
use syscalls::{Error, Syscall};

/// Call a raw syscall.
///
/// # Safety
///
/// This makes a raw syscall with unknown parameters,
/// so it may have unsafe effects.
///
#[inline]
pub unsafe fn syscall0(sys: usize) -> (usize, usize) {
    let value: usize;
    let error: usize;
    asm! {
        "syscall",
        inlateout("rax") sys => value,
        lateout("rdx") error,
        out("rcx") _,
        out("r11") _,
    }

    (value, error)
}

/// Call a raw syscall.
///
/// # Safety
///
/// This makes a raw syscall with unknown parameters,
/// so it may have unsafe effects.
///
#[inline]
pub unsafe fn syscall1(sys: usize, arg1: usize) -> (usize, usize) {
    let value: usize;
    let error: usize;
    asm! {
        "syscall",
        inlateout("rax") sys => value,
        in("rdi") arg1,
        lateout("rdx") error,
        out("rcx") _,
        out("r11") _,
    }

    (value, error)
}

/// Call a raw syscall.
///
/// # Safety
///
/// This makes a raw syscall with unknown parameters,
/// so it may have unsafe effects.
///
#[inline]
pub unsafe fn syscall2(sys: usize, arg1: usize, arg2: usize) -> (usize, usize) {
    let value: usize;
    let error: usize;
    asm! {
        "syscall",
        inlateout("rax") sys => value,
        in("rdi") arg1,
        in("rsi") arg2,
        lateout("rdx") error,
        out("rcx") _,
        out("r11") _,
    }

    (value, error)
}

/// Call a raw syscall.
///
/// # Safety
///
/// This makes a raw syscall with unknown parameters,
/// so it may have unsafe effects.
///
#[inline]
pub unsafe fn syscall3(sys: usize, arg1: usize, arg2: usize, arg3: usize) -> (usize, usize) {
    let value: usize;
    let error: usize;
    asm! {
        "syscall",
        inlateout("rax") sys => value,
        in("rdi") arg1,
        in("rsi") arg2,
        inlateout("rdx") arg3 => error,
        out("rcx") _,
        out("r11") _,
    }

    (value, error)
}

/// Call a raw syscall.
///
/// # Safety
///
/// This makes a raw syscall with unknown parameters,
/// so it may have unsafe effects.
///
#[inline]
pub unsafe fn syscall4(
    sys: usize,
    arg1: usize,
    arg2: usize,
    arg3: usize,
    arg4: usize,
) -> (usize, usize) {
    let value: usize;
    let error: usize;
    asm! {
        "syscall",
        inlateout("rax") sys => value,
        in("rdi") arg1,
        in("rsi") arg2,
        inlateout("rdx") arg3 => error,
        in("r10") arg4,
        out("rcx") _,
        out("r11") _,
    }

    (value, error)
}

/// Call a raw syscall.
///
/// # Safety
///
/// This makes a raw syscall with unknown parameters,
/// so it may have unsafe effects.
///
#[inline]
pub unsafe fn syscall5(
    sys: usize,
    arg1: usize,
    arg2: usize,
    arg3: usize,
    arg4: usize,
    arg5: usize,
) -> (usize, usize) {
    let value: usize;
    let error: usize;
    asm! {
        "syscall",
        inlateout("rax") sys => value,
        in("rdi") arg1,
        in("rsi") arg2,
        inlateout("rdx") arg3 => error,
        in("r10") arg4,
        in("r8") arg5,
        out("rcx") _,
        out("r11") _,
    }

    (value, error)
}

/// Call a raw syscall.
///
/// # Safety
///
/// This makes a raw syscall with unknown parameters,
/// so it may have unsafe effects.
///
#[inline]
pub unsafe fn syscall6(
    sys: usize,
    arg1: usize,
    arg2: usize,
    arg3: usize,
    arg4: usize,
    arg5: usize,
    arg6: usize,
) -> (usize, usize) {
    let value: usize;
    let error: usize;
    asm! {
        "syscall",
        inlateout("rax") sys => value,
        in("rdi") arg1,
        in("rsi") arg2,
        inlateout("rdx") arg3 => error,
        in("r10") arg4,
        in("r8") arg5,
        in("r9") arg6,
        out("rcx") _,
        out("r11") _,
    }

    (value, error)
}

/// Exit the current thread immediately.
///
#[inline]
pub fn exit_thread() -> ! {
    unsafe { syscall0(Syscall::ExitThread as usize) };

    unreachable!();
}

/// Print a message to the process's
/// standard output.
///
#[inline]
pub fn print_message(s: &str) -> Result<usize, Error> {
    let sys = Syscall::PrintMessage as usize;
    let (value, error) = unsafe { syscall2(sys, s.as_ptr() as usize, s.len()) };
    match Error::from_usize(error) {
        Some(Error::NoError) => Ok(value),
        Some(err) => Err(err),
        None => Err(Error::BadSyscall),
    }
}

/// Print a message to the process's
/// standard error output.
///
#[inline]
pub fn print_error(s: &str) -> Result<usize, Error> {
    let sys = Syscall::PrintError as usize;
    let (value, error) = unsafe { syscall2(sys, s.as_ptr() as usize, s.len()) };
    match Error::from_usize(error) {
        Some(Error::NoError) => Ok(value),
        Some(err) => Err(err),
        None => Err(Error::BadSyscall),
    }
}
