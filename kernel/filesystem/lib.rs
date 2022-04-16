// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements the logical design for Firefly's virtual filesystem.
//!
//! This crate does not implement a specific filesystem, such as
//! [ext4](https://en.wikipedia.org/wiki/Ext4) or [NTFS](https://en.wikipedia.org/wiki/NTFS).
//! Instead, it provides the logical structure and interface exposed
//! to userspace of the virtual filesystem.

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

extern crate alloc;

use alloc::string::String;
use bitflags::bitflags;

/// The separator used in file paths.
///
pub const SEPARATOR: char = '/';

bitflags! {
    /// Describes the actions that can be taken on a
    /// file or folder.
    ///
    pub struct Permissions: u8 {
        /// No actions can be performed on a file with
        /// no permission bits set.
        ///
        const NONE = 0;

        /// A file with this bit set can be executed
        /// to start a new process.
        ///
        /// A directory cannot have this bit set.
        ///
        const EXECUTE = 1 << 0;

        /// A file with this bit set can be modified,
        /// such as to truncate its length or overwrite
        /// its contents.
        ///
        /// A directory with this bit set can be
        /// modified, such as by creating new files
        /// within the directory. If a directory does
        /// not have this bit set, then any files or
        /// directories within this directory behave
        /// as if this bit is unset.
        ///
        const WRITE = 1 << 1;

        /// A file with this bit set can be read.
        /// If a file does not have this bit set, then
        /// the only information available is its name
        /// and permissions.
        ///
        /// A directory with this bit set can be read.
        /// If a directory does not have this bit set,
        /// then the only information available is its
        /// name and permissions.
        ///
        const READ = 1 << 2;
    }
}

/// Describes a file's type.
///
#[derive(Clone, Copy, Debug, PartialEq)]
pub enum FileType {
    /// An unknown file type.
    ///
    /// This is normally experienced when accessing
    /// a file without the [`READ`](Permissions::READ) permission.
    Unknown = 0,

    /// A regular file.
    RegularFile = 1,

    /// A directory, which can contain files and
    /// other directories.
    Directory = 2,
}

/// Describes a file or directory.
///
#[derive(Debug)]
pub struct FileInfo {
    /// The file/directory's name.
    ///
    /// The name may be absolute or relative,
    /// but it will never have a trailing slash.
    ///
    pub name: String,

    /// The file/directory's type.
    ///
    /// If the file does not have the [`READ`](Permissions::READ) permission,
    /// then it will have type [`Unknown`](FileType::Unknown).
    ///
    pub file_type: FileType,

    /// The set of actions that can be performed
    /// on the file/directory.
    ///
    pub permissions: Permissions,

    /// The file's size.
    ///
    /// A directory will have size `0`.
    ///
    pub size: usize,
}
