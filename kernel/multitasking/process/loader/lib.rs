// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides functionality to parse and validate loadable binaries.

#![no_std]

extern crate alloc;

mod elf;

use alloc::slice::Iter;
use alloc::string::String;
use alloc::vec::Vec;
use x86_64::structures::paging::PageTableFlags;
use x86_64::VirtAddr;

/// Represents the parsed information about an ELF
/// binary.
///
#[derive(Debug, PartialEq)]
pub struct Binary<'a> {
    entry_point: VirtAddr,
    segments: Vec<Segment<'a>>,
}

impl<'a> Binary<'a> {
    /// Loads the executable binary with the given name and
    /// contents.
    ///
    pub fn parse(name: &String, content: &'a [u8]) -> Result<Self, &'static str> {
        if elf::is_elf(name, content) {
            return elf::parse_elf(content);
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
pub struct Segment<'a> {
    pub start: VirtAddr,
    pub end: VirtAddr,
    pub data: &'a [u8],
    pub flags: PageTableFlags,
}
