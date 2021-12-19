//! pci is a basic implementation of PCI bus scanning, which will
//! detect and identify the PCI devices available.

use crate::{drivers, println};
use alloc::vec::Vec;
use core::fmt;
use x86_64::instructions::port::Port;
use x86_64::PhysAddr;

const CONFIG_ADDRESS: u16 = 0xcf8;
const CONFIG_DATA: u16 = 0xcfc;

const NONE: u16 = 0xffff;

// See https://en.wikipedia.org/wiki/PCI_configuration_space#/media/File:Pci-config-space.svg
const VENDOR_ID: u8 = 0x00; // u16
const COMMAND: u8 = 0x04; // u16
const HEADER_TYPE: u8 = 0x0e; // u8

/// UNKNOWN_DEVICES is the list of PCI devices that have been
/// identified by init but were not claimed by a device driver.
///
static UNKNOWN_DEVICES: spin::Mutex<Vec<Device>> = spin::Mutex::new(Vec::new());

/// init scans PCI busses for devices, populating DEVICES.
///
pub fn init() {
    if read_u8(0, 0, 0, HEADER_TYPE) & 0x80 == 0 {
        scan_bus(0);
        return;
    }

    let mut found = false;
    for func in 0..8 {
        if read_u16(0, 0, func, VENDOR_ID) == NONE {
            break;
        }

        found = true;
        scan_bus(func);
    }

    if !found {
        return;
    }

    for bus in 0..=255 {
        scan_bus(bus);
    }
}

/// debug iterates through the discovered but unsupported
/// PCI devices, printing each device.
///
pub fn debug() {
    let devices = UNKNOWN_DEVICES.lock();
    for device in devices.iter() {
        println!("Found unsupported {}.", device);
    }
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

fn read_u8(bus: u8, slot: u8, func: u8, field: u8) -> u8 {
    set_address(bus, slot, func, field);
    unsafe { Port::new(CONFIG_DATA + (field as u16 & 3)).read() }
}

fn read_u16(bus: u8, slot: u8, func: u8, field: u8) -> u16 {
    set_address(bus, slot, func, field);
    unsafe { Port::new(CONFIG_DATA + (field as u16 & 2)).read() }
}

fn read_u32(bus: u8, slot: u8, func: u8, field: u8) -> u32 {
    set_address(bus, slot, func, field);
    unsafe { Port::new(CONFIG_DATA + (field as u16 & 0)).read() }
}

fn write_u8(bus: u8, slot: u8, func: u8, field: u8, value: u8) {
    set_address(bus, slot, func, field);
    unsafe { Port::new(CONFIG_DATA).write(value) };
}

fn write_u16(bus: u8, slot: u8, func: u8, field: u8, value: u16) {
    set_address(bus, slot, func, field);
    unsafe { Port::new(CONFIG_DATA).write(value) };
}

fn write_u32(bus: u8, slot: u8, func: u8, field: u8, value: u32) {
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
    /// The index must be in the range [0, 6).
    ///
    pub fn bar(&self, index: usize) -> Bar {
        assert!(index < 6);
        let bar = self.base_address_registers[index];
        if bar & 1 == 0 {
            Bar::MemoryMapped {
                addr: PhysAddr::new(bar as u64 & 0xfff0),
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
fn scan_slot(bus: u8, slot: u8) {
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

    for i in 1..16 {
        registers[i] = read_u32(bus, slot, 0, (i * 4) as u8);
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

    if let Some(driver) = drivers::device_supported(&dev) {
        driver(dev);
    } else {
        UNKNOWN_DEVICES.lock().push(dev);
    }
}

// scan_bus scans a PCI bus for a recognised
// device.
//
fn scan_bus(bus: u8) {
    for slot in 0..32 {
        scan_slot(bus, slot);
    }
}
