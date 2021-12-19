//! mmio provides functionality for interacting with memory-mapped
//! I/O devices.

// This is fairly basic support for MMIO. You allocate a region from
// a set of physical memory frames, which maps the address space. The
// Region type contains the virtual addres space it includes, which
// can then be used via the read and write methods to read/write the
// MMIO space.

use crate::memory;
use crate::memory::MMIO_SPACE;
use core::sync::atomic;
use x86_64::structures::paging::frame::PhysFrameRange;
use x86_64::structures::paging::page::Page;
use x86_64::structures::paging::page_table::PageTableFlags;
use x86_64::structures::paging::Mapper;
use x86_64::VirtAddr;

/// MMIO_START_ADDRESS is the address where the next MMIO mapping
/// will be placed.
///
static MMIO_START_ADDRESS: spin::Mutex<VirtAddr> = spin::Mutex::new(MMIO_SPACE.start());

/// access_barrier ensures the compiler will not rearrange any
/// reads or writes from one side of the barrier to the other.
///
#[inline]
pub fn access_barrier() {
    atomic::compiler_fence(atomic::Ordering::SeqCst);
}

/// reserve_space reserves the given amount of MMIO address space,
/// returning the virtual address where the reservation begins.
///
fn reserve_space(size: u64) -> VirtAddr {
    let mut start_address = MMIO_START_ADDRESS.lock();
    let out = *start_address;

    // Check we haven't gone outside the bounds
    // of the reserved MMIO address space.
    if !MMIO_SPACE.contains_addr(out + size) {
        panic!("exceeded MMIO address space");
    }

    *start_address = out + size;
    out
}

/// RegionOverflow indicates that a read or write in an MMIO
/// region exceeded the bounds of the region.
///
#[derive(Debug)]
pub struct RegionOverflow(VirtAddr);

/// Region describes a set of memory allocated for memory-mapped
/// I/O.
///
pub struct Region {
    // start is the first valid address in the region.
    start: VirtAddr,

    // end is the last valid address in the region.
    end: VirtAddr,
}

impl Region {
    /// map maps the given physical address region into the MMIO
    /// address space, returning a byte slice through which the region
    /// can be accessed.
    ///
    /// # Safety
    ///
    /// This function is unsafe because the caller must guarantee that the
    /// given physical memory region is not being used already for other
    /// purposes.
    ///
    pub unsafe fn map(range: PhysFrameRange) -> Self {
        let first_addr = range.start.start_address();
        let last_addr = range.end.start_address();
        let size = last_addr - first_addr;

        let mut mapper = memory::kernel_pml4();
        let mut frame_allocator = memory::pmm::ALLOCATOR.lock();
        let start_address = reserve_space(size);
        let mut next_address = start_address;
        for frame in range {
            let flags = PageTableFlags::PRESENT
                | PageTableFlags::WRITABLE
                | PageTableFlags::WRITE_THROUGH
                | PageTableFlags::NO_CACHE
                | PageTableFlags::GLOBAL
                | PageTableFlags::NO_EXECUTE;
            let page = Page::from_start_address(next_address).expect("bad start address");
            next_address += page.size();
            mapper
                .map_to(page, frame, flags, &mut *frame_allocator)
                .expect("failed to map MMIO page")
                .flush();
        }

        Region {
            start: start_address,
            end: next_address - 1u64,
        }
    }

    /// as_mut returns a mutable reference to the given
    /// type at the given offset into the region.
    ///
    pub fn as_mut<T: Copy>(&self, offset: u64) -> Result<&'static mut T, RegionOverflow> {
        let addr = self.start + offset;
        let size = core::mem::size_of::<T>() as u64;
        if (addr + size) > self.end {
            return Err(RegionOverflow(addr + size));
        }

        let ptr = addr.as_ptr::<T>() as *mut T;
        unsafe { Ok(ptr.as_mut().unwrap()) }
    }

    /// read reads a generic value at the given offset into
    /// the region.
    ///
    pub fn read<T: 'static + Copy>(&self, offset: u64) -> Result<T, RegionOverflow> {
        let ptr = self.as_mut(offset)?;
        Ok(*ptr)
    }

    /// write writes a generic value to the given offset into
    /// the region.
    ///
    pub fn write<T: 'static + Copy>(&mut self, offset: u64, val: T) -> Result<(), RegionOverflow> {
        let ptr = self.as_mut(offset)?;
        *ptr = val;

        Ok(())
    }
}
