// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Analyses the CPU for supported features and branding.

#![no_std]

use raw_cpuid::CpuId;
use serial::println;

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
