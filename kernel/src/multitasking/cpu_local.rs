//! cpu_local provides functionality to access a different copy of the same
//! structure on each CPU. This is used to track data like the CPU ID and
//! the currently-executing thread.

// For now, we take a simple but slightly inefficient approach, where we
// allocate a copy of the CpuData struct in the CPU-local address space
// and store a pointer to it in the GS base. To access the data, we use
// a wrapper function to retrieve the pointer from the GS base, casting
// it to the right type, then access the data as usual.
//
// This is less efficient than using offsets from the GS base directly
// in assembly, as described here[1], but it's much simpler to implement.
// If rust-osdev/x86_64#257 is merged, that will probably be used to
// replace this module.
//
// [1]: https://github.com/rust-osdev/x86_64/pull/257#issuecomment-849514649

use crate::memory::{kernel_pml4, pmm, VirtAddrRange, CPU_LOCAL};
use crate::multitasking::thread::Thread;
use alloc::sync::Arc;
use core::mem::size_of;
use core::sync::atomic::{AtomicU64, Ordering};
use x86_64::registers::model_specific::GsBase;
use x86_64::structures::paging::{
    FrameAllocator, Mapper, Page, PageSize, PageTableFlags, Size4KiB,
};

/// init prepares the current CPU's local
/// data using the given CPU ID and stack
/// space.
///
pub fn init(cpu_id: CpuId, stack_space: &VirtAddrRange) {
    if cpu_id.0 != 0 {
        unimplemented!("additional CPUs not implemented: no idle stack space");
    }

    // Next, work out where we will store our CpuId
    // data. We align up to page size to make paging
    // easier.
    let size = align_up(size_of::<CpuId>(), Size4KiB::SIZE as usize) as u64;
    let start = CPU_LOCAL.start() + cpu_id.as_u64() * size;
    let end = start + size;

    // The page addresses should already be aligned,
    // so we shouldn't get any panics here.
    let start_page = Page::from_start_address(start).expect("bad start address");
    let end_page = Page::from_start_address(end).expect("bad end address");

    // Map our per-CPU address space.
    let mut mapper = unsafe { kernel_pml4() };
    let mut frame_allocator = pmm::ALLOCATOR.lock();
    for page in Page::range(start_page, end_page) {
        let frame = frame_allocator
            .allocate_frame()
            .expect("failed to allocate for per-CPU data");

        let flags = PageTableFlags::PRESENT | PageTableFlags::WRITABLE | PageTableFlags::NO_EXECUTE;
        unsafe {
            mapper
                .map_to(page, frame, flags, &mut *frame_allocator)
                .expect("failed to map per-CPU data")
                .flush()
        };
    }

    // Store the pointer to the CpuData in the GS base.
    GsBase::write(start);

    // Create our idle thread.
    let idle = Thread::new_idle_thread(stack_space);

    // Initialise the CpuData from a pointer at the
    // start of the address space.
    let cpu_data = start.as_mut_ptr() as *mut CpuData;
    unsafe {
        cpu_data.write(CpuData {
            id: cpu_id,
            idle_thread: idle.clone(),
            current_thread: idle,
        });
    }
}

// Helper functions to expose the CPU data.

/// cpu_data is our helper function to get
/// the pointer to the CPU data from the
/// GS register.
///
unsafe fn cpu_data() -> &'static mut CpuData {
    let ptr = GsBase::read();

    &mut *(ptr.as_mut_ptr() as *mut CpuData)
}

/// cpu_id returns this CPU's unique ID.
///
pub fn cpu_id() -> CpuId {
    unsafe { cpu_data() }.id
}

/// idle_thread returns this CPU's idle thread.
///
pub fn idle_thread() -> Arc<Thread> {
    unsafe { cpu_data() }.idle_thread.clone()
}

/// current_thread returns the currently executing thread.
///
pub fn current_thread() -> Arc<Thread> {
    unsafe { cpu_data() }.current_thread.clone()
}

/// set_current_thread overwrites the currently executing
/// thread.
///
pub fn set_current_thread(thread: Arc<Thread>) {
    unsafe { cpu_data() }.current_thread = thread;
}

/// align_up aligns the given address upwards to alignment align.
///
/// Requires that align is a power of two.
///
fn align_up(addr: usize, align: usize) -> usize {
    (addr + align - 1) & !(align - 1)
}

/// CpuId uniquely identifies a CPU core.
///
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub struct CpuId(u64);

impl CpuId {
    /// new allocates and returns the next available
    /// CpuId.
    ///
    pub fn new() -> Self {
        static NEXT_CPU_ID: AtomicU64 = AtomicU64::new(0);
        CpuId(NEXT_CPU_ID.fetch_add(1, Ordering::Relaxed))
    }

    pub const fn as_u64(&self) -> u64 {
        self.0
    }
}

// CpuData contains the data specific to an individual CPU core.
//
struct CpuData {
    id: CpuId,
    idle_thread: Arc<Thread>,
    current_thread: Arc<Thread>,
}
