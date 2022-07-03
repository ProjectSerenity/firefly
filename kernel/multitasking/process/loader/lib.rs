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
#![deny(deprecated_in_future)]
#![forbid(unsafe_code)]
#![deny(unused_crate_dependencies)]

extern crate alloc;

mod elf;

use alloc::vec::Vec;
use memory::{PageTableFlags, VirtAddr};

/// The maximum number of program segments in any
/// one process.
///
const MAX_SEGMENTS: usize = 16;

/// Represents the parsed information about an ELF
/// binary.
///
#[derive(Debug, PartialEq, Eq)]
pub struct Binary<'data> {
    pub entry_point: VirtAddr,
    pub segments: Vec<Segment<'data>>,
    pub relocatable: bool,
    pub relocations: Vec<Relocation>,
}

impl<'data> Binary<'data> {
    /// Loads the executable binary with the given name and
    /// contents.
    ///
    pub fn parse(name: &str, content: &'data [u8]) -> Result<Self, &'static str> {
        if elf::is_elf(name, content) {
            return elf::parse_elf(content);
        }

        Err("unrecognised binary format")
    }
}

/// Represents an area in memory as part of a
/// process's virtual memory space.
///
#[derive(Debug, PartialEq, Eq)]
pub struct Segment<'data> {
    pub start: VirtAddr,
    pub end: VirtAddr,
    pub data: &'data [u8],
    pub flags: PageTableFlags,
    pub align: usize,
}

/// Represents a relative address within the
/// binary, which should be incremented by
/// the binary's virtual memory base before
/// execution.
///
/// If the `base` is `None`, the value already
/// at `addr` should be incremented. Otherwise,
/// the value at `addr` should be set to `base`
/// plus the increment.
///
#[derive(Debug, PartialEq, Eq)]
pub struct Relocation {
    pub addr: VirtAddr,
    pub base: Option<u64>,
}
