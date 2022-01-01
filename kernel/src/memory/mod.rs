//! memory handles paging and a basic physical memory
//! frame allocator.

// This module governs management of physical memory.
// Specifically, ::init and ::active_level_4_table
// produce a page table for the level 4 page table
// (or PML4), as described on the OS Dev wiki: https://wiki.osdev.org/Paging#64-Bit_Paging
// and in the Intel x86 64 manual, volume 3A, section
// 4.5 (4-Level Paging). The functionality for mapping
// pages and translating virtual to physical addresses
// is implemented in the x86_64 crate, in the
// x86_64::structures::paging::OffsetPageTable returned
// by ::init.
//
// This crate also provides a basic physical memory frame
// allocator, which is used in the allocator module to
// build the memory manager.

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
    KERNEL_STACK, KERNEL_STACK_0, KERNEL_STACK_GUARD, MMIO_SPACE, NULL_PAGE, PHYSICAL_MEMORY,
    PHYSICAL_MEMORY_OFFSET, USERSPACE,
};

// PML4 functionality.

/// KERNEL_PML4_ADDRESS contains the virtual address of the kernel's
/// level 4 page table. This enables the kernel_pml4 function to
/// construct the structured data.
///
static KERNEL_PML4_ADDRESS: spin::Mutex<VirtAddr> = spin::Mutex::new(VirtAddr::zero());

/// init initialises the kernel's memory, including setting up the
/// heap.
///
/// # Safety
///
/// This function is unsafe because the caller must guarantee that the
/// complete physical memory is mapped to virtual memory at the passed
/// PHYSICAL_MEMORY_OFFSET. Also, this function must be only called once
/// to avoid aliasing &mut references (which is undefined behavior).
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

/// kernel_pml4 returns a mutable reference to the
/// kernel's level 4 table.
///
/// # Safety
///
/// kernel_pml4 must only be called once at a time to
/// avoid aliasing &mut references (which is undefined
/// behavior).
///
pub unsafe fn kernel_pml4() -> OffsetPageTable<'static> {
    let virt = KERNEL_PML4_ADDRESS.lock();
    let page_table_ptr: *mut PageTable = virt.as_mut_ptr();

    let page_table = &mut *page_table_ptr; // unsafe
    OffsetPageTable::new(page_table, PHYSICAL_MEMORY_OFFSET)
}

/// align_up aligns the given address upwards to alignment align.
///
/// Requires that align is a power of two. Unlike x86_64::align_up,
/// this operates on usize values, rather than u64.
///
pub fn align_up(addr: usize, align: usize) -> usize {
    (addr + align - 1) & !(align - 1)
}

/// StackBounds describes the address space used
/// for a stack.
///
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct StackBounds {
    start: VirtAddr,
    end: VirtAddr,
}

impl StackBounds {
    /// from returns a set of stack bounds consisting of the
    /// given virtual address range.
    ///
    pub fn from(range: &VirtAddrRange) -> Self {
        StackBounds {
            start: range.start(),
            end: range.end() + 1u64, // StackBounds is exclusive, range is inclusive.
        }
    }

    /// start returns the smallest valid address in the
    /// stack bounds. As the stack grows downwards, this
    /// is also known as the bottom of the stack.
    ///
    pub fn start(&self) -> VirtAddr {
        self.start
    }

    /// end returns the first address beyond the stack
    /// bounds. As the stack grows downwards, this is
    /// also known as the top of the stack.
    ///
    pub fn end(&self) -> VirtAddr {
        self.end
    }

    /// num_pages returns the number of pages included
    /// in the bounds.
    ///
    pub fn num_pages(&self) -> u64 {
        (self.end - self.start) / Size4KiB::SIZE as u64
    }

    /// contains returns whether the stack bounds include
    /// the given virtual address.
    ///
    pub fn contains(&self, addr: VirtAddr) -> bool {
        self.start <= addr && addr < self.end
    }
}

/// reserve_kernel_stack reserves num_pages pages of
/// stack memory for a kernel thread.
///
/// reserve_kernel_stack returns the page at the start
/// of the stack (the lowest address).
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

/// new_kernel_stack allocates num_pages pages of stack
/// memory for a kernel thread and guard page, returning
/// the address space of the allocated stack.
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
        end: stack_end.start_address(),
    })
}

/// free_kernel_stack adds the given stack to the dead
/// stacks list, allowing us to reuse it in future.
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
