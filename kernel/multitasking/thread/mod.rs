// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements preemptive multitasking, using independent threads of execution.
//!
//! This module allows the kernel to start an arbitrary number of threads,
//! preemptively scheduling between them. Threads can sleep, be resumed, and
//! exit as needed.
//!
//! ## Manipulating threads
//!
//! Threads can be created without being started using [`Thread::create_kernel_thread`].
//! This allows the caller to manipulate the thread's state (or make use of its
//! thread id) before then starting the thread with [`scheduler::resume`].
//! Alternatively, a thread can be started immediately by using [`Thread::start_kernel_thread`].
//!
//! A running thread may terminate its execution by calling [`exit`], or pause
//! its execution by calling [`scheduler::sleep`]. A sleeping
//! thread can be resumed early by calling [`crate::scheduler::resume`] (or [`KernelThreadId.resume`](KernelThreadId::resume),
//! but this will not cancel the timer that would have awoken it.
//!
//! Calling [`debug`] (or [`Thread::debug`]) will print debug info about the
//! thread.

mod stacks;

use crate::scheduler::timers;
use crate::switch::{start_kernel_thread, start_user_thread};
use crate::thread::stacks::{free_kernel_stack, new_kernel_stack, StackBounds};
use crate::{scheduler, CURRENT_THREADS, IDLE_THREADS, SCHEDULER, THREADS};
use alloc::sync::Arc;
use alloc::task::Wake;
use core::arch::asm;
use core::cell::UnsafeCell;
use core::sync::atomic::{AtomicU64, Ordering};
use core::task::Waker;
use memlayout::{VirtAddrRange, KERNEL_STACK, KERNEL_STACK_GUARD, USERSPACE};
use pretty::Bytes;
use serial::println;
use spin::lock;
use time::{Duration, TimeSlice};
use virtmem::map_pages;
use x86_64::instructions::interrupts::without_interrupts;
use x86_64::structures::paging::{Page, PageSize, PageTableFlags, Size4KiB};
use x86_64::VirtAddr;

/// The amount of CPU time given to threads when they are scheduled.
///
const DEFAULT_TIME_SLICE: TimeSlice = TimeSlice::from_duration(&Duration::from_millis(100));

/// The number of pages in each kernel stack.
///
/// This does not include the extra page for the stack
/// guard.
///
const KERNEL_STACK_PAGES: usize = 128;

/// The number of bytes in each kernel stack.
///
/// This does not include the extra page for the stack
/// guard.
///
const KERNEL_STACK_SIZE: usize = (Size4KiB::SIZE as usize) * KERNEL_STACK_PAGES; // 512 KiB.

/// Sets up the thread data for this CPU.
///
pub fn per_cpu_init() {
    // Start by identifying our stack space.
    let id = cpu::id();
    let offset = id * KERNEL_STACK_SIZE + Size4KiB::SIZE as usize; // Include stack guard.
    let stack_start = KERNEL_STACK_GUARD.start() + offset;
    let stack_end = stack_start + KERNEL_STACK_SIZE;
    let stack_space = VirtAddrRange::new(stack_start, stack_end);

    // Create our idle thread.
    let idle = Thread::new_idle_thread(&stack_space);
    let mut idle_threads = lock!(IDLE_THREADS);
    if idle_threads.len() != id {
        panic!(
            "thread::init() called for CPU {} with {} CPUs activated",
            id,
            idle_threads.len()
        );
    }

    idle_threads.push(idle.clone());
    drop(idle_threads);

    let mut current_threads = lock!(CURRENT_THREADS);
    if current_threads.len() != id {
        panic!(
            "thread::init() called for CPU {} with {} CPUs activated",
            id,
            current_threads.len()
        );
    }

    current_threads.push(idle);
}

/// Returns a copy of the currently executing thread.
///
pub fn current_thread() -> Arc<Thread> {
    lock!(CURRENT_THREADS)[cpu::id()].clone()
}

/// Returns the kernel thread id of the currently
/// executing thread.
///
pub fn current_kernel_thread_id() -> KernelThreadId {
    lock!(CURRENT_THREADS)[cpu::id()].kernel_thread_id()
}

/// Returns a Waker that will resume the current
/// thread.
///
pub fn current_thread_waker() -> Waker {
    current_kernel_thread_id().waker()
}

/// DEFAULT_RFLAGS contains the reserved bits of the
/// RFLAGS register so we can include them when we
/// build a new thread's initial stack.
///
/// Bit 1 is always set, as described in Figure 3-8
/// on page 78 of volume 1 of the Intel 64 manual.
///
const DEFAULT_RFLAGS: u64 = 0x2;

/// Prevents the current thread from sleeping on
/// its next attempt to do so.
///
/// This is designed for cases where a thread is
/// planning to suspend until an event will resume
/// it, but it's possible to lose the race and be
/// resumed before it can suspend.
///
/// Normally, such an event would result in the
/// resumption being ignored and the suspend lasting
/// for much longer than intended.
///
/// A thread in this situation should call `prevent_next_sleep`
/// first, configure the resumption, then call
/// [`suspend`] or [`sleep`](scheduler::sleep),
/// as appropriate.
///
pub fn prevent_next_sleep() {
    let current = current_thread();
    match current.thread_state() {
        ThreadState::BeingCreated => {
            panic!("thread::prevent_next_sleep() called on un-started thread")
        }
        ThreadState::Runnable => current.set_state(ThreadState::Drowsy),
        ThreadState::Drowsy => {}
        ThreadState::Insomniac => {}
        ThreadState::Sleeping => {
            panic!("thread::prevent_next_sleep() called while already asleep!")
        }
        ThreadState::Exiting => panic!("thread::prevent_next_sleep() called while exiting"),
    }
}

/// Puts the current thread to sleep indefinitely
/// and switches to the next runnable thread. The
/// thread can be awoken later by calling [`scheduler::resume`]
/// or by calling its [`ThreadId.resume`](KernelThreadId::resume).
///
/// # Panics
///
/// `suspend` will panic if called by the idle thread,
/// which must execute indefinitely to manage the
/// CPU.
///
pub fn suspend() {
    let current = current_thread();
    if current.kernel_id == KernelThreadId::IDLE {
        panic!("idle thread tried to suspend");
    }

    // This is just like scheduler::sleep, but we
    // don't create a waking timer.

    // Check we haven't already been prevented
    // from sleeping.
    if current.thread_state() == ThreadState::Insomniac {
        current.set_state(ThreadState::Runnable);
        return;
    }

    // Put ourselves to sleep.
    current.set_state(ThreadState::Sleeping);
    drop(current);

    // Switch to the next thread.
    scheduler::switch();
}

/// Terminates the current thread and switches to
/// the next runnable thread.
///
/// # Panics
///
/// `exit` will panic if called by the idle thread,
/// which must execute indefinitely to manage the
/// CPU.
///
pub fn exit() -> ! {
    let current = current_thread();
    if current.kernel_id == KernelThreadId::IDLE {
        panic!("idle thread tried to exit");
    }

    // We want to make sure we exit cleanly
    // by following the next steps in one go.
    //
    // If we get pre-empted after setting
    // the thread's state to exiting but
    // before we remove it from THREADS
    // or drop our handle to it, we'll
    // leak those resources.
    //
    // It's ok for us to get pre-empted
    // after we've dropped these resources,
    // as the pre-emption will have the
    // same effect and we'll never return
    // to exit.
    without_interrupts(|| {
        current.set_state(ThreadState::Exiting);
        lock!(THREADS).remove(&current.kernel_id);

        // We need to drop our handle on the current
        // thread now, as we'll never return from
        // switch.
        drop(current);
    });

    // We've now been unscheduled, so we
    // switch to the next thread.
    scheduler::switch();
    unreachable!("Exited thread was re-scheduled somehow");
}

/// Prints debug info about the currently executing thread.
///
/// Note: to debug a different thread, call its [`debug`](Thread::debug)
/// method.
///
pub fn debug() {
    // Start by getting the current stack pointer.
    let rsp: u64;
    unsafe {
        asm!("mov {}, rsp", out(reg) rsp, options(nostack, nomem, preserves_flags));
    }

    // Update the current stack pointer.
    let current = current_thread();
    unsafe { current.stack_pointer.get().write(rsp) };

    // Debug as normal.
    current.debug();
}

/// Uniquely identifies a thread throughout
/// the kernel.
///
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub struct KernelThreadId(u64);

impl KernelThreadId {
    /// IDLE is the unique thread id for the idle thread.
    ///
    pub const IDLE: Self = KernelThreadId(0);

    /// Allocates and returns the next available KernelThreadId.
    ///
    fn new() -> Self {
        static NEXT_THREAD_ID: AtomicU64 = AtomicU64::new(1);
        KernelThreadId(NEXT_THREAD_ID.fetch_add(1, Ordering::Relaxed))
    }

    /// Returns a numerical representation for the thread
    /// ID.
    ///
    pub const fn as_u64(&self) -> u64 {
        self.0
    }

    /// Resumes the referenced thread.
    ///
    pub fn resume(&self) {
        scheduler::resume(*self);
    }

    /// Returns a Waker that will resume this thread when
    /// invoked.
    ///
    pub fn waker(self) -> Waker {
        Waker::from(Arc::new(self))
    }

    /// Cancels any pending timers due to awake this thread.
    ///
    /// Returns whether any timers were cancelled without
    /// having fired.
    ///
    pub fn cancel_timers(self) -> bool {
        timers::cancel_all_for_thread(self)
    }
}

impl Wake for KernelThreadId {
    /// Wake the thread, using [`resume`](KernelThreadId::resume).
    ///
    fn wake(self: Arc<Self>) {
        self.resume();
    }

    /// Wake the thread, using [`resume`](KernelThreadId::resume).
    ///
    fn wake_by_ref(self: &Arc<Self>) {
        self.resume();
    }
}

/// Describes the scheduling state of a thread.
///
#[derive(Debug, Copy, Clone, Eq, PartialEq)]
pub enum ThreadState {
    /// The thread is being created
    /// and not yet runnable.
    BeingCreated,

    /// The thread is runnable.
    Runnable,

    /// The thread is planning to sleep
    /// shortly and should be prevented
    /// from doing so if resumed first.
    Drowsy,

    /// The thread was resumed while
    /// drowsy so will be prevented from
    /// its next attempt to suspend.
    Insomniac,

    /// The thread is sleeping.
    Sleeping,

    /// The thread is in the process
    /// of exiting.
    Exiting,
}

/// Contains the metadata for a thread of
/// execution.
///
#[derive(Debug)]
pub struct Thread {
    // This thread's unique id. The one exception
    // is the idle thread, where one instance exists
    // on each CPU. Every idle thread has the thread
    // id 0.
    kernel_id: KernelThreadId,

    // The thread's current state.
    state: UnsafeCell<ThreadState>,

    // The amount of CPU time remaining for this
    // thread.
    time_slice: UnsafeCell<TimeSlice>,

    // The thread's stack for handling interrupts.
    // Kernel threads use their main stack for interrupts,
    // so have no separate interrupt stack.
    interrupt_stack: Option<StackBounds>,

    // The thread's stack for handling syscalls.
    // Kernel threads don't call syscalls, so have
    // no separate syscall stack.
    syscall_stack: Option<StackBounds>,

    // The thread's saved user stack pointer. This is
    // for if we switch threads while handling a syscall.
    // While we handle syscalls, the user stack pointer
    // is saved in the CPU-local data. This is overwritten
    // when we switch threads so when that happens we save
    // it here. We then restore it to the CPU-local data
    // when we switch back in.
    user_stack_pointer: UnsafeCell<VirtAddr>,

    // The thread's primary stack. For kernel threads,
    // this is the only stack, and is in kernel space,
    // within the bounds of `KERNEL_STACK`.
    stack_bounds: Option<StackBounds>,

    // The thread's saved stack pointer. While the
    // thread is executing, this value will be stale.
    // When the thread is switched out, its final stack
    // pointer is written to this cell. When the thread
    // is resumed, its stack pointer is restored from
    // this value.
    stack_pointer: UnsafeCell<u64>,
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
    /// The number of pages that will be allocated
    /// for new user threads' stack. Unlike kernel
    /// stacks, these can grow at runtime.
    ///
    const DEFAULT_USER_STACK_PAGES: u64 = 128; // 128 4-KiB pages = 512 KiB.

    /// Creates a new kernel thread, to which we fall
    /// back if no other threads are runnable.
    ///
    fn new_idle_thread(stack_space: &VirtAddrRange) -> Arc<Thread> {
        // The idle thread always has thread id 0, which
        // is otherwise invalid.
        let kernel_id = KernelThreadId::IDLE;

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

        // Note that we don't store the idle thread
        // in THREADS, as it never enters the scheduler.
        // This means we only run the idle thread as a
        // last resort.

        Arc::new(Thread {
            kernel_id,
            state: UnsafeCell::new(ThreadState::Runnable),
            time_slice: UnsafeCell::new(TimeSlice::ZERO),
            interrupt_stack: None,
            syscall_stack: None,
            user_stack_pointer: UnsafeCell::new(VirtAddr::zero()),
            stack_pointer,
            stack_bounds,
        })
    }

    /// Creates a new kernel thread, allocating a stack,
    /// and marking it as not runnable.
    ///
    /// The new thread will not start until [`scheduler::resume`]
    /// is called with its thread id.
    ///
    /// When the thread runs, it will start by enabling
    /// interrupts and calling `entry_point`.
    ///
    pub fn create_kernel_thread(entry_point: fn() -> !) -> KernelThreadId {
        // Allocate and prepare the stack pointer.
        let stack = new_kernel_stack(KERNEL_STACK_PAGES as u64)
            .expect("failed to allocate stack for new kernel thread");

        let rsp = unsafe {
            let mut rsp: *mut u64 = stack.end().as_mut_ptr();

            // The stack pointer starts out pointing to
            // the last address in range, which is not
            // aligned. We add one to the pointer value
            // so it becomes aligned again. The next
            // `push_stack` will subtract 8 straight
            // away, so we will still remain within the
            // stack bounds.
            rsp = (rsp as *mut u8).add(1) as *mut u64;

            // Push the entry point, to be called by
            // start_kernel_thread.
            rsp = push_stack(rsp, entry_point as usize as u64);

            // Push start_kernel_thread and the initial
            // registers to be loaded by switch_stack.
            rsp = push_stack(rsp, start_kernel_thread as *const u8 as u64); // RIP.
            rsp = push_stack(rsp, 0); // Initial RBP.
            rsp = push_stack(rsp, 0); // Initial RBX.
            rsp = push_stack(rsp, 0); // Initial R12.
            rsp = push_stack(rsp, 0); // Initial R13.
            rsp = push_stack(rsp, 0); // Initial R14.
            rsp = push_stack(rsp, 0); // Initial R15.
            rsp = push_stack(rsp, DEFAULT_RFLAGS); // RFLAGS (interrupts disabled).

            rsp
        };

        let kernel_id = KernelThreadId::new();
        let thread = Arc::new(Thread {
            kernel_id,
            state: UnsafeCell::new(ThreadState::BeingCreated),
            time_slice: UnsafeCell::new(DEFAULT_TIME_SLICE),
            interrupt_stack: None,
            syscall_stack: None,
            user_stack_pointer: UnsafeCell::new(VirtAddr::zero()),
            stack_pointer: UnsafeCell::new(rsp as u64),
            stack_bounds: Some(stack),
        });

        without_interrupts(|| {
            lock!(THREADS).insert(kernel_id, thread);
        });

        kernel_id
    }

    /// Creates a new kernel thread, allocating a stack,
    /// and adding it to the scheduler.
    ///
    /// When the thread runs, it will start by enabling
    /// interrupts and calling `entry_point`.
    ///
    pub fn start_kernel_thread(entry_point: fn() -> !) -> KernelThreadId {
        let kernel_id = Thread::create_kernel_thread(entry_point);
        kernel_id.resume();

        kernel_id
    }

    /// Creates a new user thread, allocating a stack, and
    /// marking it as not runnable.
    ///
    /// The new thread will not start until [`scheduler::resume`]
    /// is called with its thread id.
    ///
    /// When the thread runs, it will start by enabling
    /// interrupts and calling `entry_point`.
    ///
    pub fn create_user_thread(entry_point: VirtAddr) -> KernelThreadId {
        // Allocate and prepare the stack pointer.
        // We place the stack at the end of userspace, growing
        // downwards. We need to be careful not to add 1 to the
        // end of the stack, as that would be a non-canonical
        // address.
        //
        // TODO: Support binaries that place binary segments in this space.
        let stack_top = USERSPACE.end() - 7u64;
        let stack_bottom =
            stack_top - (Thread::DEFAULT_USER_STACK_PAGES * Page::<Size4KiB>::SIZE) + 8u64;
        let stack_top_page = Page::containing_address(stack_top);
        let stack_bottom_page = Page::from_start_address(stack_bottom).unwrap();

        // Map the stack.
        let pages = Page::range_inclusive(stack_bottom_page, stack_top_page);
        let flags = PageTableFlags::PRESENT
            | PageTableFlags::USER_ACCESSIBLE
            | PageTableFlags::WRITABLE
            | PageTableFlags::NO_EXECUTE;

        map_pages(pages, &mut *lock!(physmem::ALLOCATOR), flags)
            .expect("failed to allocate stack for user thread");

        let stack = StackBounds::from_page_range(pages);
        let int_stack = new_kernel_stack(KERNEL_STACK_PAGES as u64)
            .expect("failed to allocate interrupt stack for new user thread");
        let sys_stack = new_kernel_stack(KERNEL_STACK_PAGES as u64)
            .expect("failed to allocate syscall stack for new user thread");

        let rsp = unsafe {
            let mut rsp: *mut u64 = stack.end().as_mut_ptr();

            // The stack pointer starts out pointing to
            // the last address in range, which is not
            // aligned. We subtract seven to the pointer
            // value so it becomes aligned again.
            rsp = (rsp as *mut u8).sub(7) as *mut u64;

            // Push the entry point to be used by
            // start_user_thread.
            rsp = push_stack(rsp, entry_point.as_u64());

            // Push start_user_thread and the initial
            // registers to be loaded by switch_stack.
            rsp = push_stack(rsp, start_user_thread as *const u8 as u64); // RIP.
            rsp = push_stack(rsp, 0); // Initial RBP.
            rsp = push_stack(rsp, 0); // Initial RBX.
            rsp = push_stack(rsp, 0); // Initial R12.
            rsp = push_stack(rsp, 0); // Initial R13.
            rsp = push_stack(rsp, 0); // Initial R14.
            rsp = push_stack(rsp, 0); // Initial R15.
            rsp = push_stack(rsp, DEFAULT_RFLAGS); // RFLAGS (interrupts disabled).

            rsp
        };

        let kernel_id = KernelThreadId::new();
        let thread = Arc::new(Thread {
            kernel_id,
            state: UnsafeCell::new(ThreadState::BeingCreated),
            time_slice: UnsafeCell::new(DEFAULT_TIME_SLICE),
            interrupt_stack: Some(int_stack),
            syscall_stack: Some(sys_stack),
            user_stack_pointer: UnsafeCell::new(VirtAddr::zero()),
            stack_pointer: UnsafeCell::new(rsp as u64),
            stack_bounds: Some(stack),
        });

        without_interrupts(|| {
            lock!(THREADS).insert(kernel_id, thread);
        });

        kernel_id
    }

    /// Returns the thread's unique `KernelThreadId`.
    ///
    /// This `KernelThreadId` represents the unique identifier
    /// for this thread throughout the kernel.
    ///
    pub fn kernel_thread_id(&self) -> KernelThreadId {
        self.kernel_id
    }

    /// Returns the thread's current scheduling state.
    ///
    pub fn thread_state(&self) -> ThreadState {
        unsafe { self.state.get().read() }
    }

    /// Returns the thread's stack pointer.
    ///
    pub(crate) fn stack_pointer(&self) -> *mut u64 {
        self.stack_pointer.get()
    }

    /// Updates the thread's scheduling state.
    ///
    /// If this changes the thread's state to `Sleeping`
    /// or `Exiting`, it is removed from the scheduler.
    ///
    /// # Panics
    ///
    /// `set_state` panics if changed to `BeingCreated`.
    ///
    pub fn set_state(&self, new_state: ThreadState) {
        unsafe { self.state.get().write(new_state) };
        match new_state {
            ThreadState::BeingCreated => panic!("thread state set to BeingCreated"),
            ThreadState::Runnable => {}
            ThreadState::Drowsy => {}
            ThreadState::Insomniac => {}
            ThreadState::Sleeping => lock!(SCHEDULER).remove(self.kernel_id),
            ThreadState::Exiting => lock!(SCHEDULER).remove(self.kernel_id),
        }
    }

    /// Returns the address of the top of the thread's
    /// interrupt stack. Kernel threads always return
    /// the null address.
    ///
    pub fn interrupt_stack(&self) -> VirtAddr {
        match self.interrupt_stack {
            None => VirtAddr::zero(),
            Some(range) => range.end() - 7u64,
        }
    }

    /// Returns the address of the top of the thread's
    /// syscall stack. Kernel threads always return
    /// the null address.
    ///
    pub fn syscall_stack(&self) -> VirtAddr {
        match self.syscall_stack {
            None => VirtAddr::zero(),
            Some(range) => range.end() - 7u64,
        }
    }

    /// Returns the address of the saved user stack
    /// pointer if this thread switched out while
    /// handling a syscall. Otherwise, the null address
    /// is returned.
    ///
    pub fn user_stack(&self) -> VirtAddr {
        unsafe { *self.user_stack_pointer.get() }
    }

    /// Updates the thread's user stack pointer.
    ///
    pub fn set_user_stack(&self, addr: VirtAddr) {
        // This is called when we switch from this
        // thread to another, in case we do so while
        // handling a syscall. If we do, the current
        // stack pointer is in the thread's syscall
        // stack and its user stack pointer is saved
        // in the CPU-local data. That will be
        // overwritten by the next thread after we
        // switch, so we need to save it into the
        // thread first.
        unsafe { self.user_stack_pointer.get().write(addr) };
    }

    /// Decrements the thread's time slice by a single
    /// tick, returning `true` if the time slice is now
    /// zero.
    ///
    pub(crate) fn tick(&self) -> bool {
        let time_slice = unsafe { &mut *self.time_slice.get() };
        time_slice.tick()
    }

    /// Adds the given additional time slice to the
    /// thread.
    ///
    pub fn add_time(&self, extra: TimeSlice) {
        let time_slice = unsafe { &mut *self.time_slice.get() };
        *time_slice += extra;
    }

    /// Resets the thread's time slice to its initial
    /// value.
    ///
    pub fn reset_time_slice(&self) {
        let time_slice = unsafe { &mut *self.time_slice.get() };
        *time_slice = DEFAULT_TIME_SLICE;
    }

    /// Prints debug information about the thread.
    ///
    /// Do not call debug on the currently executing
    /// thread, as its stack pointer will be out of
    /// date. Call [`debug`] instead.
    ///
    pub fn debug(&self) {
        let stack_bounds = match self.stack_bounds {
            Some(bounds) => bounds,
            None => {
                println!("thread {:?} has no stack", self.kernel_id.0);
                return;
            }
        };

        let stack_pointer = unsafe { VirtAddr::new(*self.stack_pointer.get()) };
        if !stack_bounds.contains(stack_pointer) {
            panic!(
                "thread {:?}: current stack pointer {:p} is not in stack bounds {:p}-{:p}",
                self.kernel_id.0,
                stack_pointer,
                stack_bounds.start(),
                stack_bounds.end()
            );
        }

        // Do the calculations, remembering that the stack grows
        // downwards, so some of these look the wrong way around.
        let total_stack = stack_bounds.end() - stack_bounds.start();
        let used_stack = stack_bounds.end() - stack_pointer;
        let free_stack = stack_pointer - stack_bounds.start();
        let percent = (100 * used_stack) / total_stack;
        println!(
            "thread {:?}: {} ({}%) of stack used, {} / {} remaining.",
            self.kernel_id.0,
            Bytes::from_u64(used_stack),
            percent,
            Bytes::from_u64(free_stack),
            Bytes::from_u64(total_stack)
        );
    }
}

impl Drop for Thread {
    fn drop(&mut self) {
        if self.kernel_id == KernelThreadId::IDLE {
            println!("WARNING: idle thread being dropped");
        }

        // Return our stack to the dead stacks list.
        if let Some(bounds) = self.stack_bounds {
            if KERNEL_STACK.contains_range(bounds.start(), bounds.end()) {
                free_kernel_stack(bounds);
            }
        }

        // Same again for our interrupt stack, if we have one.
        if let Some(bounds) = self.interrupt_stack {
            free_kernel_stack(bounds);
        }

        // Same again for our syscall stack, if we have one.
        if let Some(bounds) = self.syscall_stack {
            free_kernel_stack(bounds);
        }
    }
}
