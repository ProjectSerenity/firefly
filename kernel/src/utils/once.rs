// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements a simple wrapper type, which is initialised exactly once.

use core::ops::{Deref, DerefMut};

/// A wrapper, which is initialised extacly once.
///
pub struct Once<T> {
    inner: spin::Once<T>,
}

impl<T> Once<T> {
    pub const fn new() -> Once<T> {
        Once {
            inner: spin::Once::new(),
        }
    }

    pub fn init<F: FnOnce() -> T>(&self, f: F) {
        assert!(!self.inner.is_completed());
        self.inner.call_once(f);
    }
}

impl<T> Deref for Once<T> {
    type Target = T;

    fn deref(&self) -> &T {
        self.inner.get().expect("Once not yet initialised")
    }
}

impl<T> DerefMut for Once<T> {
    fn deref_mut(&mut self) -> &mut T {
        self.inner.get_mut().expect("Once not yet initialised")
    }
}