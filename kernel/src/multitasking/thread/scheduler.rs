//! scheduler is a simple round robin scheduler for threads.

use crate::multitasking::thread::ThreadId;
use alloc::collections::VecDeque;

/// Scheduler is a basic thread scheduler.
///
/// Currently, it implements a round-robin algorithm.
///
pub struct Scheduler {
    runnable: spin::Mutex<VecDeque<ThreadId>>,
}

impl Scheduler {
    pub fn new() -> Scheduler {
        Scheduler {
            runnable: spin::Mutex::new(VecDeque::new()),
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
