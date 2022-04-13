// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements a spinlock, which will panic if it appears to
//! be deadlocked.

#![no_std]
#![deny(clippy::wildcard_imports)]
#![deny(unused_crate_dependencies)]

use core::cell::UnsafeCell;
use core::ops::{Deref, DerefMut};
use core::sync::atomic::{AtomicBool, Ordering};
use core::{fmt, hint};

/// Indicates the maximum number of attempts can be made to
/// lock a mutex before a deadlock will be assumed and the
/// kernel will panic.
///
/// There is a fine balance here between a value so small
/// that deadlocks will be declared in safe code (resulting
/// in unnecessary and unwanted kernel panics) and a value
/// so large that there will be an unnecessary delay between
/// a deadlock occurring and being identified.
///
pub const MAX_LOCK_ATTEMPTS: usize = 500_000_000;

/// A spinlock providing mutually-exclusive access to data.
///
pub struct Mutex<T: ?Sized> {
    lock: AtomicBool,
    file: UnsafeCell<&'static str>,
    line: UnsafeCell<u32>,
    data: UnsafeCell<T>,
}

/// The mutex guard, which allows mutable access to the locked
/// data.
///
/// When the guard is dropped, the lock is released.
///
pub struct MutexGuard<'lock, T: ?Sized + 'lock> {
    lock: &'lock AtomicBool,
    data: &'lock mut T,
}

unsafe impl<T: ?Sized + Send> Sync for Mutex<T> {}
unsafe impl<T: ?Sized + Send> Send for Mutex<T> {}

impl<T> Mutex<T> {
    /// Create a new inner lock, containing the given data.
    ///
    #[inline(always)]
    pub const fn new(data: T) -> Self {
        Mutex {
            lock: AtomicBool::new(false),
            file: UnsafeCell::new("???"),
            line: UnsafeCell::new(0),
            data: UnsafeCell::new(data),
        }
    }
}

impl<T: ?Sized> Mutex<T> {
    /// Returns whether the mutex is currently locked.
    ///
    /// # Safety
    ///
    /// There is no synchronisation of the return value, so
    /// it can become incorrect immediately. This should
    /// only be used as a hint.
    ///
    #[inline(always)]
    pub fn is_locked(&self) -> bool {
        self.lock.load(Ordering::Relaxed)
    }

    /// Attempts to lock the mutex.
    ///
    #[inline(always)]
    pub fn try_lock(&self) -> Option<MutexGuard<T>> {
        if self
            .lock
            .compare_exchange_weak(false, true, Ordering::Acquire, Ordering::Relaxed)
            .is_ok()
        {
            Some(MutexGuard {
                lock: &self.lock,
                data: unsafe { &mut *self.data.get() },
            })
        } else {
            None
        }
    }
}

/// Lock the mutex, panicking with a deadlock if
/// the mutex cannot be locked after [`MAX_LOCK_ATTEMPTS`]
/// attempts.
///
#[macro_export]
macro_rules! lock {
    ($mutex:expr) => {{
        let file = file!();
        let line = line!();
        $crate::_lock(&$mutex, file, line)
    }};
}

/// Lock the mutex, returning a mutex guard, which
/// will unlock the mutex once dropped.
///
#[doc(hidden)]
pub fn _lock<'lock, T: ?Sized>(
    mutex: &'lock Mutex<T>,
    file: &'static str,
    line: u32,
) -> MutexGuard<'lock, T> {
    let mut counter = 0_usize;
    while mutex
        .lock
        .compare_exchange_weak(false, true, Ordering::Acquire, Ordering::Relaxed)
        .is_err()
    {
        while mutex.is_locked() {
            counter += 1;
            if counter > MAX_LOCK_ATTEMPTS {
                let lock_file = unsafe { *mutex.file.get() };
                let lock_line = unsafe { *mutex.line.get() };
                panic!(
                    "DEADLOCK: failed to lock at {}:{}, after mutex was locked at {}:{}",
                    file, line, lock_file, lock_line
                );
            }

            hint::spin_loop();
        }
    }

    // Now that we've locked the mutex, we store
    // the file:line where it was locked.
    unsafe {
        *mutex.file.get() = file;
        *mutex.line.get() = line;
    }

    // Create and return the mutex guard.
    MutexGuard {
        lock: &mutex.lock,
        data: unsafe { &mut *mutex.data.get() },
    }
}

impl<T: ?Sized + fmt::Debug> fmt::Debug for Mutex<T> {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self.try_lock() {
            Some(guard) => write!(f, "Mutex {{ data: ")
                .and_then(|()| (*guard).fmt(f))
                .and_then(|()| write!(f, "}}")),
            None => write!(f, "Mutex {{ <locked> }}"),
        }
    }
}

impl<T: ?Sized + Default> Default for Mutex<T> {
    fn default() -> Self {
        Self::new(Default::default())
    }
}

impl<T> From<T> for Mutex<T> {
    fn from(data: T) -> Self {
        Self::new(data)
    }
}

impl<'lock, T: ?Sized + fmt::Debug> fmt::Debug for MutexGuard<'lock, T> {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        fmt::Debug::fmt(&**self, f)
    }
}

impl<'lock, T: ?Sized + fmt::Display> fmt::Display for MutexGuard<'lock, T> {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        fmt::Display::fmt(&**self, f)
    }
}

impl<'lock, T: ?Sized> Deref for MutexGuard<'lock, T> {
    type Target = T;
    fn deref(&self) -> &T {
        self.data
    }
}

impl<'lock, T: ?Sized> DerefMut for MutexGuard<'lock, T> {
    fn deref_mut(&mut self) -> &mut T {
        self.data
    }
}

impl<'lock, T: ?Sized> Drop for MutexGuard<'lock, T> {
    fn drop(&mut self) {
        self.lock.store(false, Ordering::Release);
    }
}
