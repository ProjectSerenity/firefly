#![no_std]
#![no_main]
#![feature(custom_test_frameworks)]
#![test_runner(kernel::test_runner)]
#![reexport_test_harness_main = "test_main"]

extern crate alloc;

use alloc::{boxed::Box, vec::Vec};
use bootloader::{entry_point, BootInfo};
use core::panic::PanicInfo;
use kernel::memory::{self, KERNEL_HEAP_SIZE};
use kernel::Bitmap;

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
    for i in 0..KERNEL_HEAP_SIZE {
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

#[panic_handler]
fn panic(info: &PanicInfo) -> ! {
    kernel::test_panic_handler(info)
}
