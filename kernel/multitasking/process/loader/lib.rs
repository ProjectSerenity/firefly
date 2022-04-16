// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides functionality to parse and validate loadable binaries.

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

mod elf;

use alloc::slice::Iter;
use alloc::vec::Vec;
use memory::{PageTableFlags, VirtAddr};

/// The maximum number of program segments in any
/// one process.
///
const MAX_SEGMENTS: usize = 16;

/// Represents the parsed information about an ELF
/// binary.
///
#[derive(Debug, PartialEq)]
pub struct Binary<'data> {
    entry_point: VirtAddr,
    segments: Vec<Segment<'data>>,
}

impl<'data> Binary<'data> {
    /// Loads the executable binary with the given name and
    /// contents.
    ///
    pub fn parse(name: &str, content: &'data [u8]) -> Result<Self, &'static str> {
        if elf::is_elf(name, content) {
            return elf::parse_elf(content, true);
        }

        Err("unrecognised binary format")
    }

    /// Loads the executable binary with the given name and
    /// contents, allowing program segments to exist outside
    /// userspace.
    ///
    pub fn parse_kernel(name: &str, content: &'data [u8]) -> Result<Self, &'static str> {
        if elf::is_elf(name, content) {
            return elf::parse_elf(content, false);
        }

        Err("unrecognised binary format")
    }

    /// Returns the virtual address at which the binary should
    /// start execution.
    ///
    pub fn entry_point(&self) -> VirtAddr {
        self.entry_point
    }

    /// Returns an iterator over the memory segments in the
    /// binary.
    ///
    pub fn iter_segments(&self) -> Iter<Segment> {
        self.segments.iter()
    }
}

/// Represents an area in memory as part of a
/// process's virtual memory space.
///
#[derive(Debug, PartialEq)]
pub struct Segment<'data> {
    pub start: VirtAddr,
    pub end: VirtAddr,
    pub data: &'data [u8],
    pub flags: PageTableFlags,
}
