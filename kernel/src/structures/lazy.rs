//! lazy implements a simple wrapper type, which can be left
//! uninitialised until it is written to for the first time.

use core::ops::{Deref, DerefMut};

/// Lazy is a wrapper type that can be left uninitialized until
/// the first time it is overwritten.
///
pub struct Lazy<T> {
    value: Option<T>,
}

impl<T> Lazy<T> {
    pub const fn new() -> Lazy<T> {
        Lazy { value: None }
    }

    pub fn get(&self) -> &T {
        self.value.as_ref().expect("Lazy not yet initialised")
    }

    pub fn get_mut(&mut self) -> &mut T {
        self.value.as_mut().expect("Lazy not yet initialised")
    }

    pub fn set(&mut self, value: T) {
        self.value = Some(value);
    }
}

impl<T> Deref for Lazy<T> {
    type Target = T;

    fn deref(&self) -> &T {
        self.get()
    }
}

impl<T> DerefMut for Lazy<T> {
    fn deref_mut(&mut self) -> &mut T {
        self.get_mut()
    }
}