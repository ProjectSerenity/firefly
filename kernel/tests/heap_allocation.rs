#![no_std]
#![no_main]
#![feature(custom_test_frameworks)]
#![test_runner(kernel::test_runner)]
#![reexport_test_harness_main = "test_main"]

extern crate alloc;

use alloc::{boxed::Box, vec::Vec};
use bootloader::bootinfo::{FrameRange, MemoryRegion, MemoryRegionType};
use bootloader::{entry_point, BootInfo};
use core::panic::PanicInfo;
use kernel::memory::pmm::BitmapFrameAllocator;
use kernel::memory::{self, KERNEL_HEAP};
use kernel::Bitmap;
use x86_64::structures::paging::{FrameAllocator, FrameDeallocator, PhysFrame};
use x86_64::PhysAddr;

entry_point!(main);

fn main(boot_info: &'static BootInfo) -> ! {
    kernel::init();
    unsafe { memory::init(boot_info) };

    test_main();
    loop {}
}

#[test_case]
fn simple_allocation() {
    let heap_value_1 = Box::new(41);
    let heap_value_2 = Box::new(13);
    assert_eq!(*heap_value_1, 41);
    assert_eq!(*heap_value_2, 13);
}

#[test_case]
fn large_vec() {
    let n = 1000;
    let mut vec = Vec::new();
    for i in 0..n {
        vec.push(i);
    }
    assert_eq!(vec.iter().sum::<u64>(), (n - 1) * n / 2);
}

#[test_case]
fn many_boxes() {
    for i in 0..KERNEL_HEAP.size() {
        let x = Box::new(i);
        assert_eq!(*x, i);
    }
}

#[test_case]
fn bitmap() {
    let mut bitmap = Bitmap::new_unset(7);
    for i in 0..7 {
        // Check it's false by default.
        assert_eq!(bitmap.get(i), false);
        assert_eq!(bitmap.next_set(), None);

        // Check set.
        bitmap.set(i);
        assert_eq!(bitmap.get(i), true);
        assert_eq!(bitmap.next_set(), Some(i));

        // Check unset.
        bitmap.unset(i);
        assert_eq!(bitmap.get(i), false);
    }

    bitmap = Bitmap::new_unset(67);
    for i in 0..67 {
        // Check it's false by default.
        assert_eq!(bitmap.get(i), false);
        assert_eq!(bitmap.next_set(), None);

        // Check set.
        bitmap.set(i);
        assert_eq!(bitmap.get(i), true);
        assert_eq!(bitmap.next_set(), Some(i));

        // Check unset.
        bitmap.unset(i);
        assert_eq!(bitmap.get(i), false);
    }

    bitmap = Bitmap::new_set(7);
    for i in 0..7 {
        // Check it's true by default.
        assert_eq!(bitmap.get(i), true);

        // Check unset.
        bitmap.unset(i);
        assert_eq!(bitmap.get(i), false);
        assert_eq!(bitmap.next_unset(), Some(i));

        // Check set.
        bitmap.set(i);
        assert_eq!(bitmap.get(i), true);
    }

    bitmap = Bitmap::new_set(67);
    for i in 0..67 {
        // Check it's true by default.
        assert_eq!(bitmap.get(i), true);

        // Check unset.
        bitmap.unset(i);
        assert_eq!(bitmap.get(i), false);
        assert_eq!(bitmap.next_unset(), Some(i));

        // Check set.
        bitmap.set(i);
        assert_eq!(bitmap.get(i), true);
    }
}

#[test_case]
fn bitmap_frame_allocator() {
    let regions = [
        MemoryRegion {
            range: FrameRange {
                start_frame_number: 0u64,
                end_frame_number: 1u64,
            },
            region_type: MemoryRegionType::FrameZero,
        },
        MemoryRegion {
            range: FrameRange {
                start_frame_number: 1u64,
                end_frame_number: 4u64,
            },
            region_type: MemoryRegionType::Reserved,
        },
        MemoryRegion {
            range: FrameRange {
                start_frame_number: 4u64,
                end_frame_number: 8u64,
            },
            region_type: MemoryRegionType::Usable,
        },
        MemoryRegion {
            range: FrameRange {
                start_frame_number: 8u64,
                end_frame_number: 12u64,
            },
            region_type: MemoryRegionType::Reserved,
        },
        MemoryRegion {
            range: FrameRange {
                start_frame_number: 12u64,
                end_frame_number: 14u64,
            },
            region_type: MemoryRegionType::Usable,
        },
    ];

    let mut alloc = unsafe { BitmapFrameAllocator::new(regions.iter()) };
    assert_eq!(alloc.num_frames, 6u64);
    assert_eq!(alloc.free_frames, 6u64);

    // Helper function to speed up making frames.
    fn frame_for(addr: u64) -> PhysFrame {
        let start_addr = PhysAddr::new(addr);
        let frame = PhysFrame::from_start_address(start_addr).unwrap();
        frame
    }

    // Do some allocations.
    assert_eq!(alloc.allocate_frame(), Some(frame_for(0x4000)));
    assert_eq!(alloc.num_frames, 6u64);
    assert_eq!(alloc.free_frames, 5u64);
    assert_eq!(alloc.allocate_frame(), Some(frame_for(0x5000)));
    assert_eq!(alloc.num_frames, 6u64);
    assert_eq!(alloc.free_frames, 4u64);

    // Do a free.
    unsafe { alloc.deallocate_frame(frame_for(0x4000)) };
    assert_eq!(alloc.num_frames, 6u64);
    assert_eq!(alloc.free_frames, 5u64);

    // Next allocation should return the address we just freed.
    assert_eq!(alloc.allocate_frame(), Some(frame_for(0x4000)));
    assert_eq!(alloc.num_frames, 6u64);
    assert_eq!(alloc.free_frames, 4u64);

    // Check that all remaining allocations are as we expect.
    assert_eq!(alloc.allocate_frame(), Some(frame_for(0x6000)));
    assert_eq!(alloc.allocate_frame(), Some(frame_for(0x7000)));
    assert_eq!(alloc.allocate_frame(), Some(frame_for(0xc000)));
    assert_eq!(alloc.allocate_frame(), Some(frame_for(0xd000)));
    assert_eq!(alloc.num_frames, 6u64);
    assert_eq!(alloc.free_frames, 0u64);

    // Check that we get nothing once we run out of frames.
    assert_eq!(alloc.allocate_frame(), None);
    assert_eq!(alloc.num_frames, 6u64);
    assert_eq!(alloc.free_frames, 0u64);
}

#[panic_handler]
fn panic(info: &PanicInfo) -> ! {
    kernel::test_panic_handler(info)
}
