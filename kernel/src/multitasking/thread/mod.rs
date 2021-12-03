//! thread implements preemptive multitasking, using threads of execution.

use crate::memory::{free_kernel_stack, new_kernel_stack, StackBounds, VirtAddrRange};
use crate::multitasking::cpu_local::{current_thread, idle_thread, set_current_thread};
use crate::multitasking::thread::scheduler::Scheduler;
use crate::println;
use crate::utils::once::Once;
use alloc::collections::BTreeMap;
use alloc::sync::Arc;
use core::cell::UnsafeCell;
use core::sync::atomic::{AtomicU64, Ordering};
use crossbeam::atomic::AtomicCell;

mod scheduler;
mod switch;

type ThreadTable = BTreeMap<ThreadId, Arc<Thread>>;

/// THREADS stores all living threads, referencing them by
/// their thread id. Note that THREADS does not contain
/// the idle thread, as there will be a separate instance
/// for each CPU, but a single shared THREADS structure.
///
static THREADS: spin::Mutex<ThreadTable> = spin::Mutex::new(BTreeMap::new());

/// SCHEDULER is the thread scheduler.
///
static SCHEDULER: Once<spin::Mutex<Scheduler>> = Once::new();

/// idle_loop implements the idle thread. We
/// fall back to this if the kernel has no other
/// work left to do.
///
pub fn idle_loop() -> ! {
    println!("Kernel entering the idle thread.");
    loop {
        println!("Shutting down.");
        crate::shutdown_qemu();

        #[allow(unreachable_code)]
        {
            x86_64::instructions::interrupts::enable_and_hlt();
        }
    }
}

/// DEFAULT_RFLAGS contains the reserved bits of the
/// RFLAGS register so we can include them when we
/// build a new thread's initial stack.
///
/// Bit 1 is always set, as described in Figure 3-8
/// on page 78 of volume 1 of the Intel 64 manual.
///
const DEFAULT_RFLAGS: u64 = 0x2;

/// init prepares the thread scheduler.
///
pub fn init() {
    SCHEDULER.init(|| spin::Mutex::new(Scheduler::new()));
}

/// switch schedules out the current thread and switches to
/// the next runnable thread, which may be the current thread
/// again.
///
pub fn switch() {
    let current = current_thread().clone();
    let next = {
        let scheduler = SCHEDULER.lock();

        // Add the current thread to the runnable
        // queue, unless it's the idle thread, which
        // always has thread id 0 (which is otherwise
        // invalid).
        if current.id != ThreadId(0) && current.thread_state() == ThreadState::Runnable {
            scheduler.add(current.id);
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
        return;
    }

    // Retrieve a pointer to each stack pointer. These point
    // to the value in the Thread structure, where we keep a
    // copy of the current stack pointer.
    let current_stack_pointer = current.stack_pointer.get();
    let new_stack_pointer = next.stack_pointer.get();

    // Switch into the next thread.
    set_current_thread(next.clone());
    unsafe { switch::switch_stack(current_stack_pointer, new_stack_pointer) };
}

/// exit terminates the current thread and switches to
/// the next runnable thread.
///
pub fn exit() -> ! {
    let current = current_thread();
    if current.id == ThreadId(0) {
        panic!("idle thread tried to exit");
    }

    current.set_state(ThreadState::Exiting);
    THREADS.lock().remove(&current.id);
    if let Some(bounds) = current.stack_bounds {
        free_kernel_stack(bounds);
    }

    // We've now been unscheduled, so we
    // switch to the next thread.
    switch();
    unreachable!("Exited thread was re-scheduled somehow");
}

/// ThreadId uniquely identifies a thread.
///
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub struct ThreadId(u64);

impl ThreadId {
    /// new allocates and returns the next available
    /// ThreadId.
    ///
    fn new() -> Self {
        static NEXT_THREAD_ID: AtomicU64 = AtomicU64::new(1);
        ThreadId(NEXT_THREAD_ID.fetch_add(1, Ordering::Relaxed))
    }

    pub const fn as_u64(&self) -> u64 {
        self.0
    }
}

/// ThreadState describes the scheduling state
/// of a thread.
///
#[derive(Debug, Copy, Clone, Eq, PartialEq)]
pub enum ThreadState {
    /// The thread is runnable.
    Runnable,

    /// The thread is in the process
    /// of exiting.
    Exiting,
}

/// Thread contains the data necessary to contain
/// a thread of execution.
///
#[derive(Debug)]
pub struct Thread {
    id: ThreadId,
    state: AtomicCell<ThreadState>,
    stack_pointer: UnsafeCell<u64>,
    #[allow(dead_code)]
    stack_bounds: Option<StackBounds>,
}

/// push_stack is used to build a new thread's stack
/// without having to drop down to assembly. This is
/// done by dynamically populating the stack, value
/// by value. These are then popped off in switch_stack
/// when switching to the new thread for the first
/// time.
///
unsafe fn push_stack(mut rsp: *mut u64, value: u64) -> *mut u64 {
    // We move the stack pointer down by 8 bytes, write
    // the value to the new space, then return the updated
    // stack pointer.
    rsp = rsp.sub(1);
    rsp.write(value);
    rsp
}

// Thread is not thread-safe by default, as its
// stack pointer is stored in an UnsafeCell.
// However, as we only ever access this while
// the thread is running, which can only ever
// happen on one CPU at a time, this is actually
// thread-safe in practice. As a result, we tell
// Rust this is fine by implementing the Sync
// trait.
//
unsafe impl Sync for Thread {}

impl Thread {
    /// DEFAULT_KERNEL_STACK_PAGES is the number of pages
    /// that will be allocated for new kernel threads.
    ///
    const DEFAULT_KERNEL_STACK_PAGES: u64 = 128; // 128 4-KiB pages = 512 KiB.

    /// new_idle_thread creates a new kernel thread, to
    /// which we fall back if no other threads are runnable.
    ///
    pub fn new_idle_thread(stack_space: &VirtAddrRange) -> Arc<Thread> {
        // The idle thread always has thread id 0, which
        // is otherwise invalid.
        let id = ThreadId(0);

        // The initial stack pointer is 0, as the idle
        // thread inherits the kernel's initial stack.
        //
        // When we call switch for the first time, the
        // current stack pointer is written into the
        // idle thread. The 0 we set here is never read.
        let stack_pointer = UnsafeCell::new(0u64);

        // We inherit the kernel's initial stack, although
        // we never access any data previously stored
        // on the stack.
        let stack_bounds = Some(StackBounds::from(stack_space));

        let thread = Arc::new(Thread {
            id,
            state: AtomicCell::new(ThreadState::Runnable),
            stack_pointer,
            stack_bounds,
        });

        // Note that we don't store the idle thread
        // in THREADS, as it never enters the scheduler.
        // This means we only run the idle thread as a
        // last resort.

        thread
    }

    /// start_kernel_thread creates a new kernel thread,
    /// allocating a stack, and adding it to the scheduler.
    ///
    /// When the thread runs, it will start by enabling
    /// interrupts and calling entry_point.
    ///
    pub fn start_kernel_thread(entry_point: fn() -> !) {
        // Allocate and prepare the stack pointer.
        let stack = new_kernel_stack(Thread::DEFAULT_KERNEL_STACK_PAGES)
            .expect("failed to allocate stack for new kernel thread");

        let rsp = unsafe {
            let mut rsp: *mut u64 = stack.end().as_mut_ptr();

            // Push the entry point, to be called by
            // start_kernel_thread.
            rsp = push_stack(rsp, entry_point as u64);

            // Push start_kernel_thread and the initial
            // registers to be loaded by switch_stack.
            rsp = push_stack(rsp, switch::start_kernel_thread as *const u8 as u64); // RIP.
            rsp = push_stack(rsp, 0); // Initial RBP.
            rsp = push_stack(rsp, 0); // Initial RBX.
            rsp = push_stack(rsp, 0); // Initial R12.
            rsp = push_stack(rsp, 0); // Initial R13.
            rsp = push_stack(rsp, 0); // Initial R14.
            rsp = push_stack(rsp, 0); // Initial R15.
            rsp = push_stack(rsp, DEFAULT_RFLAGS); // RFLAGS (interrupts disabled).

            rsp
        };

        let id = ThreadId::new();
        let thread = Arc::new(Thread {
            id,
            state: AtomicCell::new(ThreadState::Runnable),
            stack_pointer: UnsafeCell::new(rsp as u64),
            stack_bounds: Some(stack),
        });

        THREADS.lock().insert(id, thread);
        SCHEDULER.lock().add(id);
    }

    /// thread_id returns the thread's ThreadId.
    ///
    pub fn thread_id(&self) -> ThreadId {
        self.id
    }

    /// thread_state returns the thread's current
    /// scheduling state.
    ///
    pub fn thread_state(&self) -> ThreadState {
        self.state.load()
    }

    /// set_state updates the thread's scheduling
    /// state.
    ///
    pub fn set_state(&self, new_state: ThreadState) {
        let scheduler = SCHEDULER.lock();
        self.state.store(new_state);
        match new_state {
            ThreadState::Runnable => {}
            ThreadState::Exiting => {
                scheduler.remove(self.id);
            }
        }
    }
}
