// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

#![no_std]
#![allow(clippy::float_arithmetic)] // Allowed in userspace.
#![deny(clippy::inline_asm_x86_att_syntax)]
#![allow(clippy::missing_panics_doc)]
#![allow(clippy::panic)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(deprecated_in_future)]
#![deny(keyword_idents)]
#![deny(macro_use_extern_crate)]
#![deny(missing_abi)]
#![allow(unsafe_code)]
#![deny(unused_crate_dependencies)]

use core::arch::asm;
use firefly::println;
use firefly_abi::{Error, Registers, Syscalls};
use firefly_syscalls::shutdown;

/// The application entry point.
///
/// # Panics
///
/// `main` will panic if any of its checks
/// fails.
///
#[inline]
pub fn main() {
    let rsp: u64;
    unsafe { asm!("mov {:r}, rsp", out(reg) rsp) };
    println!("Stack pointer: {:p}.", rsp as usize as *const ());

    check_abi_registers();
    println!("PASS: debug_abi_registers");

    check_abi_errors();
    println!("PASS: debug_abi_errors");

    check_abi_bounds();
    println!("PASS: debug_abi_bounds");

    println!("PASS");
    shutdown().expect("failed to request shutdown");
}

/// Check that the kernel sees all general-purpose
/// registers the same way that we do in userspace,
/// using debug_abi_registers.
///
fn check_abi_registers() {
    let mut got = Registers {
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

    let sent = Registers {
        // Use bit patterns that are unlikely
        // to be mistaken for one another if
        // bits are copied across by mistake.
        rax: Syscalls::DebugAbiRegisters.as_u64(),
        rbx: 0, // RBX is used internally by LLVM and cannot be overridden.
        rcx: 0, // RCX is destroyed.
        rdx: 0x1032_5476_98ba_dcfe_u64,
        rsi: 0x0011_2233_4455_6677_u64,
        rdi: (&mut got) as *mut Registers as usize as u64,
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

macro_rules! test {
    // Typed syscall wrapper.
    ($syscall:ident($($args:expr),+), $want:expr) => {
        assert_eq!(
            firefly_syscalls::$syscall($($args),+),
            $want
        );
    };

    // Raw syscall.
    ($sys:ident $syscall:ident($($args:expr),+), $want:ident) => {
        assert_eq!(
            unsafe {
                firefly_syscalls::$sys(
                    Syscalls::$syscall.as_u64(),
                    $($args as u64),+
                )
            },
            (0u64, $want.as_u64())
        );
    };
}

/// Check that userspace and the kernel agree
/// on how to handle syscalls that only return
/// an error.
///
fn check_abi_errors() {
    test!(debug_abi_errors(Error::NoError), Ok(()));
    test!(debug_abi_errors(Error::BadSyscall), Err(Error::BadSyscall));
    test!(
        debug_abi_errors(Error::IllegalArg1),
        Err(Error::IllegalArg1)
    );

    // Check the kernel safely handles a non-existant syscall.
    assert_eq!(
        unsafe { firefly_syscalls::syscall0(0xffff_ffff_ffff_ffff_u64) },
        (0u64, Error::BadSyscall.as_u64())
    );
}

/// Check that the syscall handler performs
/// bounds checks on signed integers, unsigned
/// integers, enumerations, and pointers.
///
fn check_abi_bounds() {
    let ok = Error::NoError;
    let err1 = Error::IllegalArg1;
    let err2 = Error::IllegalArg2;
    let err3 = Error::IllegalArg3;
    let err4 = Error::IllegalArg4;
    let err5 = Error::IllegalArg5;
    let err6 = Error::IllegalArg6;
    const BYTE: u8 = 1;
    let ptr = &BYTE as *const u8;
    let null = core::ptr::null::<u8>();
    let noncanonical = 0x8000_0000_0000_usize as *const u8;
    let kernelspace = 0xffff_ffff_ffff_ffff_usize as *const u8;

    // Signed integer.
    test!(debug_abi_bounds(-128, 0, Error::NoError, ptr), Ok(()));
    test!(debug_abi_bounds(0, 0, Error::NoError, ptr), Ok(()));
    test!(debug_abi_bounds(127, 0, Error::NoError, ptr), Ok(()));
    test!(syscall4 DebugAbiBounds(-129i16, 0u8, Error::NoError.as_u64(), ptr), err1);
    test!(syscall4 DebugAbiBounds(128i16, 0u8, Error::NoError.as_u64(), ptr), err1);

    // Unsigned integer.
    test!(debug_abi_bounds(0, 0, Error::NoError, ptr), Ok(()));
    test!(debug_abi_bounds(0, 255, Error::NoError, ptr), Ok(()));
    test!(syscall4 DebugAbiBounds(0i16, 256u16, Error::NoError.as_u64(), ptr), err2);

    // Enumeration.
    test!(debug_abi_bounds(0, 0, Error::NoError, ptr), Ok(()));
    test!(debug_abi_bounds(0, 0, Error::IllegalArg1, ptr), Ok(()));
    test!(syscall4 DebugAbiBounds(0i16, 0u16, 0xffff_ffff_ffff_ffff_u64, ptr), err3);

    // Pointer.
    test!(debug_abi_bounds(0, 0, Error::NoError, ptr), Ok(()));
    test!(debug_abi_bounds(0, 0, Error::NoError, null), Err(err4));
    test!(
        debug_abi_bounds(0, 0, Error::NoError, noncanonical),
        Err(err4)
    );
    test!(
        debug_abi_bounds(0, 0, Error::NoError, kernelspace),
        Err(err4)
    );

    // Check that non-zero values in unused arguments are rejected.
    test!(syscall6 DebugAbiBounds(0i16, 0u16, Error::NoError, ptr, 0u64, 0u64), ok);
    test!(syscall6 DebugAbiBounds(0i16, 0u16, Error::NoError, ptr, 1u64, 0u64), err5);
    test!(syscall6 DebugAbiBounds(0i16, 0u16, Error::NoError, ptr, 0u64, 1u64), err6);
}
