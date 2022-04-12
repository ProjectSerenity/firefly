// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides the list of syscalls implemented by the Firefly kernel.

#![no_std]
#![deny(clippy::wildcard_imports)]
#![deny(unused_crate_dependencies)]

/// The set of syscalls implemented by the Firefly kernel.
///
#[derive(Clone, Copy, Debug, PartialEq)]
pub enum Syscall {
    /// Exit the current thread.
    ExitThread = 0,
    // Ensure new values are added to check_numerical_conversion below.
}

impl Syscall {
    /// Returns the syscall with the given numerical value
    /// or None.
    ///
    pub fn from_usize(num: usize) -> Option<Self> {
        match num {
            0 => Some(Self::ExitThread),
            _ => None,
        }
    }
}

/// The set of possible error codes returned by a syscall.
///
#[derive(Clone, Copy, Debug, PartialEq)]
pub enum Error {
    /// The syscall completed successfully.
    NoError = 0,

    /// The requested syscall does not exist, or has not
    /// been implemented.
    BadSyscall = 1,
    // Ensure new values are added to check_numerical_conversion below.
}

impl Error {
    /// Returns the error with the given numerical value
    /// or None.
    ///
    pub fn from_usize(num: usize) -> Option<Self> {
        match num {
            0 => Some(Self::NoError),
            1 => Some(Self::BadSyscall),
            _ => None,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn check_numerical_conversion() {
        let syscalls = [Syscall::ExitThread];

        for syscall in syscalls.iter().copied() {
            assert_eq!(Some(syscall), Syscall::from_usize(syscall as usize));
        }

        let errors = [Error::NoError, Error::BadSyscall];

        for error in errors.iter().copied() {
            assert_eq!(Some(error), Error::from_usize(error as usize));
        }
    }
}
