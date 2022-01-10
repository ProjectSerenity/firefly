// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Handles paging, memory management, and enabling the kernel's heap allocator.
//!
//! ## Physical memory management
//!
//! The [`pmm`] module provides the ability to allocate physical
//! memory frames. This is then used as the foundation for the
//! virtual memory manager.
//!
//! ## Virtual memory management
//!
//! The [`vmm`] module provides functionality to analyse and modify
//! page tables, manage the virtual memory address space, and
//! operate the kernel's heap.
//!
//! ## Memory-mapped I/O
//!
//! The [`mmio`] module provides a way to map physical memory into
//! the virtual address space with safe data accessors and write
//! back page flags.
//!
//! # Memory management
//!
//! This module governs various details of the management of virtual
//! and physical memory.
//!
//! ## Initialising the kernel heap
//!
//! Calling [`init`] initialises the physical and virtual memory
//! managers, then initialises the kernel heap:
//!
//! - The bootstrap physical memory manager is initialised using the memory map included in the boot info we are passed by the bootloader.
//! - The bootstrap allocator is used to initialise the virtual memory management (including for allocating page tables).
//! - The global heap allocator is initialised to set up the kernel's heap.
//! - The heap and the bootstrap physical memory manager are used to initialse the second stage physical memory manager.
//!
//! ## Kernel page tables
//!
//! Calling [`kernel_pml4`] returns a mutable reference to the kernel's
//! level 4 page table. This can be used to inspect or modify the virtual
//! page mappings.
//!
//! ## Virtual memory helpers
//!
//! This module contains various constants and helper functions for
//! physical and virtual memory management. The following constants
//! describe a [region of virtual memory](VirtAddrRange) that is used
//! for a prescribed purpose:
//!
//! - [`NULL_PAGE`]: The first virtual page, which is reserved to ensure null pointer dereferences cause a page fault.
//! - [`USERSPACE`]: The first half of virtual memory, which is used by userspace processes.
//! - [`KERNEL_BINARY`]: The kernel binary is mapped within this range.
//! - [`BOOT_INFO`]: The boot info provided by the bootloader is stored here.
//! - [`KERNEL_HEAP`]: The region used for the kernel's heap.
//! - [`KERNEL_STACK_GUARD`]: The page beneath the kernel's initial stack, reserved to ensure stack overflows cause a page fault.
//! - [`KERNEL_STACK`]: The region used for the all kernel stacks.
//! - [`KERNEL_STACK_0`]: The region used for the kernel's initial stack.
//! - [`MMIO_SPACE`]: The region used for mapping direct memory access for memory-mapped I/O.
//! - [`CPU_LOCAL`]: The region used for storing CPU-local data.
//! - [`PHYSICAL_MEMORY`]: The region into which all physical memory is mapped.
//!
//! There are also the following address constants:
//!
//! - [`KERNEL_STACK_1_START`]: The bottom of the second kernel stack.
//! - [`PHYSICAL_MEMORY_OFFSET`]: The offset at which all physical memory is mapped.
//!
//! The [`phys_to_virt_addr`] function can be called to return a virtual
//! address that can be used to access the passed physical address. A set
//! of page tables (such as the kernel's level 4 page table returned by
//! [`kernel_pml4`]) can be used with [`virt_to_phys_addrs`] to determine
//! the set of physical memory buffers referenced by the given virtual
//! memory buffer.
//!
//! ## Kernel stack management
//!
//! Each kernel thread (including the initial kernel thread, started by
//! the bootloader) has its own stack, which exist within the [`KERNEL_STACK`]
//! memory region. The initial kernel thread is given its stack ([`KERNEL_STACK_0`])
//! implicitly by the bootloader. Subsequent kernel threads are allocated
//! by calling [`new_kernel_stack`] and can be de-allocated by calling
//! [`free_kernel_stack`]. De-allocated stacks are reused and can be
//! returned by subsequent calls to [`new_kernel_stack`].

use alloc::vec;
use alloc::vec::Vec;
use bootloader::BootInfo;
use core::sync::atomic::{AtomicU64, Ordering};
use x86_64::registers::control::Cr3;
use x86_64::structures::paging::mapper::{MapToError, MappedFrame, TranslateResult};
use x86_64::structures::paging::{
    FrameAllocator, Mapper, OffsetPageTable, Page, PageSize, PageTable, PageTableFlags, Size4KiB,
    Translate,
};
use x86_64::{PhysAddr, VirtAddr};

mod constants;
pub mod mmio;
pub mod pmm;
pub mod vmm;

pub use crate::memory::constants::{
    phys_to_virt_addr, VirtAddrRange, BOOT_INFO, CPU_LOCAL, KERNEL_BINARY, KERNEL_HEAP,
    KERNEL_STACK, KERNEL_STACK_0, KERNEL_STACK_1_START, KERNEL_STACK_GUARD, MMIO_SPACE, NULL_PAGE,
    PHYSICAL_MEMORY, PHYSICAL_MEMORY_OFFSET, USERSPACE,
};

// PML4 functionality.

/// KERNEL_PML4_ADDRESS contains the virtual address of the kernel's
/// level 4 page table. This enables the kernel_pml4 function to
/// construct the structured data.
///
static KERNEL_PML4_ADDRESS: spin::Mutex<VirtAddr> = spin::Mutex::new(VirtAddr::zero());

/// Initialise the kernel's memory, including setting up the
/// heap.
///
/// - The bootstrap physical memory manager is initialised using the memory map included in the boot info we are passed by the bootloader.
/// - The bootstrap allocator is used to initialise the virtual memory management (including for allocating page tables).
/// - The global heap allocator is initialised to set up the kernel's heap.
/// - The heap and the bootstrap physical memory manager are used to initialse the second stage physical memory manager.
///
/// # Safety
///
/// This function is unsafe because the caller must guarantee
/// that the complete physical memory is mapped to virtual memory
/// at [`PHYSICAL_MEMORY_OFFSET`].
///
/// `init` must be called only once to avoid aliasing &mut
/// references (which is undefined behavior).
///
pub unsafe fn init(boot_info: &'static BootInfo) {
    // Prepare the kernel's PML4.
    let (level_4_table_frame, _) = Cr3::read();
    let phys = level_4_table_frame.start_address();
    *KERNEL_PML4_ADDRESS.lock() = phys_to_virt_addr(phys);

    let mut page_table = kernel_pml4();
    let mut frame_allocator = pmm::bootstrap(&boot_info.memory_map);

    vmm::init(&mut page_table, &mut frame_allocator).expect("heap initialization failed");

    // Switch over to a more sophisticated physical memory manager.
    pmm::init(frame_allocator);
}

/// Returns a mutable reference to the kernel's level 4 page
/// table.
///
/// # Safety
///
/// The returned page tables must only be used to translate
/// addresses when it is the active level 4 page table.
///
pub unsafe fn kernel_pml4() -> OffsetPageTable<'static> {
    let virt = KERNEL_PML4_ADDRESS.lock();
    let page_table_ptr: *mut PageTable = virt.as_mut_ptr();

    let page_table = &mut *page_table_ptr; // unsafe
    OffsetPageTable::new(page_table, PHYSICAL_MEMORY_OFFSET)
}

/// Describes a single contiguous physical memory region.
///
#[derive(Clone, Copy, Debug, PartialEq, PartialOrd)]
pub struct PhysBuffer {
    pub addr: PhysAddr,
    pub len: usize,
}

/// Translates a contiguous virtual memory region into one
/// or more contiguous physical memory regions.
///
/// If any part of the virtual memory region is not mapped
/// in the given page table, then None is returned.
///
pub fn virt_to_phys_addrs<T: Translate>(
    page_table: &T,
    addr: VirtAddr,
    len: usize,
) -> Option<Vec<PhysBuffer>> {
    // We will allow an address with length zero
    // as a special case for a single address.
    if len == 0 {
        match page_table.translate_addr(addr) {
            None => return None,
            Some(addr) => return Some(vec![PhysBuffer { addr, len }]),
        }
    }

    // Now we pass through the buffer until we
    // have translated all of it.
    let mut bufs = Vec::new();
    let mut addr = addr;
    let mut len = len;
    while len > 0 {
        match page_table.translate(addr) {
            TranslateResult::NotMapped => return None,
            TranslateResult::InvalidFrameAddress(_) => return None,
            TranslateResult::Mapped { frame, offset, .. } => {
                // Advance the buffer by the amount of
                // physical memory we just found.
                let found = (frame.size() - offset) as usize;
                let phys_addr = match frame {
                    MappedFrame::Size4KiB(frame) => frame.start_address() + offset,
                    MappedFrame::Size2MiB(frame) => frame.start_address() + offset,
                    MappedFrame::Size1GiB(frame) => frame.start_address() + offset,
                };

                bufs.push(PhysBuffer {
                    addr: phys_addr,
                    len: core::cmp::min(len, found),
                });
                addr += found;
                len = len.saturating_sub(found);
            }
        }
    }

    // TODO(#10): Merge contiguous regions to reduce the number of buffers we return.

    Some(bufs)
}

/// Aligns the given address upwards to the given alignment.
///
/// Requires that align is a power of two. Unlike x86_64::align_up,
/// this operates on usize values, rather than u64.
///
pub fn align_up(addr: usize, align: usize) -> usize {
    (addr + align - 1) & !(align - 1)
}

/// Describes the address space used for a kernel stack region.
///
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct StackBounds {
    start: VirtAddr,
    end: VirtAddr,
}

impl StackBounds {
    /// Returns a set of stack bounds consisting of the given
    /// virtual address range.
    ///
    pub fn from(range: &VirtAddrRange) -> Self {
        StackBounds {
            start: range.start(),
            end: range.end(),
        }
    }

    /// Returns the smallest valid address in the stack bounds.
    /// As the stack grows downwards, this is also known as the
    /// bottom of the stack.
    ///
    pub fn start(&self) -> VirtAddr {
        self.start
    }

    /// Returns the largest valid address in the stack bounds.
    /// As the stack grows downwards, this is also known as the
    /// top of the stack.
    ///
    pub fn end(&self) -> VirtAddr {
        self.end
    }

    /// Returns the number of pages included in the bounds.
    ///
    pub fn num_pages(&self) -> u64 {
        ((self.end - self.start) + (Size4KiB::SIZE - 1)) / Size4KiB::SIZE as u64
    }

    /// Returns whether the stack bounds include the given
    /// virtual address.
    ///
    pub fn contains(&self, addr: VirtAddr) -> bool {
        self.start <= addr && addr <= self.end
    }
}

/// Reserves `num_pages` pages of stack memory for a kernel
/// thread.
///
/// `reserve_kernel_stack` returns the page at the start of
/// the stack (the lowest address).
///
fn reserve_kernel_stack(num_pages: u64) -> Page {
    static STACK_ALLOC_NEXT: AtomicU64 = AtomicU64::new(constants::KERNEL_STACK_1_START.as_u64());
    let start_addr = VirtAddr::new(
        STACK_ALLOC_NEXT.fetch_add(num_pages * Page::<Size4KiB>::SIZE, Ordering::Relaxed),
    );

    let last_addr = start_addr + (num_pages * Page::<Size4KiB>::SIZE) - 1u64;
    if !KERNEL_STACK.contains_range(start_addr, last_addr) {
        panic!("cannot reserve kernel stack: kernel stack space exhausted");
    }

    Page::from_start_address(start_addr).expect("`STACK_ALLOC_NEXT` not page aligned")
}

/// DEAD_STACKS is a free list of kernel stacks that
/// have been released by kernel threads that have
/// exited.
///
/// If there is a stack available in DEAD_STACKS
/// when a new thread is created, it is used instead
/// of allocating a new stack. This mitigates the
/// inability to track unused stacks in new_kernel_stack,
/// which would otherwise limit the number of
/// kernel threads that can be created during the
/// lifetime of the kernel. Instead, we're left
/// with just a limit on the number of simultaneous
/// kernel threads.
///
static DEAD_STACKS: spin::Mutex<Vec<StackBounds>> = spin::Mutex::new(Vec::new());

/// Allocates `num_pages` pages of stack memory for a
/// kernel thread and guard page.
///
/// `new_kernel_stack` returns the address space of the
/// allocated stack.
///
pub fn new_kernel_stack(num_pages: u64) -> Result<StackBounds, MapToError<Size4KiB>> {
    // Check whether we can just recycle an old stack.
    // We use an extra scope so we don't hold the lock
    // on DEAD_STACKS for unnecessarily long.
    {
        let mut stacks = DEAD_STACKS.lock();
        let stack = stacks.pop();
        if let Some(stack) = stack {
            if stack.num_pages() >= num_pages {
                return Ok(stack);
            } else {
                stacks.push(stack);
            }
        }
    }

    let guard_page = reserve_kernel_stack(num_pages + 1);
    let stack_start = guard_page + 1;
    let stack_end = stack_start + num_pages;

    let mut mapper = unsafe { kernel_pml4() };
    let mut frame_allocator = pmm::ALLOCATOR.lock();
    for page in Page::range(stack_start, stack_end) {
        let frame = frame_allocator
            .allocate_frame()
            .ok_or(MapToError::FrameAllocationFailed)?;

        let flags = PageTableFlags::PRESENT | PageTableFlags::WRITABLE | PageTableFlags::NO_EXECUTE;
        unsafe {
            mapper
                .map_to(page, frame, flags, &mut *frame_allocator)?
                .flush()
        };
    }

    Ok(StackBounds {
        start: stack_start.start_address(),
        end: stack_end.start_address() - 1u64,
    })
}

/// Adds the given stack to the dead stacks list, so it
/// can be reused later.
///
pub fn free_kernel_stack(stack_bounds: StackBounds) {
    DEAD_STACKS.lock().push(stack_bounds);
}

#[test_case]
fn simple_allocation() {
    use alloc::boxed::Box;
    let heap_value_1 = Box::new(41);
    let heap_value_2 = Box::new(13);
    assert_eq!(*heap_value_1, 41);
    assert_eq!(*heap_value_2, 13);
}

#[test_case]
fn large_vec() {
    use alloc::vec::Vec;
    let n = 1000;
    let mut vec = Vec::new();
    for i in 0..n {
        vec.push(i);
    }

    assert_eq!(vec.iter().sum::<u64>(), (n - 1) * n / 2);
}

#[test_case]
fn many_boxes() {
    use alloc::boxed::Box;
    for i in 0..KERNEL_HEAP.size() {
        let x = Box::new(i);
        assert_eq!(*x, i);
    }
}

/// DebugPageTable is a helper type for testing code that
/// uses page tables. It emulates the behaviour for a level
/// 4 page table using heap memory, without modifying the
/// system page tables.
///
#[cfg(test)]
pub struct DebugPageTable {
    mappings: BTreeMap<VirtAddr, PhysFrame>,
}

#[cfg(test)]
use alloc::collections::BTreeMap;
#[cfg(test)]
use x86_64::align_down;
#[cfg(test)]
use x86_64::structures::paging::PhysFrame;

#[cfg(test)]
impl DebugPageTable {
    pub fn new() -> Self {
        DebugPageTable {
            mappings: BTreeMap::new(),
        }
    }

    pub fn map(&mut self, addr: VirtAddr, frame: PhysFrame) {
        // Check the virtual address is at a page boundary,
        // to simplify things.
        assert_eq!(addr.as_u64(), align_down(addr.as_u64(), Size4KiB::SIZE));

        self.mappings.insert(addr, frame);
    }
}

#[cfg(test)]
impl Translate for DebugPageTable {
    fn translate(&self, addr: VirtAddr) -> TranslateResult {
        let truncated = VirtAddr::new(align_down(addr.as_u64(), Size4KiB::SIZE));
        match self.mappings.get(&truncated) {
            None => return TranslateResult::NotMapped,
            Some(frame) => TranslateResult::Mapped {
                frame: MappedFrame::Size4KiB(*frame),
                offset: addr - truncated,
                flags: PageTableFlags::PRESENT,
            },
        }
    }
}

#[test_case]
fn debug_page_table() {
    // Check that the debug page table works
    // correctly.
    let mut page_table = DebugPageTable::new();
    fn phys_frame(addr: u64) -> PhysFrame {
        let addr = PhysAddr::new(addr);
        let frame = PhysFrame::from_start_address(addr);
        frame.unwrap()
    }

    assert_eq!(page_table.translate_addr(VirtAddr::new(4096)), None);
    page_table.map(VirtAddr::new(4096), phys_frame(4096));
    assert_eq!(
        page_table.translate_addr(VirtAddr::new(4096)),
        Some(PhysAddr::new(4096))
    );
    assert_eq!(
        page_table.translate_addr(VirtAddr::new(4097)),
        Some(PhysAddr::new(4097))
    );
}

#[test_case]
fn virt_to_phys_addrs() {
    // Start by making some mappings we can use.
    // We map as follows:
    // - page 1 => frame 3
    // - page 2 => frame 1
    // - page 3 => frame 2
    let page1 = VirtAddr::new(1 * Size4KiB::SIZE);
    let page2 = VirtAddr::new(2 * Size4KiB::SIZE);
    let page3 = VirtAddr::new(3 * Size4KiB::SIZE);
    let frame1 = PhysAddr::new(1 * Size4KiB::SIZE);
    let frame2 = PhysAddr::new(2 * Size4KiB::SIZE);
    let frame3 = PhysAddr::new(3 * Size4KiB::SIZE);

    let mut page_table = DebugPageTable::new();
    fn phys_frame(addr: PhysAddr) -> PhysFrame {
        let frame = PhysFrame::from_start_address(addr);
        frame.unwrap()
    }

    page_table.map(page1, phys_frame(frame3));
    page_table.map(page2, phys_frame(frame1));
    page_table.map(page3, phys_frame(frame2));

    // Simple example: single address.
    assert_eq!(
        virt_to_phys_addrs(&page_table, page1, 0),
        Some(vec![PhysBuffer {
            addr: frame3,
            len: 0
        }])
    );

    // Simple example: within a single page.
    assert_eq!(
        virt_to_phys_addrs(&page_table, page1 + 2u64, 2),
        Some(vec![PhysBuffer {
            addr: frame3 + 2u64,
            len: 2
        }])
    );

    // Crossing a split page boundary.
    assert_eq!(
        virt_to_phys_addrs(&page_table, page1 + 4090u64, 12),
        Some(vec![
            PhysBuffer {
                addr: frame3 + 4090u64,
                len: 6
            },
            PhysBuffer {
                addr: frame1,
                len: 6
            }
        ])
    );

    // Crossing a contiguous page boundary.
    // TODO: merge contiguous regions to reduce the number of buffers we return.
    assert_eq!(
        virt_to_phys_addrs(&page_table, page2 + 4090u64, 12),
        Some(vec![
            PhysBuffer {
                addr: frame1 + 4090u64,
                len: 6
            },
            PhysBuffer {
                addr: frame2,
                len: 6
            }
        ])
    );
}
