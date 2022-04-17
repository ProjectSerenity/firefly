// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides the list of syscalls implemented by the Firefly kernel.

#![no_std]
#![deny(clippy::float_arithmetic)]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![deny(clippy::panic)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(unused_crate_dependencies)]
#![forbid(unsafe_code)]

/// The set of syscalls implemented by the Firefly kernel.
///
#[derive(Clone, Copy, Debug, PartialEq)]
pub enum Syscall {
    /// Exit the current thread.
    ExitThread = 0,

    /// Write a message to the current process's
    /// standard output.
    PrintMessage = 3,

    /// Write a message to the current process's
    /// standard error output.
    PrintError = 4,

    /// Read cryptographically-secure pseudorandom
    /// numbers into a memory buffer.
    ReadRandom = 10,
    // Ensure new values are added to check_numerical_conversion below.
}

impl Syscall {
    /// Returns the syscall with the given numerical value
    /// or None.
    ///
    pub fn from_usize(num: usize) -> Option<Self> {
        match num {
            0 => Some(Self::ExitThread),
            3 => Some(Self::PrintMessage),
            4 => Some(Self::PrintError),
            10 => Some(Self::ReadRandom),
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

    /// An invalid or malformed parameter was provided to
    /// the syscall.
    IllegalParameter = 2,
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
            2 => Some(Self::IllegalParameter),
            _ => None,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn check_numerical_conversion() {
        let syscalls = [
            Syscall::ExitThread,
            Syscall::PrintMessage,
            Syscall::PrintError,
            Syscall::ReadRandom,
        ];

        for syscall in syscalls.iter().copied() {
            assert_eq!(Some(syscall), Syscall::from_usize(syscall as usize));
        }

        let errors = [Error::NoError, Error::BadSyscall, Error::IllegalParameter];

        for error in errors.iter().copied() {
            assert_eq!(Some(error), Error::from_usize(error as usize));
        }
    }
}
