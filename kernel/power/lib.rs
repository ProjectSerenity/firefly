// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Power management functionality, using the Advanced Configuration
//! and Power Interface (ACPI).

// See the ACPI manual version 6.4 at https://uefi.org/sites/default/files/resources/ACPI_Spec_6_4_Jan22.pdf

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
#![allow(unsafe_code)]
#![deny(unused_crate_dependencies)]

extern crate alloc;

use acpi::platform::{PlatformInfo, ProcessorState};
use acpi::sdt::Signature;
use acpi::{AcpiHandler, AcpiTables, PhysicalMapping};
use alloc::boxed::Box;
use alloc::vec;
use aml::value::{AmlValue, Args};
use aml::{AmlContext, AmlError, AmlName, DebugVerbosity, Handler};
use core::mem::size_of;
use core::ptr::NonNull;
use core::slice;
use memory::{phys_to_virt_addr, PhysAddr};
use serial::println;
use x86_64::instructions::port::Port;
use x86_64::instructions::{hlt, interrupts};

/// Fixed ACPI description table (FADT) offsets,
/// as specified in section 5.2.9, in table 5.9.
///
const MAJOR_VERSION: usize = 8; // FADT Major Version, 1 byte.
const POWER_MANAGEMENT_1A_CONTROL_BLOCK: usize = 64; // PM1a_CNT_BLK, 4 bytes.
const POWER_MANAGEMENT_1B_CONTROL_BLOCK: usize = 68; // PM1b_CNT_BLK, 4 bytes.
const MINOR_VERSION: usize = 131; // FADT Minor Version, 1 byte.

/// Sleep constants, as specified in section
/// 4.8.4.2.1.
///
const SLEEP_ENABLE: u16 = 1 << 13; // SLP_EN.

/// Detect any application processors, incrementing
/// the maximum number of cores for each one.
///
pub fn application_processors() -> usize {
    let acpi_tables = match unsafe { AcpiTables::search_for_rsdp_bios(PhysicalOffsetAcpiHandler) } {
        Ok(acpi_tables) => acpi_tables,
        Err(err) => {
            // We print an error and continue with just
            // the bootstrap processor.
            println!("Failed to find ACPI tables: {:?}", err);
            return 0;
        }
    };

    // Parse the fixed ACPI description table (FADT) in the
    // system description table (SDT) to find the power
    // management control block.

    let fadt = if let Some(fadt) = acpi_tables.sdts.get(&Signature::FADT) {
        fadt
    } else {
        return 0;
    };

    if !fadt.validated
        || (fadt.length as usize) < POWER_MANAGEMENT_1B_CONTROL_BLOCK + size_of::<u32>()
    {
        return 0;
    }

    let major_version = read_at::<u8>(fadt.physical_address + MAJOR_VERSION);
    let minor_version = read_at::<u8>(fadt.physical_address + MINOR_VERSION);
    println!("ACPI version {}.{}.", major_version, minor_version);

    // Parse the patform information to identify the number
    // of application processors (APs), if any.

    let platform_info = if let Ok(platform_info) = PlatformInfo::new(&acpi_tables) {
        platform_info
    } else {
        return 0;
    };

    let proc_info = if let Some(proc_info) = platform_info.processor_info {
        proc_info
    } else {
        return 0;
    };

    proc_info
        .application_processors
        .iter()
        .filter(|ap| ap.state == ProcessorState::WaitingForSipi)
        .count()
}

/// Shutdown the machine, terminating execution.
///
#[allow(clippy::missing_panics_doc)]
pub fn shutdown() -> ! {
    // Identify the power management configuration so
    // we have the information necessary to perform a
    // shutdown.

    let acpi_tables = match unsafe { AcpiTables::search_for_rsdp_bios(PhysicalOffsetAcpiHandler) } {
        Ok(acpi_tables) => acpi_tables,
        Err(err) => {
            println!("Failed to find ACPI tables: {:?}", err);
            fallback_shutdown();
        }
    };

    // Parse the fixed ACPI description table (FADT) in the
    // system description table (SDT) to find the power
    // management control block.

    let fadt = if let Some(fadt) = acpi_tables.sdts.get(&Signature::FADT) {
        fadt
    } else {
        fallback_shutdown();
    };

    if !fadt.validated
        || (fadt.length as usize) < POWER_MANAGEMENT_1B_CONTROL_BLOCK + size_of::<u32>()
    {
        fallback_shutdown();
    }

    let major_version = read_at::<u8>(fadt.physical_address + MAJOR_VERSION);
    let control_blocka = read_at::<u32>(fadt.physical_address + POWER_MANAGEMENT_1A_CONTROL_BLOCK);
    let control_blockb = read_at::<u32>(fadt.physical_address + POWER_MANAGEMENT_1B_CONTROL_BLOCK);

    // Parse the differentiated system description table
    // (DSDT) to find the _S5 object, which contains the
    // sleep type for shutdown.

    let mut aml = AmlContext::new(Box::new(PhysicalOffsetAmlHandler), DebugVerbosity::None);
    let dsdt = if let Some(dsdt) = &acpi_tables.dsdt {
        dsdt
    } else {
        fallback_shutdown();
    };

    let phys = PhysAddr::new(dsdt.address);
    let virt = phys_to_virt_addr(phys);
    let stream =
        unsafe { slice::from_raw_parts(virt.as_usize() as *const u8, dsdt.length as usize) };
    let (sleep_typea, sleep_typeb) = match aml.parse_table(stream) {
        Ok(_) => {
            // System state 5 (Soft Off). See section
            // 7.4.2.6.
            let name = AmlName::from_str("\\_S5").unwrap();
            let s5 = if let Ok(AmlValue::Package(s5)) = aml.namespace.get_by_path(&name) {
                s5
            } else {
                fallback_shutdown();
            };

            let typea = &s5[0];
            let typeb = &s5[1];
            if let (AmlValue::Integer(typea), AmlValue::Integer(typeb)) = (typea, typeb) {
                (*typea as u16, *typeb as u16)
            } else {
                fallback_shutdown();
            }
        }
        Err(err) => {
            println!("Failed to parse AML tables in ACPI DSDT: {:?}", err);
            fallback_shutdown();
        }
    };

    // Inform the firmware that we are preparing to sleep.
    // Before ACPI 5, this meant calling the \_GTS (Going
    // To Sleep) method. Since then, it's the \_PTS
    // (Preparing To Sleep) method.
    let pts_name = if major_version < 5 {
        AmlName::from_str("\\_GTS").unwrap()
    } else {
        AmlName::from_str("\\_PTS").unwrap()
    };

    let pts_sleep_type = AmlValue::Integer(5);
    let pts_args = Args::from_list(vec![pts_sleep_type]).unwrap();
    match aml.invoke_method(&pts_name, pts_args) {
        Ok(_) => {}
        Err(AmlError::ValueDoesNotExist(_)) => {}
        Err(err) => println!("{}: got error {:?}", pts_name, err),
    }

    println!("Shutting down...");

    // See chapter 16
    unsafe {
        Port::new(control_blocka as u16).write(sleep_typea | SLEEP_ENABLE);
    }
    if control_blockb != 0 {
        unsafe {
            Port::new(control_blockb as u16).write(sleep_typeb | SLEEP_ENABLE);
        }
    }

    loop {
        interrupts::disable();
        hlt();
    }
}

/// If we fail to shutdown using ACPI, we fall back to
/// halting the CPU.
///
fn fallback_shutdown() -> ! {
    println!("Failed to shutdown using ACPI. Halting...");
    loop {
        interrupts::disable();
        hlt();
    }
}

// Although this functionality is similar to that in MMIO,
// we reimplement it here to avoid having to store the
// `MMIO::Region` somewhere.

/// Read an instance of T at the physical address `phys`.
///
fn read_at<T: Copy>(phys: usize) -> T {
    let phys = PhysAddr::new(phys);
    let virt = phys_to_virt_addr(phys);
    unsafe { *(virt.as_usize() as *const T) }
}

/// Write T to the physical address `phys`.
///
fn write_to<T: Copy>(phys: usize, val: T) {
    let phys = PhysAddr::new(phys);
    let virt = phys_to_virt_addr(phys);
    unsafe {
        *(virt.as_usize() as *mut T) = val;
    }
}

// Helper signaling types for interacting with the ACPI
// crate.

/// An ACPI Machine Language (AML) handler that uses the
/// physical offset mapping to access physical memory.
///
#[derive(Clone, Debug)]
pub struct PhysicalOffsetAmlHandler;

impl Handler for PhysicalOffsetAmlHandler {
    fn read_u8(&self, phys: usize) -> u8 {
        read_at::<u8>(phys)
    }
    fn read_u16(&self, phys: usize) -> u16 {
        read_at::<u16>(phys)
    }
    fn read_u32(&self, phys: usize) -> u32 {
        read_at::<u32>(phys)
    }
    fn read_u64(&self, phys: usize) -> u64 {
        read_at::<u64>(phys)
    }
    fn write_u8(&mut self, phys: usize, val: u8) {
        write_to(phys, val)
    }
    fn write_u16(&mut self, phys: usize, val: u16) {
        write_to(phys, val)
    }
    fn write_u32(&mut self, phys: usize, val: u32) {
        write_to(phys, val)
    }
    fn write_u64(&mut self, phys: usize, val: u64) {
        write_to(phys, val)
    }
    fn read_io_u8(&self, port: u16) -> u8 {
        unsafe { Port::new(port).read() }
    }
    fn read_io_u16(&self, port: u16) -> u16 {
        unsafe { Port::new(port).read() }
    }
    fn read_io_u32(&self, port: u16) -> u32 {
        unsafe { Port::new(port).read() }
    }
    fn write_io_u8(&self, port: u16, val: u8) {
        unsafe { Port::new(port).write(val) }
    }
    fn write_io_u16(&self, port: u16, val: u16) {
        unsafe { Port::new(port).write(val) }
    }
    fn write_io_u32(&self, port: u16, val: u32) {
        unsafe { Port::new(port).write(val) }
    }
    fn read_pci_u8(&self, _segment: u16, bus: u8, device: u8, func: u8, offset: u16) -> u8 {
        pci::read_u8(bus, device, func, offset as u8)
    }
    fn read_pci_u16(&self, _segment: u16, bus: u8, device: u8, func: u8, offset: u16) -> u16 {
        pci::read_u16(bus, device, func, offset as u8)
    }
    fn read_pci_u32(&self, _segment: u16, bus: u8, device: u8, func: u8, offset: u16) -> u32 {
        pci::read_u32(bus, device, func, offset as u8)
    }
    fn write_pci_u8(&self, _segment: u16, bus: u8, device: u8, func: u8, offset: u16, val: u8) {
        pci::write_u8(bus, device, func, offset as u8, val);
    }
    fn write_pci_u16(&self, _segment: u16, bus: u8, device: u8, func: u8, offset: u16, val: u16) {
        pci::write_u16(bus, device, func, offset as u8, val);
    }
    fn write_pci_u32(&self, _segment: u16, bus: u8, device: u8, func: u8, offset: u16, val: u32) {
        pci::write_u32(bus, device, func, offset as u8, val);
    }
}

/// An ACPI handler that uses the physical offset mapping
/// to access physical memory. That is, rather than
/// creating new mappings to access physical memory, the
/// handler simply derives the offset into the virtual
/// memory region where all physical memory is mapped.
///
#[derive(Clone, Debug)]
pub struct PhysicalOffsetAcpiHandler;

impl AcpiHandler for PhysicalOffsetAcpiHandler {
    /// Derives the virtual address where `phys` can be found.
    ///
    unsafe fn map_physical_region<T>(&self, phys: usize, size: usize) -> PhysicalMapping<Self, T> {
        let virt = phys_to_virt_addr(PhysAddr::new(phys));
        let virt = NonNull::new(virt.as_usize() as *mut T).unwrap();
        PhysicalMapping::new(phys, virt, size, size, Self)
    }

    /// This does nothing, as we have no need to unmap the
    /// memory that was previously returned.
    ///
    fn unmap_physical_region<T>(_region: &PhysicalMapping<Self, T>) {}
}
