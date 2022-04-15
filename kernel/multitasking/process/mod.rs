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
use loader::Binary;
use memory::constants::USERSPACE;
use memory::{
    phys_to_virt_addr, PageMappingError, PageTable, PageTableFlags, PhysAddr, PhysFrame,
    PhysFrameAllocator, PhysFrameDeallocator, VirtAddr, VirtPage, VirtPageRange, VirtPageSize,
};
use physmem::{ArenaFrameAllocator, BitmapFrameTracker, ALLOCATOR};
use serial::println;
use spin::lock;
use virtmem::{kernel_mappings_frozen, new_page_table};
use x86_64::instructions::interrupts::without_interrupts;
use x86_64::registers::control::Cr3;

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
    MapError(PageMappingError),
}

/// Contains a virtual memory space, shared between
/// one or more threads of execution.
///
pub struct Process {
    /// This process's unique id.
    kernel_process_id: KernelProcessId,

    /// The physical frame where the process's level
    /// 4 page table resides.
    page_table: PhysFrame,

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
    pub fn create_user_process(
        name: &str,
        binary: &[u8],
    ) -> Result<(KernelProcessId, KernelThreadId), Error> {
        let bin = Binary::parse(name, binary).map_err(Error::BadBinary)?;

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
        for segment in bin.iter_segments() {
            let page_start = VirtPage::containing_address(segment.start, VirtPageSize::Size4KiB);
            let page_end = VirtPage::containing_address(segment.end, VirtPageSize::Size4KiB);
            let page_range = VirtPage::range_inclusive(page_start, page_end);

            match process.map_pages(page_range, &mut *allocator, segment.flags) {
                Ok(frames) => allocations.push((segment, frames)),
                Err(err) => {
                    // Drop the process to clean up any
                    // allocations we've made already.
                    // Drop the allocator first as the
                    // process's cleanup code locks it.
                    drop(allocator);
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
                unsafe { write_bytes(virt.as_usize() as *mut u8, 0x00, frame.size().bytes()) };
            }

            // Next, we need to check whether
            // the segment is offset into the
            // page.
            let page = VirtPage::containing_address(segment.start, VirtPageSize::Size4KiB);
            let offset = segment.start - page.start_address();

            // Next, we copy the file data into
            // memory.
            let mut idx = 0;
            for (i, frame) in frames.iter().enumerate() {
                // Work out where we copy to.
                let start = if i == 0 { offset as usize } else { 0 };

                let len = min(segment.data.len() - idx, frame.size().bytes() - start);
                let virt = phys_to_virt_addr(frame.start_address()) + start;
                let dst = unsafe { slice::from_raw_parts_mut(virt.as_usize() as *mut u8, len) };
                dst.copy_from_slice(&segment.data[idx..(idx + len)]);

                idx += len;
                if idx >= segment.data.len() {
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

        let kernel_thread_id = process.create_user_thread(bin.entry_point());

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
    pub(crate) fn page_table(&self) -> PhysFrame {
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
    /// # Panics
    ///
    /// `map_pages` will panic if [`freeze_kernel_mappings`](virtmem::freeze_kernel_mappings)
    /// has not yet been called.
    ///
    pub fn map_pages<A>(
        &mut self,
        page_range: VirtPageRange,
        allocator: &mut A,
        flags: PageTableFlags,
    ) -> Result<Vec<PhysFrame>, PageMappingError>
    where
        A: PhysFrameAllocator + PhysFrameDeallocator,
    {
        if !kernel_mappings_frozen() {
            panic!("mapping process user memory without having frozen the kernel page mappings");
        }

        // Prepare a the process's page table.
        let mut page_table = unsafe { PageTable::at(self.page_table.start_address()).unwrap() };

        // Prepare the physical allocator.
        let mut arena = ArenaFrameAllocator::new(allocator, &mut self.tracker);

        // Perform the mappings.
        let mut frames = Vec::new();
        for page in page_range {
            if !USERSPACE.contains_addr(page.start_address()) {
                panic!("cannot map non-user page using Process.map_pages");
            }

            let frame = arena
                .allocate_phys_frame(page.size().phys_frame_size())
                .ok_or(PageMappingError::PageTableAllocationFailed)?;
            frames.push(frame);
            unsafe {
                // No need to flush the TLB, as this is not the main
                // page table yet. The TLB will be flushed when we
                // change address space.
                page_table.map(page, frame, flags, &mut arena)?.ignore();
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
        let current = PhysAddr::from_x86_64(page_table.start_address());
        if current == self.page_table.start_address() {
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
            allocator.deallocate_phys_frame(self.page_table);
        }

        println!("Exiting process {}", self.kernel_process_id.0);
    }
}
