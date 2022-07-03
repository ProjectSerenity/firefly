// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Analyses the CPU for supported features and branding.

#![no_std]
#![deny(clippy::float_arithmetic)]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![allow(clippy::panic)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(deprecated_in_future)]
#![deny(unused_crate_dependencies)]
#![allow(unsafe_code)]
#![feature(asm_const)]

mod local;

use core::arch::asm;
use core::sync::atomic::{AtomicUsize, Ordering};
use raw_cpuid::CpuId;
use serial::println;
use x86_64::registers::control::{Cr4, Cr4Flags};
use x86_64::registers::rflags;
use x86_64::registers::rflags::RFlags;

pub use local::{
    global_init, id, per_cpu_init, set_syscall_stack_pointer, set_user_stack_pointer,
    syscall_stack_pointer, user_stack_pointer,
};

/// This stores the maximum number of logical cores.
///
/// The value is not modified once initialised by [`global_init`].
///
static MAX_CORES: AtomicUsize = AtomicUsize::new(1);

/// Returns the maximum number of logical cores on this
/// machine.
///
/// This value should be used to ensure data local to
/// each CPU has sufficient entries for all values returned
/// by [`id`].
///
pub fn max_cores() -> usize {
    MAX_CORES.load(Ordering::Relaxed)
}

/// SMAP tracks whether Supervisor Mode Access Prevention
/// is supported by the CPU and has thus been enabled. It
/// will not change after check_features has returned for
/// the first time.
///
static mut SMAP: bool = false;

/// Checks that the CPU supports all the features we need.
///
/// # Panics
///
/// `check_features` will panic if the CPU does not support
/// any features Firefly requires.
///
pub fn check_features() {
    let cpuid = CpuId::new();
    match cpuid.get_feature_info() {
        None => panic!("unable to determine CPU features"),
        Some(features) => {
            if !features.has_msr() {
                panic!("CPU does not support model-specific registers");
            }
        }
    }

    match cpuid.get_extended_feature_info() {
        None => panic!("unable to determine CPU features"),
        Some(features) => {
            if features.has_smap() {
                // Enable SMAP, which prevents access to user
                // memory in ring 3 page mappings while running
                // as the kernel.
                unsafe {
                    Cr4::update(|flags| *flags |= Cr4Flags::SUPERVISOR_MODE_ACCESS_PREVENTION);
                    SMAP = true;
                }
                println!("Enabled SMAP.");
            }
            if features.has_smep() {
                // Enable SMEP, which prevents the execution of
                // usermode code while running as the kernel.
                unsafe {
                    Cr4::update(|flags| *flags |= Cr4Flags::SUPERVISOR_MODE_EXECUTION_PROTECTION)
                };
                println!("Enabled SMEP.");
            }
        }
    }

    match cpuid.get_extended_processor_and_feature_identifiers() {
        None => panic!("unable to determine CPU features"),
        Some(features) => {
            if !features.has_syscall_sysret() {
                panic!("CPU does not support the syscall/sysret instructions");
            }
        }
    }
}

/// Prints the CPU's branding information.
///
pub fn print_branding() {
    let cpuid = CpuId::new();
    if let Some(branding) = cpuid.get_processor_brand_string() {
        println!("Kernel running on {} CPU.", branding.as_str());
    } else if let Some(version) = cpuid.get_vendor_info() {
        println!("Kernel running on {} CPU.", version.as_str());
    } else {
        println!("Kernel running on unknown CPU.");
    }
}

/// The Access Control flag in the RFLAGS. Note
/// that this has the same value as the Alignment
/// Check flag. See the x86_64 manual, volume 3A,
/// section 2.3.
///
const ACCESS_CONTROL: RFlags = RFlags::ALIGNMENT_CHECK;

/// Enables access to user memory, as described
/// in [`with_user_memory_access`].
///
#[inline]
pub fn enable_user_memory_access() {
    unsafe {
        if SMAP {
            asm!("stac");
        }
    }
}

/// Disables access to user memory, as described
/// in [`with_user_memory_access`].
///
#[inline]
pub fn disable_user_memory_access() {
    unsafe {
        if SMAP {
            asm!("clac");
        }
    }
}

/// Provides temporary access to user memory via
/// user page mappings when SMAP is enabled. See
/// the Intel x86_64 manual, volume 3A, sections
/// 4.6.1 and 2.3.
///
/// If SMAP is not supported by the CPU, this has
/// no effect.
///
/// ```
/// // Access is disallowed if SMAP is enabled.
/// with_user_memory_access(|| {
///     // Access is allowed.
///     with_user_memory_access(|| {
///         // Access is allowed.
///     });
///     // Access is still allowed.
/// });
/// // Access is disallowed again if SMAP is enabled.
/// ```
///
#[inline]
pub fn with_user_memory_access<F, R>(f: F) -> R
where
    F: FnOnce() -> R,
{
    // Determine whether access is currently allowed.
    let allowed = rflags::read().contains(ACCESS_CONTROL);
    if !allowed {
        enable_user_memory_access();
    }

    // Do `f` while access is allowed.
    let ret = f();

    // Remove access if we did not have it initially.
    if !allowed {
        disable_user_memory_access();
    }

    // Return the result of `f`.
    ret
}
