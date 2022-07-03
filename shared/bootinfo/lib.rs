// Forked from bootloader 0.9.22, copyright 2018 Philipp Oppermann.
//
// Use of the original source code is governed by the MIT
// license that can be found in the LICENSE.orig file.
//
// Subsequent work copyright 2022 The Firefly Authors.
//
// Use of new and modified source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides boot information to the kernel.

#![no_std]
#![deny(clippy::float_arithmetic)]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![allow(clippy::panic)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(deprecated_in_future)]
#![deny(improper_ctypes)]
#![deny(unused_crate_dependencies)]
#![forbid(unsafe_code)]

mod memory_map;

pub use self::memory_map::*;

/// Defines the entry point function.
///
/// The function must have the signature `fn(&'static BootInfo) -> !`.
///
/// This macro just creates a function named `_start`, which the linker will use as the entry
/// point. The advantage of using this macro instead of providing an own `_start` function is
/// that the macro ensures that the function and argument types are correct.
#[macro_export]
macro_rules! entry_point {
    ($path:path) => {
        #[export_name = "_start"]
        pub extern "C" fn __impl_start(boot_info: &'static $crate::BootInfo) -> ! {
            // validate the signature of the program entry point
            let f: fn(&'static $crate::BootInfo) -> ! = $path;

            f(boot_info)
        }
    };
}

/// This structure represents the information that the bootloader passes to the kernel.
///
/// The information is passed as an argument to the entry point:
///
/// ```ignore
/// pub extern "C" fn _start(boot_info: &'static BootInfo) -> ! {
///    // […]
/// }
/// ```
///
/// Note that no type checking occurs for the entry point function, so be careful to
/// use the correct argument types. To ensure that the entry point function has the correct
/// signature, use the [`entry_point`] macro.
#[derive(Debug)]
#[repr(C)]
pub struct BootInfo {
    /// A map of the physical memory regions of the underlying machine.
    ///
    /// The bootloader queries this information from the BIOS/UEFI firmware and translates this
    /// information to Rust types. It also marks any memory regions that the bootloader uses in
    /// the memory map before passing it to the kernel. Regions marked as usable can be freely
    /// used by the kernel.
    pub memory_map: MemoryMap,
    /// The offset into the virtual address space where the physical memory is mapped.
    ///
    /// Physical addresses can be converted to virtual addresses by adding this offset to them.
    ///
    /// The mapping of the physical memory allows to access arbitrary physical frames. Accessing
    /// frames that are also mapped at other virtual addresses can easily break memory safety and
    /// cause undefined behavior. Only frames reported as `USABLE` by the memory map in the `BootInfo`
    /// can be safely accessed.
    pub physical_memory_offset: u64,
    tls_template: TlsTemplate,
    _non_exhaustive: u8, // `()` is not FFI safe
}

impl BootInfo {
    /// Create a new boot information structure. This function is only for internal purposes.
    #[doc(hidden)]
    pub fn new(
        memory_map: MemoryMap,
        tls_template: Option<TlsTemplate>,
        physical_memory_offset: u64,
    ) -> Self {
        let tls_template = tls_template.unwrap_or(TlsTemplate {
            start_addr: 0,
            file_size: 0,
            mem_size: 0,
        });
        BootInfo {
            memory_map,
            tls_template,
            physical_memory_offset,
            _non_exhaustive: 0,
        }
    }

    /// Returns information about the thread local storage segment of the kernel.
    ///
    /// Returns `None` if the kernel has no thread local storage segment.
    ///
    /// (The reason this is a method instead of a normal field is that `Option`
    /// is not FFI-safe.)
    pub fn tls_template(&self) -> Option<TlsTemplate> {
        if self.tls_template.mem_size > 0 {
            Some(self.tls_template)
        } else {
            None
        }
    }
}

/// Information about the thread local storage (TLS) template.
///
/// This template can be used to set up thread local storage for threads. For
/// each thread, a new memory location of size `mem_size` must be initialized.
/// Then the first `file_size` bytes of this template needs to be copied to the
/// location. The additional `mem_size - file_size` bytes must be initialized with
/// zero.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
#[repr(C)]
pub struct TlsTemplate {
    /// The virtual start address of the thread local storage template.
    pub start_addr: u64,
    /// The number of data bytes in the template.
    ///
    /// Corresponds to the length of the `.tdata` section.
    pub file_size: u64,
    /// The total number of bytes that the TLS segment should have in memory.
    ///
    /// Corresponds to the combined length of the `.tdata` and `.tbss` sections.
    pub mem_size: u64,
}

extern "C" {
    fn _improper_ctypes_check_bootinfo(_boot_info: BootInfo);
}
