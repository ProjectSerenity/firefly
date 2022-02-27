// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements user processes, which share a virtual memory space.

use crate::thread::{KernelThreadId, Thread};
use crate::PROCESSES;
use alloc::collections::btree_map::{BTreeMap, Iter};
use alloc::vec::Vec;
use core::cmp::min;
use core::ptr::write_bytes;
use core::slice;
use core::sync::atomic::{AtomicU64, Ordering};
use memlayout::{phys_to_virt_addr, VirtAddrRange, PHYSICAL_MEMORY_OFFSET, USERSPACE};
use physmem::{ArenaFrameAllocator, BitmapFrameTracker, ALLOCATOR};
use serial::println;
use spin::lock;
use virtmem::{kernel_mappings_frozen, new_page_table};
use x86_64::instructions::interrupts::without_interrupts;
use x86_64::registers::control::Cr3;
use x86_64::structures::paging::mapper::MapToError;
use x86_64::structures::paging::{
    FrameAllocator, FrameDeallocator, Mapper, OffsetPageTable, Page, PageTable, PageTableFlags,
    PhysFrame, Size4KiB,
};
use x86_64::VirtAddr;
use xmas_elf::header::{sanity_check, Class, Data, Machine, Version};
use xmas_elf::program::{ProgramHeader, Type};
use xmas_elf::ElfFile;

/// Uniquely identifies a thread within a process.
///
/// Note that this is different from a [`KernelThreadId`],
/// which is a globally unique thread id.
///
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub struct ProcessThreadId(u64);

impl ProcessThreadId {
    /// Returns a numerical representation for the process
    /// ID.
    ///
    pub const fn as_u64(&self) -> u64 {
        self.0
    }
}

/// Uniquely identifies a process throughout the
/// kernel.
///
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub struct KernelProcessId(u64);

impl KernelProcessId {
    /// Allocates and returns the next available ProcessId.
    ///
    fn new() -> Self {
        static NEXT_PROCESS_ID: AtomicU64 = AtomicU64::new(1);
        KernelProcessId(NEXT_PROCESS_ID.fetch_add(1, Ordering::Relaxed))
    }

    /// Returns a numerical representation for the process
    /// ID.
    ///
    pub const fn as_u64(&self) -> u64 {
        self.0
    }
}

/// Describes an error encountered while creating a new
/// process.
///
#[derive(Debug)]
pub enum Error {
    /// The executable binary used to create the process
    /// is invalid.
    BadBinary(&'static str),

    /// Failed to map the executable into the virtual
    /// memory space.
    MapError(MapToError<Size4KiB>),
}

/// Contains a virtual memory space, shared between
/// one or more threads of execution.
///
pub struct Process {
    /// This process's unique id.
    kernel_process_id: KernelProcessId,

    /// The physical frame where the process's level
    /// 4 page table resides.
    page_table: PhysFrame<Size4KiB>,

    /// The tracker we use for our physical memory
    /// allocation arena.
    tracker: BitmapFrameTracker,

    /// The process thread id for the next thread
    /// belonging to this process.
    next_thread_id: ProcessThreadId,

    /// Tracks this process's threads, mapping each
    /// process thread id to the corresponding kernel
    /// thread id.
    threads: BTreeMap<ProcessThreadId, KernelThreadId>,
}

impl Process {
    /// Creates a new process from the given executable
    /// binary, which is used to construct the virtual
    /// memory space.
    ///
    pub fn create_user_process(binary: &[u8]) -> Result<(KernelProcessId, KernelThreadId), Error> {
        const GNU_STACK: Type = Type::OsSpecific(1685382481); // GNU stack segment.

        let elf = ElfFile::new(binary).map_err(Error::BadBinary)?;
        sanity_check(&elf).map_err(Error::BadBinary)?;

        match elf.header.pt1.class() {
            Class::SixtyFour => {}
            Class::ThirtyTwo => return Err(Error::BadBinary("32-bit binaries are not supported")),
            _ => return Err(Error::BadBinary("unknown binary class")),
        }

        match elf.header.pt1.data() {
            Data::LittleEndian => {}
            Data::BigEndian => {
                return Err(Error::BadBinary("big endian binaries are not supported"))
            }
            _ => return Err(Error::BadBinary("unknown binary data")),
        }

        match elf.header.pt1.version() {
            Version::Current => {}
            _ => return Err(Error::BadBinary("unknown binary version")),
        }

        // We ignore the OS ABI.

        match elf.header.pt2.machine().as_machine() {
            Machine::X86_64 => {}
            _ => return Err(Error::BadBinary("unsupported instruction set architecture")),
        }

        let entry_point = VirtAddr::try_new(elf.header.pt2.entry_point())
            .map_err(|_| Error::BadBinary("invalid entry point virtual address"))?;
        if !USERSPACE.contains_addr(entry_point) {
            return Err(Error::BadBinary("invalid entry point outside userspace"));
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
                    let typ = header.get_type().map_err(Error::BadBinary)?;
                    match typ {
                        Type::Load => {
                            if header.mem_size < header.file_size {
                                return Err(Error::BadBinary(
                                    "program segment is larger on disk than in memory",
                                ));
                            }

                            // Check the segment doesn't overlap with
                            // any of the others and that the entire
                            // virtual memory space is valid.
                            let start = VirtAddr::try_new(header.virtual_addr).map_err(|_| {
                                Error::BadBinary("invalid virtual address in program segment")
                            })?;
                            let end = VirtAddr::try_new(header.virtual_addr + header.mem_size)
                                .map_err(|_| {
                                    Error::BadBinary("invalid memory size in program segment")
                                })?;
                            let range = VirtAddrRange::new(start, end);
                            for other in regions.iter() {
                                if range.overlaps_with(other) {
                                    return Err(Error::BadBinary("program segments overlap"));
                                }
                            }

                            if !USERSPACE.contains(&range) {
                                return Err(Error::BadBinary(
                                    "program segment is outside userspace",
                                ));
                            }

                            regions.push(range);
                            segments.push(header);
                        }
                        Type::Tls => {
                            return Err(Error::BadBinary(
                                "thread-local storage is not yet supported",
                            ));
                        }
                        Type::Interp => {
                            return Err(Error::BadBinary(
                                "interpreted binaries are not yet supported",
                            ));
                        }
                        GNU_STACK => {
                            if header.flags.is_execute() {
                                return Err(Error::BadBinary(
                                    "executable stacks are not supported",
                                ));
                            }
                        }
                        _ => {} // Ignore for now.
                    }
                }
                ProgramHeader::Ph32(_) => {
                    return Err(Error::BadBinary("32-bit binaries are not supported"))
                }
            }
        }

        let kernel_process_id = KernelProcessId::new();
        let mut process = Process {
            kernel_process_id,
            page_table: new_page_table(),
            tracker: lock!(ALLOCATOR).new_tracker(),
            next_thread_id: ProcessThreadId(0),
            threads: BTreeMap::new(),
        };

        // Allocate the virtual memory for the binary.
        // We copy its contents in separately to minimise
        // lock contention on the allocator. We collect
        // the set of physical frames that underpin each
        // allocation so we can copy the data in later.
        let mut allocations = Vec::new();
        let mut allocator = lock!(ALLOCATOR);
        for segment in segments.into_iter() {
            // We've already checked the virtual address and
            // both the file and memory size, so we can use
            // VirtAddr::new safely.
            let page_start = Page::containing_address(VirtAddr::new(segment.virtual_addr));
            let page_end =
                Page::containing_address(VirtAddr::new(segment.virtual_addr + segment.mem_size));
            let page_range = Page::range_inclusive(page_start, page_end);

            let mut flags = PageTableFlags::PRESENT | PageTableFlags::USER_ACCESSIBLE;
            if !segment.flags.is_execute() {
                flags |= PageTableFlags::NO_EXECUTE;
            }
            if segment.flags.is_write() {
                flags |= PageTableFlags::WRITABLE;
            }

            match process.map_pages(page_range, &mut *allocator, flags) {
                Ok(frames) => allocations.push((segment, frames)),
                Err(err) => {
                    // Drop the process to clean up any
                    // allocations we've made already.
                    drop(process);
                    return Err(Error::MapError(err));
                }
            }
        }

        drop(allocator);

        // Copy the segments into memory.
        for (segment, frames) in allocations.iter() {
            // We haven't changed to the process's
            // page table, so we can't access the
            // virtual memory directly. Instead,
            // we use the underlying physical memory
            // frames and use those.
            //
            // First, we zero the pages to ensure
            // the user process can't access any
            // stale memory.
            //
            // TODO: Only zero the parts of pages that won't be overwritten with segment data to save time.
            for frame in frames.iter() {
                let virt = phys_to_virt_addr(frame.start_address());
                unsafe { write_bytes(virt.as_mut_ptr::<u8>(), 0x00, frame.size() as usize) };
            }

            // Next, we need to check whether
            // the segment is offset into the
            // page.
            let start_addr = VirtAddr::new(segment.virtual_addr);
            let page = Page::<Size4KiB>::containing_address(start_addr);
            let offset = start_addr - page.start_address();

            // Next, we copy the file data into
            // memory.
            let data = segment.raw_data(&elf);
            let mut idx = 0;
            for (i, frame) in frames.iter().enumerate() {
                // Work out where we copy to.
                let start = if i == 0 { offset as usize } else { 0 };

                let len = min(data.len() - idx, frame.size() as usize - start);
                let virt = phys_to_virt_addr(frame.start_address()) + start;
                let dst = unsafe { slice::from_raw_parts_mut(virt.as_mut_ptr::<u8>(), len) };
                dst.copy_from_slice(&data[idx..(idx + len)]);

                idx += len;
                if idx >= data.len() {
                    break;
                }
            }

            // Finally, we would write zeroes into
            // the BSS section (found in segments
            // with a larger mem_size than file_size,
            // with the extra zeroes going after
            // the file_size, up to the mem_size).
            //
            // However, we've already zeroed the
            // memory regions, so we don't need to
            // do anything more.
        }

        let kernel_thread_id = process.create_user_thread(entry_point);

        without_interrupts(|| {
            lock!(PROCESSES).insert(kernel_process_id, process);
        });

        Ok((kernel_process_id, kernel_thread_id))
    }

    /// Creates a new user thread in this process, allocating
    /// a stack and marking it as not runnable.
    ///
    /// The new thread will not run until [`scheduler::resume`](crate::scheduler::resume)
    /// is called with its kernel thread id.
    ///
    /// When the thread runs, it will start by enabling
    /// interrupts and calling `entry_point`.
    ///
    pub fn create_user_thread(&mut self, entry_point: VirtAddr) -> KernelThreadId {
        let process_thread_id = self.next_thread_id;
        self.next_thread_id = ProcessThreadId(process_thread_id.0 + 1);

        let kernel_thread_id = Thread::create_user_thread(entry_point, self, process_thread_id);
        self.threads.insert(process_thread_id, kernel_thread_id);

        kernel_thread_id
    }

    /// Returns the process's unique `KernelProcessId`.
    ///
    pub fn kernel_process_id(&self) -> KernelProcessId {
        self.kernel_process_id
    }

    /// Returns the level 4 page table for this process.
    ///
    pub(crate) fn page_table(&self) -> PhysFrame<Size4KiB> {
        self.page_table
    }

    /// Returns an iterator over the process's
    /// threads.
    ///
    pub fn thread_iter(&self) -> Iter<ProcessThreadId, KernelThreadId> {
        self.threads.iter()
    }

    /// Remove the given thread from the process. This should
    /// only be used when the thread is exiting.
    ///
    pub(crate) fn remove_thread(&mut self, thread_id: ProcessThreadId) {
        self.threads.remove(&thread_id);
    }

    /// Map the given page range, which can be inclusive or exclusive
    /// into the process's virtual memory space. This should not be
    /// used to map memory in kernelspace.
    ///
    pub fn map_pages<R, A>(
        &mut self,
        page_range: R,
        allocator: &mut A,
        flags: PageTableFlags,
    ) -> Result<Vec<PhysFrame<Size4KiB>>, MapToError<Size4KiB>>
    where
        R: Iterator<Item = Page>,
        A: FrameAllocator<Size4KiB> + FrameDeallocator<Size4KiB>,
    {
        if !kernel_mappings_frozen() {
            panic!("mapping process user memory without having frozen the kernel page mappings");
        }

        // Prepare a page mapper using the process's
        // page table.
        let virt = phys_to_virt_addr(self.page_table.start_address());
        let page_table_ptr: *mut PageTable = virt.as_mut_ptr();
        let page_table = unsafe { &mut *page_table_ptr };
        let mut mapper = unsafe { OffsetPageTable::new(page_table, PHYSICAL_MEMORY_OFFSET) };

        // Prepare the physical allocator.
        let mut arena = ArenaFrameAllocator::new(allocator, &mut self.tracker);

        // Perform the mappings.
        let mut frames = Vec::new();
        for page in page_range {
            if !USERSPACE.contains_addr(page.start_address()) {
                panic!("cannot map non-user page using Process.map_pages");
            }

            let frame = arena
                .allocate_frame()
                .ok_or(MapToError::FrameAllocationFailed)?;
            frames.push(frame);
            unsafe {
                mapper.map_to(page, frame, flags, &mut arena)?.flush();
            }
        }

        Ok(frames)
    }
}

impl Drop for Process {
    fn drop(&mut self) {
        // TODO: Confirm that all threads are dead.

        // Check that our page table is not in use.
        let (page_table, _) = Cr3::read();
        if page_table.start_address() == self.page_table.start_address() {
            panic!(
                "Process {} is being dropped while its page table is active",
                self.kernel_process_id.0
            );
        }

        // Deallocate all our memory.
        let mut allocator = lock!(ALLOCATOR);
        let mut arena = ArenaFrameAllocator::new(&mut *allocator, &mut self.tracker);
        unsafe {
            arena.deallocate_all_frames();
            drop(arena);
            allocator.deallocate_frame(self.page_table);
        }

        println!("Exiting process {}", self.kernel_process_id.0);
    }
}