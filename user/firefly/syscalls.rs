// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides helper functions for calling Firefly syscalls.

use core::arch::asm;

/// Exit the current thread immediately.
///
#[inline]
pub fn exit_thread() -> ! {
    unsafe {
        // Call exit_thread (syscall 0).
        asm! {
            "xor rax, rax",
            "syscall",
        }
    };

    unreachable!();
}
