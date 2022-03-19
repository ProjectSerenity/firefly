// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides functionality to parse and validate ELF binaries.

use super::{Binary, Segment};
use alloc::vec::Vec;
use memlayout::{VirtAddrRange, USERSPACE};
use x86_64::structures::paging::PageTableFlags;
use x86_64::VirtAddr;
use xmas_elf::header::{sanity_check, Class, Data, Machine, Version};
use xmas_elf::program::{ProgramHeader, Type};
use xmas_elf::ElfFile;

impl<'a> Binary<'a> {
    /// Parse an ELF binary, validating its structure.
    ///
    /// If `parse` returns an `Ok` result, no further
    /// checks will be necessary.
    ///
    pub fn parse_elf(binary: &'a [u8]) -> Result<Self, &'static str> {
        const GNU_STACK: Type = Type::OsSpecific(1685382481); // GNU stack segment.

        let elf = ElfFile::new(binary)?;
        sanity_check(&elf)?;

        match elf.header.pt1.class() {
            Class::SixtyFour => {}
            Class::ThirtyTwo => return Err("32-bit binaries are not supported"),
            _ => return Err("unknown binary class"),
        }

        match elf.header.pt1.data() {
            Data::LittleEndian => {}
            Data::BigEndian => return Err("big endian binaries are not supported"),
            _ => return Err("unknown binary data"),
        }

        match elf.header.pt1.version() {
            Version::Current => {}
            _ => return Err("unknown binary version"),
        }

        // We ignore the OS ABI.

        match elf.header.pt2.machine().as_machine() {
            Machine::X86_64 => {}
            _ => return Err("unsupported instruction set architecture"),
        }

        let entry_point = VirtAddr::try_new(elf.header.pt2.entry_point())
            .map_err(|_| "invalid entry point virtual address")?;
        if !USERSPACE.contains_addr(entry_point) {
            return Err("invalid entry point outside userspace");
        }

        // Collect the program segments, checking everything
        // is correct. We want to ensure that once we allocate
        // and switch to a new page table, we won't encounter
        // any errors and have to switch back. We also check
        // that none of the segments overlap.
        let mut regions = Vec::new();
        let mut segments = Vec::new();
        for prog in elf.program_iter() {
            match prog {
                ProgramHeader::Ph64(header) => {
                    let typ = header.get_type()?;
                    match typ {
                        Type::Load => {
                            if header.mem_size < header.file_size {
                                return Err("program segment is larger on disk than in memory");
                            }

                            // Check the segment doesn't overlap with
                            // any of the others and that the entire
                            // virtual memory space is valid.
                            let start = VirtAddr::try_new(header.virtual_addr)
                                .map_err(|_| "invalid virtual address in program segment")?;
                            let end_addr = header
                                .virtual_addr
                                .checked_add(header.mem_size)
                                .ok_or("invalid memory size in program segment")?;
                            let end = VirtAddr::try_new(end_addr)
                                .map_err(|_| "invalid memory size in program segment")?;
                            let range = VirtAddrRange::new(start, end);
                            for other in regions.iter() {
                                if range.overlaps_with(other) {
                                    return Err("program segments overlap");
                                }
                            }

                            if !USERSPACE.contains(&range) {
                                return Err("program segment is outside userspace");
                            }

                            let data = header.raw_data(&elf);
                            let mut flags =
                                PageTableFlags::PRESENT | PageTableFlags::USER_ACCESSIBLE;
                            if !header.flags.is_execute() {
                                flags |= PageTableFlags::NO_EXECUTE;
                            }
                            if header.flags.is_write() {
                                flags |= PageTableFlags::WRITABLE;
                            }

                            let segment = Segment {
                                start,
                                end,
                                data,
                                flags,
                            };

                            regions.push(range);
                            segments.push(segment);
                        }
                        Type::Tls => {
                            return Err("thread-local storage is not yet supported");
                        }
                        Type::Interp => {
                            return Err("interpreted binaries are not yet supported");
                        }
                        GNU_STACK => {
                            if header.flags.is_execute() {
                                return Err("executable stacks are not supported");
                            }
                        }
                        _ => {} // Ignore for now.
                    }
                }
                ProgramHeader::Ph32(_) => return Err("32-bit binaries are not supported"),
            }
        }

        // Check that the entry point is in one of the
        // segments.
        regions
            .iter()
            .find(|&region| region.contains_addr(entry_point))
            .ok_or("entry point is not in any program segment")?;

        Ok(Binary {
            entry_point,
            segments,
        })
    }
}
