// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Analyses the CPU for supported features and branding.

use lazy_static::lazy_static;
use raw_cpuid::CpuId;
use serial::println;

lazy_static! {
    static ref CPU_ID: CpuId = CpuId::new();
}

/// Checks that the CPU supports all the features we need.
///
/// # Panics
///
/// `init` will panic if the CPU does not support any features
/// Firefly requires.
///
pub fn init() {
    match CPU_ID.get_extended_processor_and_feature_identifiers() {
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
    if let Some(branding) = CPU_ID.get_processor_brand_string() {
        println!("Kernel running on {} CPU.", branding.as_str());
    } else if let Some(version) = CPU_ID.get_vendor_info() {
        println!("Kernel running on {} CPU.", version.as_str());
    } else {
        println!("Kernel running on unknown CPU.");
    }
}
