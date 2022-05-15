// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

#![no_std]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![allow(clippy::missing_panics_doc)]
#![allow(clippy::panic)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(unused_crate_dependencies)]
#![allow(unsafe_code)]

use core::arch::asm;
use firefly::syscalls::{debug_abi_errors, debug_abi_range, Error, Syscalls};
use firefly::{println, syscalls};

/// The application entry point.
///
/// # Panics
///
/// `main` will panic if any of its checks
/// fails.
///
#[inline]
pub fn main() {
    check_abi_registers();
    println!("PASS: debug_abi_registers");

    check_abi_errors();
    println!("PASS: debug_abi_errors");

    check_abi_range();
    println!("PASS: debug_abi_range");

    println!("PASS");
}

/// Check that the kernel sees all general-purpose
/// registers the same way that we do in userspace,
/// using debug_abi_registers.
///
fn check_abi_registers() {
    let mut got = syscalls::Registers {
        rax: 0,
        rbx: 0,
        rcx: 0,
        rdx: 0,
        rsi: 0,
        rdi: 0,
        rbp: 0,
        rip: 0,
        rsp: 0,
        r8: 0,
        r9: 0,
        r10: 0,
        r11: 0,
        r12: 0,
        r13: 0,
        r14: 0,
        r15: 0,
        rflags: 0,
    };

    let sent = syscalls::Registers {
        // Use bit patterns that are unlikely
        // to be mistaken for one another if
        // bits are copied across by mistake.
        rax: Syscalls::DebugAbiRegisters.as_u64(),
        rbx: 0, // RBX is used internally by LLVM and cannot be overridden.
        rcx: 0, // RCX is destroyed.
        rdx: 0x1032_5476_98ba_dcfe_u64,
        rsi: 0x0011_2233_4455_6677_u64,
        rdi: (&mut got) as *mut syscalls::Registers as usize as u64,
        rbp: 0, // We calculate this later, which is easier than predicting it exactly.
        rip: 0, // We calculate this later, which is easier than predicting it exactly.
        rsp: 0, // We calculate this later, which is easier than predicting it exactly.
        r8: 0x2041_6385_a7c9_ebfd_u64,
        r9: 0x1357_9bdf_0246_8ace_u64,
        r10: 0xfdb9_7531_eca8_6420_u64,
        r11: 0, // R11 is destroyed.
        r12: 0xfbd9_7351_eac8_6240_u64,
        r13: 0x0819_2a3b_4c5d_6e7f_u64,
        r14: 0xf7e6_d5c4_b3a2_9180_u64,
        r15: 0x0f1e_2d3c_4b5a_6978_u64,
        rflags: 0x8796_a5b4_c3d2_e1f0_u64,
    };

    let result: u64;
    unsafe {
        asm! {
            "syscall",
            inlateout("rax") sent.rax => result,
            // Skip RBX.
            inlateout("rcx") sent.rcx => _,
            inlateout("rdx") sent.rdx => _,
            in("rsi") sent.rsi,
            in("rdi") sent.rdi,
            // Skip RBP.
            // Skip RIP.
            // Skip RSP.
            in("r8") sent.r8,
            in("r9") sent.r9,
            in("r10") sent.r10,
            inlateout("r11") sent.r11 => _,
            in("r12") sent.r12,
            in("r13") sent.r13,
            in("r14") sent.r14,
            in("r15") sent.r15,
        }
    }

    // Check the error code.
    match Error::from_u64(result) {
        Some(Error::NoError) => {}
        Some(err) => panic!("debug_abi_registers: got {:?}", err),
        None => panic!("debug_abi_registers: got invalid error code {}", result),
    }

    // Check the individual saved
    // registers. Since the registers
    // structure is packed, we can't
    // just use assert_eq! on the
    // fields directly, so we have to
    // copy the values out first.

    let grax = got.rax;
    // Skip RBX.
    // Skip RCX.
    let grdx = got.rdx;
    let grsi = got.rsi;
    let grdi = got.rdi;
    // Skip RBP.
    // Skip RIP.
    // Skip RSP.
    let gr8 = got.r8;
    let gr9 = got.r9;
    let gr10 = got.r10;
    // Skip R11.
    let gr12 = got.r12;
    let gr13 = got.r13;
    let gr14 = got.r14;
    let gr15 = got.r15;

    let srax = sent.rax;
    // Skip RBX.
    // Skip RCX.
    let srdx = sent.rdx;
    let srsi = sent.rsi;
    let srdi = sent.rdi;
    // Skip RBP.
    // Skip RIP.
    // Skip RSP.
    let sr8 = sent.r8;
    let sr9 = sent.r9;
    let sr10 = sent.r10;
    // Skip R11.
    let sr12 = sent.r12;
    let sr13 = sent.r13;
    let sr14 = sent.r14;
    let sr15 = sent.r15;

    assert_eq!(grax, srax, "RAX");
    // Skip RBX, as LLVM controls the value.
    // Skip RCX, as the kernel never sees it.
    assert_eq!(grdx, srdx, "RDX");
    assert_eq!(grsi, srsi, "RSI");
    assert_eq!(grdi, srdi, "RDI");
    // We skip the pointer registers,
    // as userspace will break rapidly
    // if they're not correct and it's
    // very fiddly to predict the right
    // value.
    // Skip RBP.
    // Skip RIP.
    // Skip RSP.
    assert_eq!(gr8, sr8, "R8");
    assert_eq!(gr9, sr9, "R9");
    assert_eq!(gr10, sr10, "R10");
    // Skip R11, as the kernel never sees it.
    assert_eq!(gr12, sr12, "R12");
    assert_eq!(gr13, sr13, "R13");
    assert_eq!(gr14, sr14, "R14");
    assert_eq!(gr15, sr15, "R15");
}

/// Check that userspace and the kernel agree
/// on how to handle syscalls that only return
/// an error.
///
fn check_abi_errors() {
    assert_eq!(debug_abi_errors(Error::NoError), Error::NoError);
    assert_eq!(debug_abi_errors(Error::BadSyscall), Error::BadSyscall);
    assert_eq!(
        debug_abi_errors(Error::IllegalParameter),
        Error::IllegalParameter
    );
}

/// Check that the syscall handler performs
/// bounds checks on signed integers, unsigned
/// integers, and enumerations.
///
fn check_abi_range() {
    // Signed integer.
    assert_eq!(debug_abi_range(-128, 0, Error::NoError), Error::NoError);
    assert_eq!(debug_abi_range(0, 0, Error::NoError), Error::NoError);
    assert_eq!(debug_abi_range(127, 0, Error::NoError), Error::NoError);
    assert_eq!(
        unsafe {
            syscalls::syscall3(
                Syscalls::DebugAbiRange.as_u64(),
                -129i16 as u64,
                0u8 as u64,
                Error::NoError.as_u64(),
            )
        },
        (0u64, Error::IllegalParameter.as_u64())
    );
    assert_eq!(
        unsafe {
            syscalls::syscall3(
                Syscalls::DebugAbiRange.as_u64(),
                128i16 as u64,
                0u8 as u64,
                Error::NoError.as_u64(),
            )
        },
        (0u64, Error::IllegalParameter.as_u64())
    );

    // Unsigned integer.
    assert_eq!(debug_abi_range(0, 0, Error::NoError), Error::NoError);
    assert_eq!(debug_abi_range(0, 255, Error::NoError), Error::NoError);
    assert_eq!(
        unsafe {
            syscalls::syscall3(
                Syscalls::DebugAbiRange.as_u64(),
                0i16 as u64,
                256u16 as u64,
                Error::NoError.as_u64(),
            )
        },
        (0u64, Error::IllegalParameter.as_u64())
    );

    // Enumeration.
    assert_eq!(debug_abi_range(0, 0, Error::NoError), Error::NoError);
    assert_eq!(
        debug_abi_range(0, 0, Error::IllegalParameter),
        Error::NoError
    );
    assert_eq!(
        unsafe {
            syscalls::syscall3(
                Syscalls::DebugAbiRange.as_u64(),
                0i16 as u64,
                0u16 as u64,
                0xffff_ffff_ffff_ffff_u64,
            )
        },
        (0u64, Error::IllegalParameter.as_u64())
    );
}