//! Implement cooperative and preemptive multitasking, plus CPU-local data.
//!
//! ## Cooperative multitasking
//!
//! Rust provides Futures and [`async`/`await`](https://rust-lang.github.io/async-book/01_getting_started/04_async_await_primer.html)
//! as native cooperative multitasking features. The [`task`] module provides
//! a simple Future wrapper and corresponding Executor to allow the safe use
//! of `async` and `await` in the kernel.
//!
//! ## Preemptive multitasking
//!
//! The [`thread`] module implements Firefly threads, each of which has its
//! own stack and execution state. This also includes the scheduler, which
//! can be used to switch from one thread to another, and for a thread to
//! sleep and be resumed. Combined with the Programmable Interval Timer
//! handler, this will pre-empt threads to allow fair sharing of the CPU.
//!
//! ## CPU-local data
//!
//! The [`cpu_local`] module allocates a memory region for each active CPU,
//! storing the address for the region in the GS register. This allows the
//! CPU to have an independent copy of its own data.

pub mod cpu_local;
pub mod task;
pub mod thread;
