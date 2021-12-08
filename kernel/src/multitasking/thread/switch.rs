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

    /// replace_stack replaces the current stack with a
    /// new stack, using the System V ABI. Its last action
    /// is to start executing a new thread.
    ///
    /// replace_stack takes a pointer to the new thread's
    /// saved stack pointer.
    ///
    pub fn replace_stack(new_stack_pointer: *const u64);

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