// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! A basic implementation of PCI bus scanning, which will detect and identify
//! the PCI devices available.
//!
//! The PCI module provides a [`scan`] function to scan the set of attached
//! PCI buses for supported devices. Any supported devices are collected and
//! returned.
//!
//! PCI [`Device`]s can be used to access the resources and data of a PCI
//! device.

#![no_std]
#![deny(clippy::float_arithmetic)]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![deny(clippy::panic)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![deny(clippy::wildcard_imports)]
#![deny(deprecated_in_future)]
#![deny(keyword_idents)]
#![deny(macro_use_extern_crate)]
#![deny(missing_abi)]
#![allow(unsafe_code)]
#![deny(unused_crate_dependencies)]

extern crate alloc;

use alloc::vec::Vec;
use core::fmt;
use memory::PhysAddr;
use x86_64::instructions::port::Port;

const CONFIG_ADDRESS: u16 = 0xcf8;
const CONFIG_DATA: u16 = 0xcfc;

const NONE: u16 = 0xffff;

// See https://en.wikipedia.org/wiki/PCI_configuration_space#/media/File:Pci-config-space.svg
const VENDOR_ID: u8 = 0x00; // u16
const COMMAND: u8 = 0x04; // u16
const HEADER_TYPE: u8 = 0x0e; // u8

/// Represents a device driver that can take ownership
/// of a PCI device.
///
pub type Driver = fn(device: Device);

/// Checks whether a device driver supports the given
/// device. If it does, it will return some [`Driver`]
/// implementation which should be called to install
/// the device. Otherwise, it will return `None`.
///
pub type DriverSupportCheck = fn(device: &Device) -> Option<Driver>;

/// Scans the PCI buses for devices, returning the set of
/// discovered devices.
///
pub fn scan() -> Vec<Device> {
    let mut devices = Vec::new();
    if read_u8(0, 0, 0, HEADER_TYPE) & 0x80 == 0 {
        scan_bus(0, &mut devices);
        return devices;
    }

    let mut found = false;
    for func in 0..8 {
        if read_u16(0, 0, func, VENDOR_ID) == NONE {
            break;
        }

        found = true;
        scan_bus(func, &mut devices);
    }

    if !found {
        return devices;
    }

    for bus in 0..=255 {
        scan_bus(bus, &mut devices);
    }

    devices
}

/// Capability represents a PCI device
/// capability.
///
#[derive(Debug)]
pub struct Capability {
    pub id: u8,
    pub data: Vec<u8>,
}

/// Bar represents a PCI base address
/// register.
///
#[derive(Debug)]
pub enum Bar {
    IOMapped { port: u32 },
    MemoryMapped { addr: PhysAddr },
}

/// Device represents a PCI device.
///
pub struct Device {
    bus: u8,
    slot: u8,
    func: u8,

    pub vendor: u16,
    pub device: u16,
    pub command: u16,
    pub status: u16,
    pub revision_id: u8,
    pub prog_if: u8,
    pub subclass: u8,
    pub class_code: u8,
    pub cache_line_size: u8,
    pub latency_timer: u8,
    pub header_type: u8,
    pub built_in_self_test: u8,
    pub base_address_registers: [u32; 6],
    pub cardbus_cis_pointer: u32,
    pub subsystem_vendor: u16,
    pub subsystem_device: u16,
    pub expansion_rom_base_address: u32,
    pub capabilities_pointer: u8,
    pub interrupt_line: u8,
    pub interrupt_pin: u8,
    pub min_grant: u8,
    pub max_latency: u8,

    pub capabilities: Vec<Capability>,
}

// set_address sets the PCI slot.
//
fn set_address(bus: u8, slot: u8, func: u8, field: u8) {
    let lbus = bus as u32;
    let lslot = slot as u32;
    let lfunc = func as u32;
    let lfield = field as u32;

    let address = (lbus << 16) | (lslot << 11) | (lfunc << 8) | (lfield & 0xfc) | 0x80000000;

    unsafe {
        Port::new(CONFIG_ADDRESS).write(address);
    }
}

// The read_X and write_X functions below are fairly
// straightforward. They're all duplicated as methods
// on a device, simply because it would be fiddly and
// tedious to maintain a device as we go along through
// the discovery process.

/// Read an 8-bit unsigned integer at the given PCI
/// field.
///
pub fn read_u8(bus: u8, slot: u8, func: u8, field: u8) -> u8 {
    set_address(bus, slot, func, field);
    unsafe { Port::new(CONFIG_DATA + (field as u16 & 3)).read() }
}

/// Read an 16-bit unsigned integer at the given PCI
/// field.
///
pub fn read_u16(bus: u8, slot: u8, func: u8, field: u8) -> u16 {
    set_address(bus, slot, func, field);
    unsafe { Port::new(CONFIG_DATA + (field as u16 & 2)).read() }
}

/// Read an 32-bit unsigned integer at the given PCI
/// field.
///
pub fn read_u32(bus: u8, slot: u8, func: u8, field: u8) -> u32 {
    set_address(bus, slot, func, field);
    unsafe { Port::new(CONFIG_DATA).read() }
}

/// Write an 8-bit unsigned integer to the given PCI
/// field.
///
pub fn write_u8(bus: u8, slot: u8, func: u8, field: u8, value: u8) {
    set_address(bus, slot, func, field);
    unsafe { Port::new(CONFIG_DATA).write(value) };
}

/// Write an 16-bit unsigned integer to the given PCI
/// field.
///
pub fn write_u16(bus: u8, slot: u8, func: u8, field: u8, value: u16) {
    set_address(bus, slot, func, field);
    unsafe { Port::new(CONFIG_DATA).write(value) };
}

/// Write an 32-bit unsigned integer to the given PCI
/// field.
///
pub fn write_u32(bus: u8, slot: u8, func: u8, field: u8, value: u32) {
    set_address(bus, slot, func, field);
    unsafe { Port::new(CONFIG_DATA).write(value) };
}

impl Device {
    pub fn read_field_u8(&self, field: u8) -> u8 {
        read_u8(self.bus, self.slot, self.func, field)
    }

    pub fn read_field_u16(&self, field: u8) -> u16 {
        read_u16(self.bus, self.slot, self.func, field)
    }

    pub fn read_field_u32(&self, field: u8) -> u32 {
        read_u32(self.bus, self.slot, self.func, field)
    }

    pub fn write_field_u8(&self, field: u8, value: u8) {
        write_u8(self.bus, self.slot, self.func, field, value);
    }

    pub fn write_field_u16(&self, field: u8, value: u16) {
        write_u16(self.bus, self.slot, self.func, field, value);
    }

    pub fn write_field_u32(&self, field: u8, value: u32) {
        write_u32(self.bus, self.slot, self.func, field, value);
    }

    /// enable_bus_master informs the device
    /// that it can initiate direct memory
    /// access.
    ///
    pub fn enable_bus_master(&self) {
        let command = self.read_field_u16(COMMAND);
        self.write_field_u16(COMMAND, command | (1 << 2));
    }

    /// bar returns the corresponding base
    /// address register.
    ///
    /// # Panics
    ///
    /// `bar` will panic if `index` is not
    /// in the range `[0, 6)`.
    ///
    pub fn bar(&self, index: usize) -> Bar {
        assert!(index < 6);
        let bar = self.base_address_registers[index];
        if bar & 1 == 0 {
            Bar::MemoryMapped {
                addr: PhysAddr::new((bar & !0b1111) as usize),
            }
        } else {
            Bar::IOMapped {
                port: bar & 0xffff_fffc,
            }
        }
    }
}

impl fmt::Display for Device {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "PCI device with vendor={:04x}, device={:04x}, subsystem vendor={:04x}, subsystem device={:04x}",
            self.vendor, self.device, self.subsystem_vendor, self.subsystem_device
        )
    }
}

// scan_slot scans a PCI slot for a recognised
// device.
//
fn scan_slot(bus: u8, slot: u8, devices: &mut Vec<Device>) {
    // See https://en.wikipedia.org/wiki/PCI_configuration_space#/media/File:Pci-config-space.svg
    //
    // Read the header fields, 32 bits at a time.
    // We stop to check the vendor is valid and
    // the header type is 0, but otherwise just
    // store everything blindly.
    let mut registers = [0u32; 16];
    registers[0] = read_u32(bus, slot, 0, 0x00);
    if registers[0] as u16 == NONE {
        // Device doesn't exist.
        return;
    }

    for (i, register) in registers.iter_mut().enumerate().skip(1) {
        *register = read_u32(bus, slot, 0, (i * 4) as u8);
    }

    if (registers[3] >> 16) as u8 != 0 {
        // Not a type-0 header.
        return;
    }

    // Fetch the list of capabilities.
    let mut capabilities = Vec::new();
    let mut offset = registers[13] as u8;
    while offset != 0 {
        let id = read_u8(bus, slot, 0, offset);
        let len = read_u8(bus, slot, 0, offset + 2);
        let mut data = Vec::with_capacity(len as usize);
        for i in 0..len {
            data.push(read_u8(bus, slot, 0, offset + 3 + i));
        }

        capabilities.push(Capability { id, data });
        offset = read_u8(bus, slot, 0, offset + 1);
    }

    let dev = Device {
        bus,
        slot,
        func: 0,
        vendor: registers[0] as u16,
        device: (registers[0] >> 16) as u16,
        command: registers[1] as u16,
        status: (registers[1] >> 16) as u16,
        revision_id: registers[2] as u8,
        prog_if: (registers[2] >> 8) as u8,
        subclass: (registers[2] >> 16) as u8,
        class_code: (registers[2] >> 24) as u8,
        cache_line_size: registers[3] as u8,
        latency_timer: (registers[3] >> 8) as u8,
        header_type: (registers[3] >> 16) as u8,
        built_in_self_test: (registers[3] >> 24) as u8,
        base_address_registers: [
            registers[4],
            registers[5],
            registers[6],
            registers[7],
            registers[8],
            registers[9],
        ],
        cardbus_cis_pointer: registers[10],
        subsystem_vendor: registers[11] as u16,
        subsystem_device: (registers[11] >> 16) as u16,
        expansion_rom_base_address: registers[12],
        capabilities_pointer: registers[13] as u8,
        interrupt_line: registers[15] as u8,
        interrupt_pin: (registers[15] >> 8) as u8,
        min_grant: (registers[15] >> 16) as u8,
        max_latency: (registers[15] >> 24) as u8,
        capabilities,
    };

    devices.push(dev);
}

// scan_bus scans a PCI bus for a recognised
// device.
//
fn scan_bus(bus: u8, devices: &mut Vec<Device>) {
    for slot in 0..32 {
        scan_slot(bus, slot, devices);
    }
}
