// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements a simple round robin scheduler for threads.
//!
//! ## Initialisation
//!
//! The scheduler will not activate and start preempting the running thread
//! until the kernel's initial thread calls [`start`], at which point the
//! scheduler takes ownership of the flow of execution.
//!
//! ## Thread switching
//!
//! [`switch`] can be called to proactively switch to the next available
//! thread in the scheduler.

pub mod timers;

use crate::switch::switch_stack;
use crate::thread::{current_thread, Thread, ThreadId, ThreadState};
use crate::{CURRENT_THREADS, IDLE_THREADS, SCHEDULER, THREADS};
use alloc::collections::VecDeque;
use alloc::sync::Arc;
use core::arch::asm;
use core::sync::atomic::{AtomicBool, Ordering};
use segmentation::with_segment_data;
use spin::Mutex;
use time::{after, Duration};
use x86_64::instructions::interrupts;

/// Scheduler is a basic thread scheduler.
///
/// Currently, it implements a round-robin algorithm.
///
pub(super) struct Scheduler {
    runnable: Mutex<VecDeque<ThreadId>>,
}

impl Scheduler {
    pub fn new() -> Scheduler {
        Scheduler {
            runnable: Mutex::new(VecDeque::new()),
        }
    }

    /// add queues a thread onto the runnable queue.
    ///
    pub fn add(&self, thread: ThreadId) {
        self.runnable.lock().push_back(thread);
    }

    /// next returns the next thread able to run.
    ///
    /// The thread is removed from the runnable queue,
    /// so it must be added again afterwards if still
    /// able to run.
    ///
    pub fn next(&self) -> Option<ThreadId> {
        self.runnable.lock().pop_front()
    }

    /// remove removes the thread from the queue.
    ///
    pub fn remove(&self, thread: ThreadId) {
        self.runnable.lock().retain(|id| *id != thread);
    }
}

/// Tracks whether the scheduler has been activated.
/// It is set in [`start`] and can be checked with
/// [`ready`].
///
static INITIALISED: AtomicBool = AtomicBool::new(false);

/// Implements the idle thread.
///
/// We fall back to this if the kernel has no other
/// work left to do.
///
fn idle_loop() -> ! {
    loop {
        x86_64::instructions::interrupts::enable_and_hlt();
    }
}

/// Hands control over to the scheduler.
///
/// This lets the idle thread take control of the kernel's
/// initial state and lets the scheduler take ownership of
/// the flow of execution.
///
/// Note that newly created threads will not be started and
/// the kernel's initial thread will not be preempted until
/// `start` is called.
///
pub fn start() -> ! {
    // Mark the scheduler as in control.
    INITIALISED.store(true, Ordering::Relaxed);

    // Hand over to the scheduler.
    switch();

    // We're now executing as the idle thread.
    idle_loop();
}

/// Returns whether the scheduler has been activated and owns
/// the flow of execution.
///
pub fn ready() -> bool {
    INITIALISED.load(Ordering::Relaxed)
}

/// Check whether the currently executing thread's time slice
/// has been exhausted. If it has, we refill it and switch to
/// the next runnable thread. If The current thread still has
/// time remaining, `preempt` returns.
///
pub fn preempt() {
    // Time to pre-empt the current thread.
    let current_thread = current_thread();
    if !current_thread.tick() {
        return;
    }

    let global_thread_id = current_thread.global_thread_id();
    if global_thread_id != ThreadId::IDLE {
        current_thread.reset_time_slice();
    }

    // Drop our reference to the current thread,
    // so the scheduler has full control.
    drop(current_thread);

    switch();
}

/// Sleep the current thread for the given `duration`.
///
pub fn sleep(duration: Duration) {
    let stop = after(duration);
    let current = current_thread();

    // Check we haven't already been prevented
    // from sleeping.
    if current.thread_state() == ThreadState::Insomniac {
        current.set_state(ThreadState::Runnable);
        return;
    }

    // Create a timer to wake us up.
    timers::add(current.global_thread_id(), stop);

    // Put ourselves to sleep.
    current.set_state(ThreadState::Sleeping);
    drop(current);

    // Switch to the next thread.
    switch();
}

/// Set the currently executing thread.
///
fn set_current_thread(thread: Arc<Thread>) {
    // Save the current thread's user stack pointer into the current thread.
    current_thread().set_user_stack(cpu::user_stack_pointer());

    // Overwrite the state from the new thread.
    let interrupt_stack = thread.interrupt_stack();
    with_segment_data(|data| data.set_interrupt_stack(interrupt_stack));
    cpu::set_syscall_stack_pointer(thread.syscall_stack());
    cpu::set_user_stack_pointer(thread.user_stack());
    CURRENT_THREADS.lock()[cpu::id()] = thread;
}

/// Returns a copy of the idle thread for this CPU.
///
fn idle_thread() -> Arc<Thread> {
    IDLE_THREADS.lock()[cpu::id()].clone()
}

/// Schedules out the current thread and switches to the next
/// runnable thread.
///
/// If no other threads are ready to run, `switch` may return
/// immediately. Calling `switch` does not modify the current
/// thread's time slice.
///
pub fn switch() {
    let restart_interrupts = interrupts::are_enabled();
    interrupts::disable();
    let current = current_thread();
    let next = {
        let scheduler = SCHEDULER.lock();

        // Add the current thread to the runnable
        // queue, unless it's the idle thread, which
        // always has thread id 0 (which is otherwise
        // invalid).
        let current_thread_id = current.global_thread_id();
        if current_thread_id != ThreadId::IDLE && current.thread_state() == ThreadState::Runnable {
            scheduler.add(current_thread_id);
        }

        match scheduler.next() {
            Some(thread) => THREADS.lock().get(&thread).unwrap().clone(),
            None => idle_thread(),
        }
    };

    if Arc::ptr_eq(&current, &next) {
        // We're already running the right
        // thread, so return without doing
        // anything further.
        if restart_interrupts {
            interrupts::enable();
        }

        return;
    }

    // Retrieve a pointer to each stack pointer. These point
    // to the value in the Thread structure, where we keep a
    // copy of the current stack pointer.
    let current_stack_pointer = current.stack_pointer();
    let new_stack_pointer = next.stack_pointer();

    // Switch into the next thread and re-enable interrupts.
    set_current_thread(next);

    // Note that once we have multiple CPUs, there will be
    // an unsafe gap between when we schedule the current
    // thread in the block that assigns next and when we
    // call switch_stack below. In this gap, the saved state
    // in the thread structure will be stale, as it was last
    // updated when the current thread last started. If
    // another CPU schedules the current thread in this gap,
    // it will switch to the stale state, and do so while
    // we're using it here. We'll need to work out how to
    // stop that happening before adding support for multiple
    // CPUs.

    // We can now drop our reference to the current thread
    // If the current thread is not exiting, then there
    // will be one handle to it in THREADS. The next thread
    // will have one handle in THREADS and another now in
    // our CPU-local data. The reason we want to do this here
    // is that if the current thread is exiting, we will
    // never return from switch_stack.
    //
    // We don't drop the next thread, as we've already
    // moved our reference to it when we called
    // set_current_thread(next).
    //
    // We drop the current thread, even when it is exiting.
    // This means the thread's stack will be freed, so there
    // is a slight risk another thread on another CPU will
    // be given our stack. As a result, we use a jump, rather
    // than a function call, to avoid using our stack.
    if current.thread_state() == ThreadState::Exiting {
        debug_assert!(Arc::strong_count(&current) == 1);
        drop(current);
        unsafe {
            asm!(
                "mov rdi, {0}",
                "jmp replace_stack",
                in(reg) new_stack_pointer,
                options(nostack, nomem, preserves_flags)
            );
        }
    } else {
        drop(current);
        unsafe { switch_stack(current_stack_pointer, new_stack_pointer) };
    }
}

/// Marks the given thread as runnable, allowing it to
/// run.
///
/// `resume` returns whether the thread still exists and
/// is now runnable.
///
pub fn resume(thread_id: ThreadId) -> bool {
    match THREADS.lock().get(&thread_id) {
        None => false,
        Some(thread) => match thread.thread_state() {
            ThreadState::BeingCreated => {
                thread.set_state(ThreadState::Runnable);
                SCHEDULER.lock().add(thread_id);
                true
            }
            ThreadState::Runnable => true,
            ThreadState::Drowsy => {
                thread.set_state(ThreadState::Insomniac);
                true
            }
            ThreadState::Insomniac => true,
            ThreadState::Sleeping => {
                thread.set_state(ThreadState::Runnable);
                SCHEDULER.lock().add(thread_id);
                true
            }
            ThreadState::Exiting => false,
        },
    }
}
