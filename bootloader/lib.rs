// Forked from bootloader 0.9.22, copyright 2018 Philipp Oppermann.
//
// Use of the original source code is governed by the MIT
// license that can be found in the LICENSE.orig file.
//
// Subsequent work copyright 2022 The Firefly Authors.
//
// Use of new and modified source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! This library part of the bootloader allows kernels to retrieve information from the bootloader.
//!
//! To combine your kernel with the bootloader crate you need a tool such
//! as [`bootimage`](https://github.com/rust-osdev/bootimage). See the
//! [_Writing an OS in Rust_](https://os.phil-opp.com/minimal-rust-kernel/#creating-a-bootimage)
//! blog for an explanation.

#![no_std]
#![deny(clippy::float_arithmetic)]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![allow(clippy::panic)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(deprecated_in_future)]
#![deny(keyword_idents)]
#![deny(macro_use_extern_crate)]
#![deny(missing_abi)]
#![allow(unsafe_code)]
#![deny(unused_crate_dependencies)]

use bootinfo::{BootInfo, FrameRange, MemoryRegion, MemoryRegionType};
use core::arch::{asm, global_asm};
use core::convert::TryInto;
use core::slice;
use fixedvec::{alloc_stack, FixedVec};
use x86_64::instructions::tlb;
use x86_64::registers::control::{Cr0, Cr0Flags, Cr4, Cr4Flags, Efer, EferFlags};
use x86_64::structures::paging::frame::PhysFrameRange;
use x86_64::structures::paging::page_table::PageTableEntry;
use x86_64::structures::paging::{
    Mapper, Page, PageTable, PageTableFlags, PageTableIndex, PhysFrame, RecursivePageTable,
    Size2MiB, Size4KiB,
};
use x86_64::{PhysAddr, VirtAddr};
use xmas_elf::program::{ProgramHeader, ProgramHeader64};

global_asm!(include_str!("stage_1.s"));
global_asm!(include_str!("stage_2.s"));
global_asm!(include_str!("e820.s"));
global_asm!(include_str!("stage_3.s"));
global_asm!(include_str!("uart_serial_port.s"));

unsafe fn context_switch(boot_info: VirtAddr, entry_point: VirtAddr, stack_pointer: VirtAddr) -> ! {
    asm!("mov rsp, {1}; call {0}; 2: jmp 2b",
         in(reg) entry_point.as_u64(), in(reg) stack_pointer.as_u64(), in("rdi") boot_info.as_u64());
    ::core::hint::unreachable_unchecked()
}

mod boot_info;
mod frame_allocator;
mod level4_entries;
mod page_table;

// Kernel configuration constants.

const BOOT_INFO_ADDRESS: u64 = 0xffff800040000000;
const KERNEL_STACK_ADDRESS: u64 = 0xffff80005554f000;
const KERNEL_STACK_SIZE: u64 = 128;
const PHYSICAL_MEMORY_OFFSET: u64 = 0xffff800080000000;

pub struct IdentityMappedAddr(PhysAddr);

impl IdentityMappedAddr {
    fn phys(&self) -> PhysAddr {
        self.0
    }

    fn virt(&self) -> VirtAddr {
        VirtAddr::new(self.0.as_u64())
    }

    fn as_u64(&self) -> u64 {
        self.0.as_u64()
    }
}

// Symbols defined in `linker.ld`
extern "C" {
    static mmap_ent: usize;
    static _memory_map: usize;
    static _kernel_start_addr: usize;
    static _kernel_size_addr: usize;
    static __page_table_start: usize;
    static __page_table_end: usize;
    static __bootloader_end: usize;
    static __bootloader_start: usize;
    static _p4: usize;
}

/// The Rust entry point for the bootloader.
///
/// This is called at the end of stage 3 by
/// the assembly.
///
/// # Safety
///
/// This is unsafe, as it accesses symbols
/// defined in the linker script, which may
/// have unknown bitpatterns. However, any
/// invalid values in these symbols would
/// already have caused a boot failure by
/// now.
///
#[no_mangle]
pub unsafe extern "C" fn stage_4() -> ! {
    // Set stack segment
    asm!(
        "push rbx
          mov bx, 0x0
          mov ss, bx
          pop rbx"
    );

    let kernel_start = 0x400000;
    let kernel_size = *(&_kernel_size_addr as *const _ as *const u32) as u64;
    let memory_map_addr = &_memory_map as *const _ as u64;
    let memory_map_entry_count = (mmap_ent & 0xff) as u64; // Extract lower 8 bits
    let page_table_start = &__page_table_start as *const _ as u64;
    let page_table_end = &__page_table_end as *const _ as u64;
    let bootloader_start = &__bootloader_start as *const _ as u64;
    let bootloader_end = &__bootloader_end as *const _ as u64;
    let p4_physical = &_p4 as *const _ as u64;

    bootloader_main(
        IdentityMappedAddr(PhysAddr::new(kernel_start)),
        kernel_size,
        VirtAddr::new(memory_map_addr),
        memory_map_entry_count,
        PhysAddr::new(page_table_start),
        PhysAddr::new(page_table_end),
        PhysAddr::new(bootloader_start),
        PhysAddr::new(bootloader_end),
        PhysAddr::new(p4_physical),
    )
}

#[allow(clippy::too_many_arguments)]
fn bootloader_main(
    kernel_start: IdentityMappedAddr,
    kernel_size: u64,
    memory_map_addr: VirtAddr,
    memory_map_entry_count: u64,
    page_table_start: PhysAddr,
    page_table_end: PhysAddr,
    bootloader_start: PhysAddr,
    bootloader_end: PhysAddr,
    p4_physical: PhysAddr,
) -> ! {
    let mut memory_map = boot_info::create_from(memory_map_addr, memory_map_entry_count);

    let max_phys_addr = memory_map
        .iter()
        .map(|r| r.range.end_addr())
        .max()
        .expect("no physical memory regions found");

    // Extract required information from the ELF file.
    let mut preallocated_space = alloc_stack!([ProgramHeader64; 32]);
    let mut segments = FixedVec::new(&mut preallocated_space);
    let entry_point;
    {
        let kernel_start_ptr = kernel_start.as_u64() as usize as *const u8;
        let kernel = unsafe { slice::from_raw_parts(kernel_start_ptr, kernel_size as usize) };
        let elf_file = xmas_elf::ElfFile::new(kernel).unwrap();
        xmas_elf::header::sanity_check(&elf_file).unwrap();

        entry_point = elf_file.header.pt2.entry_point();

        for program_header in elf_file.program_iter() {
            match program_header {
                ProgramHeader::Ph64(header) => segments
                    .push(*header)
                    .expect("does not support more than 32 program segments"),
                ProgramHeader::Ph32(_) => panic!("does not support 32 bit elf files"),
            }
        }
    }

    // Mark used virtual addresses
    let mut level4_entries = level4_entries::UsedLevel4Entries::new(&segments);

    // Enable support for the no-execute bit in page tables.
    enable_nxe_bit();

    // Enable support for the global bit in page tables.
    enable_global_bit();

    // Create a recursive page table entry
    let recursive_index = PageTableIndex::new(level4_entries.get_free_entry().try_into().unwrap());
    let mut entry = PageTableEntry::new();
    entry.set_addr(
        p4_physical,
        PageTableFlags::PRESENT | PageTableFlags::WRITABLE,
    );

    // Write the recursive entry into the page table
    let page_table = unsafe { &mut *(p4_physical.as_u64() as *mut PageTable) };
    page_table[recursive_index] = entry;
    tlb::flush_all();

    let recursive_page_table_addr = Page::from_page_table_indices(
        recursive_index,
        recursive_index,
        recursive_index,
        recursive_index,
    )
    .start_address();
    let page_table = unsafe { &mut *(recursive_page_table_addr.as_mut_ptr()) };
    let mut rec_page_table =
        RecursivePageTable::new(page_table).expect("recursive page table creation failed");

    // Create a frame allocator, which marks allocated frames as used in the memory map.
    let mut frame_allocator = frame_allocator::FrameAllocator {
        memory_map: &mut memory_map,
    };

    // Mark already used memory areas in frame allocator.
    {
        let zero_frame: PhysFrame = PhysFrame::from_start_address(PhysAddr::new(0)).unwrap();
        frame_allocator.mark_allocated_region(MemoryRegion {
            range: frame_range(PhysFrame::range(zero_frame, zero_frame + 1)),
            region_type: MemoryRegionType::FrameZero,
        });
        let bootloader_start_frame = PhysFrame::containing_address(bootloader_start);
        let bootloader_end_frame = PhysFrame::containing_address(bootloader_end - 1u64);
        let bootloader_memory_area =
            PhysFrame::range(bootloader_start_frame, bootloader_end_frame + 1);
        frame_allocator.mark_allocated_region(MemoryRegion {
            range: frame_range(bootloader_memory_area),
            region_type: MemoryRegionType::Bootloader,
        });
        let kernel_start_frame = PhysFrame::containing_address(kernel_start.phys());
        let kernel_end_frame =
            PhysFrame::containing_address(kernel_start.phys() + kernel_size - 1u64);
        let kernel_memory_area = PhysFrame::range(kernel_start_frame, kernel_end_frame + 1);
        frame_allocator.mark_allocated_region(MemoryRegion {
            range: frame_range(kernel_memory_area),
            region_type: MemoryRegionType::Kernel,
        });
        let page_table_start_frame = PhysFrame::containing_address(page_table_start);
        let page_table_end_frame = PhysFrame::containing_address(page_table_end - 1u64);
        let page_table_memory_area =
            PhysFrame::range(page_table_start_frame, page_table_end_frame + 1);
        frame_allocator.mark_allocated_region(MemoryRegion {
            range: frame_range(page_table_memory_area),
            region_type: MemoryRegionType::PageTable,
        });
    }

    // Unmap the ELF file.
    let kernel_start_page: Page<Size2MiB> = Page::containing_address(kernel_start.virt());
    let kernel_end_page: Page<Size2MiB> =
        Page::containing_address(kernel_start.virt() + kernel_size - 1u64);
    for page in Page::range_inclusive(kernel_start_page, kernel_end_page) {
        rec_page_table.unmap(page).expect("dealloc error").1.flush();
    }

    // Map a page for the boot info structure
    let boot_info_page = {
        let page: Page = Page::containing_address(VirtAddr::new(BOOT_INFO_ADDRESS));
        let frame = frame_allocator
            .allocate_frame(MemoryRegionType::BootInfo)
            .expect("frame allocation failed");
        let flags = PageTableFlags::PRESENT | PageTableFlags::WRITABLE | PageTableFlags::NO_EXECUTE;
        unsafe {
            page_table::map_page(
                page,
                frame,
                flags,
                &mut rec_page_table,
                &mut frame_allocator,
            )
        }
        .expect("Mapping of bootinfo page failed")
        .flush();
        page
    };

    // If no kernel stack address is provided, map the kernel stack after the boot info page
    let kernel_stack_address = Page::containing_address(VirtAddr::new(KERNEL_STACK_ADDRESS));

    // Map kernel segments.
    let kernel_memory_info = page_table::map_kernel(
        kernel_start.phys(),
        kernel_stack_address,
        KERNEL_STACK_SIZE,
        &segments,
        &mut rec_page_table,
        &mut frame_allocator,
    )
    .expect("kernel mapping failed");

    let physical_memory_offset = {
        let physical_memory_offset = PHYSICAL_MEMORY_OFFSET;

        let virt_for_phys =
            |phys: PhysAddr| -> VirtAddr { VirtAddr::new(phys.as_u64() + physical_memory_offset) };

        let start_frame = PhysFrame::<Size2MiB>::containing_address(PhysAddr::new(0));
        let end_frame = PhysFrame::<Size2MiB>::containing_address(PhysAddr::new(max_phys_addr));

        for frame in PhysFrame::range_inclusive(start_frame, end_frame) {
            let page = Page::containing_address(virt_for_phys(frame.start_address()));
            let flags = PageTableFlags::PRESENT
                | PageTableFlags::GLOBAL
                | PageTableFlags::WRITABLE
                | PageTableFlags::NO_EXECUTE;
            unsafe {
                page_table::map_page(
                    page,
                    frame,
                    flags,
                    &mut rec_page_table,
                    &mut frame_allocator,
                )
            }
            .expect("Mapping of physical memory page failed")
            .flush();
        }

        physical_memory_offset
    };

    // Construct boot info structure.
    let mut boot_info = BootInfo::new(
        memory_map,
        kernel_memory_info.tls_segment,
        physical_memory_offset,
    );
    boot_info.memory_map.sort();

    // Write boot info to boot info page.
    let boot_info_addr = boot_info_page.start_address();
    unsafe { boot_info_addr.as_mut_ptr::<BootInfo>().write(boot_info) };

    // Make sure that the kernel respects the write-protection bits, even when in ring 0.
    enable_write_protect_bit();

    // unmap recursive entry
    rec_page_table
        .unmap(Page::<Size4KiB>::containing_address(
            recursive_page_table_addr,
        ))
        .expect("error deallocating recursive entry")
        .1
        .flush();

    let entry_point = VirtAddr::new(entry_point);
    unsafe { context_switch(boot_info_addr, entry_point, kernel_memory_info.stack_end) };
}

fn enable_nxe_bit() {
    unsafe { Efer::update(|efer| *efer |= EferFlags::NO_EXECUTE_ENABLE) }
}

fn enable_global_bit() {
    unsafe { Cr4::update(|cr4| *cr4 |= Cr4Flags::PAGE_GLOBAL) };
}

fn enable_write_protect_bit() {
    unsafe { Cr0::update(|cr0| *cr0 |= Cr0Flags::WRITE_PROTECT) };
}

fn phys_frame_range(range: FrameRange) -> PhysFrameRange {
    PhysFrameRange {
        start: PhysFrame::from_start_address(PhysAddr::new(range.start_addr())).unwrap(),
        end: PhysFrame::from_start_address(PhysAddr::new(range.end_addr())).unwrap(),
    }
}

fn frame_range(range: PhysFrameRange) -> FrameRange {
    FrameRange::new(
        range.start.start_address().as_u64(),
        range.end.start_address().as_u64(),
    )
}
