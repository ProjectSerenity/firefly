//! pci is a basic implementation of PCI bus scanning, which will
//! detect and identify the PCI devices available.

use crate::{drivers, println};
use alloc::vec::Vec;
use core::fmt;
use x86_64::instructions::port::Port;

pub const CONFIG_ADDRESS: u16 = 0xcf8;
pub const CONFIG_DATA: u16 = 0xcfc;

pub const NONE: u16 = 0xffff;

// See https://en.wikipedia.org/wiki/PCI_configuration_space#/media/File:Pci-config-space.svg

pub const VENDOR_ID: u8 = 0x00; // u16
pub const DEVICE_ID: u8 = 0x02; // u16
pub const COMMAND: u8 = 0x04; // u16
pub const STATUS: u8 = 0x06; // u16
pub const REVISION_ID: u8 = 0x08; // u8
pub const SUBCLASS: u8 = 0x0a; // u8
pub const CLASS: u8 = 0x0b; // u8
pub const HEADER_TYPE: u8 = 0x0e; // u8
pub const SUBSYSTEM_VENDOR_ID: u8 = 0x2c; // u16
pub const SUBSYSTEM_ID: u8 = 0x2e; // u16
pub const INTERRUPT_LINE: u8 = 0x3c; // u8

pub const BAR0: u8 = 0x10; // u32
pub const BAR1: u8 = 0x14; // u32
pub const BAR2: u8 = 0x18; // u32
pub const BAR3: u8 = 0x1c; // u32
pub const BAR4: u8 = 0x20; // u32
pub const BAR5: u8 = 0x24; // u32

pub const SECONDARY_BUS: u8 = 0x19;

pub const TYPE_BRIDGE: u16 = 0x0604;

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

/// Device represents a PCI device.
///
pub struct Device {
    bus: u8,
    slot: u8,
    func: u8,

    pub vendor: u16,
    pub device: u16,
    pub devtype: u16,
    pub subsystem_vendor: u16,
    pub subsystem: u16,
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

    pub fn get_irq(&self) -> usize {
        self.read_field_u8(INTERRUPT_LINE) as usize
    }
}

impl fmt::Display for Device {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        write!(
            f,
            "PCI device with vendor={:04x}, device={:04x}, type={:04x}, subsystem vendor={:04x}, subsystem={:04x}",
            self.vendor, self.device, self.devtype, self.subsystem_vendor, self.subsystem
        )
    }
}

// read_vendor_device returns the vendor and device
// details for the given slot.
//
fn read_vendor_device(bus: u8, slot: u8) -> (u16, u16) {
    let data = read_u32(bus, slot, 0, VENDOR_ID);
    let vendor = (data & 0xffff) as u16;
    let device = (data >> 16) as u16;

    (vendor, device)
}

// read_device_type returns the device type of the
// given device.
//
fn read_device_type(bus: u8, slot: u8, func: u8) -> u16 {
    let class = read_u8(bus, slot, func, CLASS) as u16;
    let subclass = read_u8(bus, slot, func, SUBCLASS) as u16;
    (class << 8) | subclass
}

// read_subsystem returns the vendor and id for the
// subsystem details for the given slot.
//
fn read_subsystem(bus: u8, slot: u8) -> (u16, u16) {
    let data = read_u32(bus, slot, 0, SUBSYSTEM_VENDOR_ID);
    let vendor = (data & 0xffff) as u16;
    let device = (data >> 16) as u16;

    (vendor, device)
}

// scan_slot scans a PCI slot for a recognised
// device.
//
fn scan_slot(bus: u8, slot: u8) {
    let (vendor, device) = read_vendor_device(bus, slot);
    if vendor == NONE {
        // Device doesn't exist.
        return;
    }

    let devtype = read_device_type(bus, slot, 0);
    let (subsystem_vendor, subsystem) = read_subsystem(bus, slot);

    let dev = Device {
        bus,
        slot,
        func: 0,
        vendor,
        device,
        devtype,
        subsystem_vendor,
        subsystem,
    };

    if let Some(driver) = drivers::device_supported(&dev) {
        driver(dev);
    } else {
        UNKNOWN_DEVICES.lock().push(dev);
    }

    if devtype == TYPE_BRIDGE {
        let bus = read_u8(bus, slot, 0, SECONDARY_BUS);
        scan_bus(bus);
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
