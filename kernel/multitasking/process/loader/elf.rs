// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides functionality to parse and validate ELF binaries.

use super::{Binary, Segment};
use alloc::string::String;
use alloc::vec::Vec;
use memlayout::{VirtAddrRange, USERSPACE};
use x86_64::structures::paging::PageTableFlags;
use x86_64::VirtAddr;
use xmas_elf::header::{sanity_check, Class, Data, Machine, Version};
use xmas_elf::program::{ProgramHeader, Type};
use xmas_elf::ElfFile;

/// Determines whether the given binary is likely
/// and ELF binary.
///
pub fn is_elf(name: &String, content: &[u8]) -> bool {
    // We don't care about the name for ELF binaries.
    _ = name;
    content.starts_with(&[0x7f, b'E', b'L', b'F'])
}

/// Parse an ELF binary, validating its structure.
///
/// If `parse` returns an `Ok` result, no further
/// checks will be necessary.
///
pub fn parse_elf<'a>(binary: &'a [u8]) -> Result<Binary, &'static str> {
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
                        let mut flags = PageTableFlags::PRESENT | PageTableFlags::USER_ACCESSIBLE;
                        if !header.flags.is_execute() {
                            flags |= PageTableFlags::NO_EXECUTE;
                        }
                        if header.flags.is_write() {
                            flags |= PageTableFlags::WRITABLE;
                        }

                        if flags.contains(PageTableFlags::WRITABLE)
                            && !flags.contains(PageTableFlags::NO_EXECUTE)
                        {
                            return Err("program segments cannot be both writable and executable");
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

#[cfg(test)]
mod test {
    extern crate std;
    use super::*;
    use alloc::vec;
    use core::include_bytes;
    use hex_literal::hex;

    #[test]
    fn test_elf_parsing() {
        // ```
        // % readelf -W -l testdata/x86_64-linux-none-simple.elf
        //
        // Elf file type is EXEC (Executable file)
        // Entry point 0x201170
        // There are 5 program headers, starting at offset 64
        //
        // Program Headers:
        //   Type           Offset   VirtAddr           PhysAddr           FileSiz  MemSiz   Flg Align
        //   PHDR           0x000040 0x0000000000200040 0x0000000000200040 0x000118 0x000118 R   0x8
        //   LOAD           0x000000 0x0000000000200000 0x0000000000200000 0x000170 0x000170 R   0x1000
        //   LOAD           0x000170 0x0000000000201170 0x0000000000201170 0x000005 0x000005 R E 0x1000
        //   GNU_STACK      0x000000 0x0000000000000000 0x0000000000000000 0x000000 0x000000 RW  0
        //   NOTE           0x000158 0x0000000000200158 0x0000000000200158 0x000018 0x000018 R   0x4
        //
        //  Section to Segment mapping:
        //   Segment Sections...
        //    00
        //    01     .note.gnu.build-id
        //    02     .text
        //    03
        //    04     .note.gnu.build-id
        //
        // % xxd -seek $((0x0)) -len $((0x170)) -plain testdata/x86_64-linux-none-simple.elf
        // 7f454c4602010100000000000000000002003e0001000000701120000000
        // 000040000000000000000002000000000000000000004000380005004000
        // 070005000600000004000000400000000000000040002000000000004000
        // 200000000000180100000000000018010000000000000800000000000000
        // 010000000400000000000000000000000000200000000000000020000000
        // 000070010000000000007001000000000000001000000000000001000000
        // 050000007001000000000000701120000000000070112000000000000500
        // 0000000000000500000000000000001000000000000051e5746406000000
        // 000000000000000000000000000000000000000000000000000000000000
        // 000000000000000000000000000000000000040000000400000058010000
        // 000000005801200000000000580120000000000018000000000000001800
        // 0000000000000400000000000000040000000800000003000000474e5500
        // 2dd0365d5b0e7deb
        //
        // % xxd -seek $((0x170)) -len $((0x5)) -plain testdata/x86_64-linux-none-simple.elf
        // 4831c00f05
        //
        // ```
        let simple = include_bytes!("testdata/x86_64-linux-none-simple.elf");
        assert_eq!(
            parse_elf(simple),
            Ok(Binary {
                entry_point: VirtAddr::new(0x201170),
                segments: vec![
                    Segment {
                        start: VirtAddr::new(0x200000),
                        end: VirtAddr::new(0x200000 + 0x170),
                        data: &hex!(
                            "7f454c4602010100000000000000000002003e0001000000701120000000"
                            "000040000000000000000002000000000000000000004000380005004000"
                            "070005000600000004000000400000000000000040002000000000004000"
                            "200000000000180100000000000018010000000000000800000000000000"
                            "010000000400000000000000000000000000200000000000000020000000"
                            "000070010000000000007001000000000000001000000000000001000000"
                            "050000007001000000000000701120000000000070112000000000000500"
                            "0000000000000500000000000000001000000000000051e5746406000000"
                            "000000000000000000000000000000000000000000000000000000000000"
                            "000000000000000000000000000000000000040000000400000058010000"
                            "000000005801200000000000580120000000000018000000000000001800"
                            "0000000000000400000000000000040000000800000003000000474e5500"
                            "2dd0365d5b0e7deb"
                        ),
                        flags: PageTableFlags::PRESENT
                            | PageTableFlags::USER_ACCESSIBLE
                            | PageTableFlags::NO_EXECUTE,
                    },
                    Segment {
                        start: VirtAddr::new(0x201170),
                        end: VirtAddr::new(0x201170 + 0x5),
                        data: &hex!("4831c00f05"),
                        flags: PageTableFlags::PRESENT | PageTableFlags::USER_ACCESSIBLE,
                    },
                ],
            })
        );
    }
}
