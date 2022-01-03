// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! switch contains the functionality to switch between threads.

global_asm!(include_str!("switch.s"));

// The following functions are implemented in switch.s.
//
extern "sysv64" {
    /// switch_stack replaces the current stack with a new
    /// stack, using the System V ABI. Its last action is
    /// to start executing the new thread.
    ///
    /// switch_stack takes a pointer to each thread's saved
    /// stack pointer.
    ///
    pub fn switch_stack(current_stack_pointer: *const u64, new_stack_pointer: *const u64);

    // Note that we have no Rust function declaration for
    // replace_stack, as we jump to it in inline assembly,
    // rather than calling it from Rust. This allows us to
    // avoid using the current stack at all, removing the
    // risk of memory corruption.

    /// start_kernel_thread should be used to start a new
    /// kernel thread by placing its address into the new
    /// thread's stack before calling switch_stack.
    ///
    /// start_kernel enables interrupts, then pops the
    /// thread's entry point from the stack and calls it.
    ///
    /// The entry point must never return, or an invalid
    /// instruction exception will be triggered.
    ///
    pub fn start_kernel_thread() -> !;
}
