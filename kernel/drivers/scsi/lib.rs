// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Provides types and functions for the SCSI driver
//! protocol.

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
#![deny(macro_use_extern_crate)]
#![deny(missing_abi)]
#![forbid(unsafe_code)]
#![deny(unused_crate_dependencies)]

mod sense;

pub use sense::{parse_sense_data, AdditionalSense, SenseKey};

/// A SCSI operation code.
///
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub enum OperationCode {
    TestUnitReady = 0x00,
    Inquiry = 0x12,
    ReadCapacity10 = 0x25,
    ReportLuns = 0xa0,
    Read16 = 0x88,
    Write16 = 0x8a,
    ServiceActionIn = 0x9e,
}

/// A SCSI SERVICE ACTION.
///
pub enum ServiceAction {
    ReadCapacity16 = 0x10,
}

/// Represents a SCSI command descriptor block structure.
///
pub trait CommandDescriptorBlock {
    /// Copy the command descriptor block into the given
    /// buffer.
    ///
    fn serialise(&self, buffer: &mut [u8]);
}

/// The TEST UNIT READY command.
///
pub struct TestUnitReady;

impl TestUnitReady {
    const CDB_LEN: usize = 6;

    pub fn new() -> Self {
        TestUnitReady
    }
}

impl Default for TestUnitReady {
    fn default() -> Self {
        Self::new()
    }
}

impl CommandDescriptorBlock for TestUnitReady {
    fn serialise(&self, buffer: &mut [u8]) {
        if buffer.len() < Self::CDB_LEN {
            panic!(
                "command descriptor block is {} bytes long, need {}",
                buffer.len(),
                Self::CDB_LEN
            );
        }

        buffer[0] = OperationCode::TestUnitReady as u8; // Operation code.
        buffer[1] = 0x00; // Reserved.
        buffer[2] = 0x00; // Reserved.
        buffer[3] = 0x00; // Reserved.
        buffer[4] = 0x00; // Reserved.
        buffer[5] = 0x00; // Control.
    }
}

/// The PAGE CODE field in an INQUIRY
/// command.
///
#[allow(dead_code)]
pub enum PageCode {
    SupportedVPDPages = 0x00,
    UnitSerialNumber = 0x80,
    DeviceIdentification = 0x83,
    SoftwareInterfaceIdentification = 0x84,
    ManagementNetworkAddresses = 0x85,
    ExtendedInquiryData = 0x86,
    ModePagePolicy = 0x87,
    SCSIPorts = 0x88,
    PowerCondition = 0x8a,
    DeviceConstituents = 0x8b,
    CFAProfileInformation = 0x8c,
    PowerConsumption = 0x8d,
    BlockLimits = 0xb0,
    BlockDeviceCharacteristics = 0xb1,
    LogicalBlockProvisioning = 0xb2,
    Referrals = 0xb3,
    SupportedBlockLengthsAndProtectionTypes = 0xb4,
    BlockDeviceCharacteristicsExtension = 0xb5,
    ZonedBlockDeviceCharacteristics = 0xb6,
    BlockLimitsExtension = 0xb7,
    FirmwareNumbersPage = 0xc0,
    DataCodePage = 0xc1,
    JumperSettingsPage = 0xc2,
    DeviceBehaviorPage = 0xc3,
}

/// A peripheral device type, as returned
/// in the standard INQUIRY data.
///
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub enum PeripheralDeviceType {
    DirectAccessBlockDevice = 0x00,
    SequentialAccessDevice = 0x01,
    PrinterDevice = 0x02,
    ProcessorDevice = 0x03,
    WriteOnceDevice = 0x04,
    CdDvdDevice = 0x05,
    ScannerDevice = 0x06,
    OpticalMemoryDevice = 0x07,
    MediumChangerDevice = 0x08,
    CommunicationsDevice = 0x09,
    // 0x0a - 0x0b: obsolete.
    StorageArrayControllerDevice = 0x0c,
    EnclosureServicesDevice = 0x0d,
    SimplifiedDirectAccessDevice = 0x0e,
    OpticalCardReaderWriterDevice = 0x0f,
    BridgeControllerCommands = 0x10,
    ObjectBasedStorageDevice = 0x11,
    AutomationDriveInterface = 0x12,
    // 0x13 - 0x1d: reserved.
    WellKnownLogicalUnit = 0x1e,
    Unknown = 0x1f,
}

impl PeripheralDeviceType {
    pub fn from_u8(n: u8) -> Option<Self> {
        match n & 0b11111 {
            0x00 => Some(PeripheralDeviceType::DirectAccessBlockDevice),
            0x01 => Some(PeripheralDeviceType::SequentialAccessDevice),
            0x02 => Some(PeripheralDeviceType::PrinterDevice),
            0x03 => Some(PeripheralDeviceType::ProcessorDevice),
            0x04 => Some(PeripheralDeviceType::WriteOnceDevice),
            0x05 => Some(PeripheralDeviceType::CdDvdDevice),
            0x06 => Some(PeripheralDeviceType::ScannerDevice),
            0x07 => Some(PeripheralDeviceType::OpticalMemoryDevice),
            0x08 => Some(PeripheralDeviceType::MediumChangerDevice),
            0x09 => Some(PeripheralDeviceType::CommunicationsDevice),
            0x0c => Some(PeripheralDeviceType::StorageArrayControllerDevice),
            0x0d => Some(PeripheralDeviceType::EnclosureServicesDevice),
            0x0e => Some(PeripheralDeviceType::SimplifiedDirectAccessDevice),
            0x0f => Some(PeripheralDeviceType::OpticalCardReaderWriterDevice),
            0x10 => Some(PeripheralDeviceType::BridgeControllerCommands),
            0x11 => Some(PeripheralDeviceType::ObjectBasedStorageDevice),
            0x12 => Some(PeripheralDeviceType::AutomationDriveInterface),
            0x1e => Some(PeripheralDeviceType::WellKnownLogicalUnit),
            0x1f => Some(PeripheralDeviceType::Unknown),
            _ => None,
        }
    }
}

/// The INQUIRY command.
///
pub struct Inquiry {
    page_code: Option<PageCode>,
    allocation_length: u16,
}

impl Inquiry {
    const CDB_LEN: usize = 6;

    pub fn new(page_code: Option<PageCode>, allocation_length: u16) -> Self {
        Inquiry {
            page_code,
            allocation_length,
        }
    }
}

impl CommandDescriptorBlock for Inquiry {
    fn serialise(&self, buffer: &mut [u8]) {
        if buffer.len() < Self::CDB_LEN {
            panic!(
                "command descriptor block is {} bytes long, need {}",
                buffer.len(),
                Self::CDB_LEN
            );
        }

        buffer[0] = OperationCode::Inquiry as u8; // Operation code.
        if let Some(page_code) = &self.page_code {
            buffer[1] = 1; // Reserved, EVPD bit.
            buffer[2] = *page_code as u8; // Page code.
        } else {
            buffer[1] = 0; // Reserved, EVPD bit.
            buffer[2] = 0; // Page code.
        }
        buffer[3..(3 + 2)].copy_from_slice(&self.allocation_length.to_be_bytes());
        buffer[5] = 0x00; // Control.
    }
}

/// The READ CAPACITY(10) command.
///
pub struct ReadCapacity10;

impl ReadCapacity10 {
    const CDB_LEN: usize = 10;

    pub fn new() -> Self {
        ReadCapacity10
    }
}

impl Default for ReadCapacity10 {
    fn default() -> Self {
        Self::new()
    }
}

impl CommandDescriptorBlock for ReadCapacity10 {
    fn serialise(&self, buffer: &mut [u8]) {
        if buffer.len() < Self::CDB_LEN {
            panic!(
                "command descriptor block is {} bytes long, need {}",
                buffer.len(),
                Self::CDB_LEN
            );
        }

        buffer[0] = OperationCode::ReadCapacity10 as u8; // Operation code.
        buffer[1] = 0x00; // Reserved, obsolete.
        buffer[2] = 0x00; // Logical block address (obsolete).
        buffer[3] = 0x00; // Logical block address (obsolete).
        buffer[4] = 0x00; // Logical block address (obsolete).
        buffer[5] = 0x00; // Logical block address (obsolete).
        buffer[6] = 0x00; // Reserved.
        buffer[7] = 0x00; // Reserved.
        buffer[8] = 0x00; // Reserved, PMI.
        buffer[9] = 0x00; // Control.
    }
}

/// The SELECT REPORT field in a REPORT LUNS
/// command.
///
#[allow(dead_code)]
#[derive(Clone, Copy, PartialEq, Eq, Debug)]
pub enum SelectReport {
    /// All logical units with the following addressing
    /// methods:
    ///
    /// 1. Logical unit addressing method,
    /// 2. Peripheral device addressing method,
    /// 3. Flat space addressing method.
    Limited = 0x00,

    /// Well-known logical units.
    WellKnown = 0x01,

    /// All logical units.
    All = 0x02,
}

/// The REPORT LUNS command.
///
pub struct ReportLuns {
    select_report: SelectReport,
    allocation_length: u32,
}

impl ReportLuns {
    const CDB_LEN: usize = 12;

    pub fn new(select_report: SelectReport, allocation_length: u32) -> Self {
        ReportLuns {
            select_report,
            allocation_length,
        }
    }
}

impl CommandDescriptorBlock for ReportLuns {
    fn serialise(&self, buffer: &mut [u8]) {
        if buffer.len() < Self::CDB_LEN {
            panic!(
                "command descriptor block is {} bytes long, need {}",
                buffer.len(),
                Self::CDB_LEN
            );
        }

        buffer[0] = OperationCode::ReportLuns as u8; // Operation code.
        buffer[1] = 0x00; // Reserved.
        buffer[2] = self.select_report as u8;
        buffer[3] = 0x00; // Reserved.
        buffer[4] = 0x00; // Reserved.
        buffer[5] = 0x00; // Reserved.
        buffer[6..(6 + 4)].copy_from_slice(&self.allocation_length.to_be_bytes());
        buffer[10] = 0x00; // Reserved.
        buffer[11] = 0x00; // Control.
    }
}

/// The READ(16) command.
///
pub struct Read16 {
    logical_block_address: u64,
    transfer_length: u32,
}

impl Read16 {
    const CDB_LEN: usize = 16;

    pub fn new(logical_block_address: u64, transfer_length: u32) -> Self {
        Read16 {
            logical_block_address,
            transfer_length,
        }
    }
}

impl CommandDescriptorBlock for Read16 {
    fn serialise(&self, buffer: &mut [u8]) {
        if buffer.len() < Self::CDB_LEN {
            panic!(
                "command descriptor block is {} bytes long, need {}",
                buffer.len(),
                Self::CDB_LEN
            );
        }

        buffer[0] = OperationCode::Read16 as u8; // Operation code.
        buffer[1] = 0x00; // Rdprotect, DPO, FUA, RARC, obsolete, DLD2.
        buffer[2..(2 + 8)].copy_from_slice(&self.logical_block_address.to_be_bytes());
        buffer[10..(10 + 4)].copy_from_slice(&self.transfer_length.to_be_bytes());
        buffer[14] = 0x00; // DLD1, DLD0, group number.
        buffer[15] = 0x00; // Control.
    }
}

/// The WRITE(16) command.
///
pub struct Write16 {
    logical_block_address: u64,
    transfer_length: u32,
}

impl Write16 {
    const CDB_LEN: usize = 16;

    pub fn new(logical_block_address: u64, transfer_length: u32) -> Self {
        Write16 {
            logical_block_address,
            transfer_length,
        }
    }
}

impl CommandDescriptorBlock for Write16 {
    fn serialise(&self, buffer: &mut [u8]) {
        if buffer.len() < Self::CDB_LEN {
            panic!(
                "command descriptor block is {} bytes long, need {}",
                buffer.len(),
                Self::CDB_LEN
            );
        }

        buffer[0] = OperationCode::Write16 as u8; // Operation code.
        buffer[1] = 0x00; // Wrprotect, DPO, FUA, reserved, obsolete, DLD2.
        buffer[2..(2 + 8)].copy_from_slice(&self.logical_block_address.to_be_bytes());
        buffer[10..(10 + 4)].copy_from_slice(&self.transfer_length.to_be_bytes());
        buffer[14] = 0x00; // DLD1, DLD0, group number.
        buffer[15] = 0x00; // Control.
    }
}

/// The READ CAPACITY(16) command.
///
pub struct ReadCapacity16 {
    allocation_length: u32,
}

impl ReadCapacity16 {
    const CDB_LEN: usize = 16;

    pub fn new(allocation_length: u32) -> Self {
        ReadCapacity16 { allocation_length }
    }
}

impl CommandDescriptorBlock for ReadCapacity16 {
    fn serialise(&self, buffer: &mut [u8]) {
        if buffer.len() < Self::CDB_LEN {
            panic!(
                "command descriptor block is {} bytes long, need {}",
                buffer.len(),
                Self::CDB_LEN
            );
        }

        buffer[0] = OperationCode::ServiceActionIn as u8; // Operation code.
        buffer[1] = ServiceAction::ReadCapacity16 as u8; // Reserved, service action.
        buffer[2] = 0x00; // Logical block address (obsolete).
        buffer[3] = 0x00; // Logical block address (obsolete).
        buffer[4] = 0x00; // Logical block address (obsolete).
        buffer[5] = 0x00; // Logical block address (obsolete).
        buffer[6] = 0x00; // Logical block address (obsolete).
        buffer[7] = 0x00; // Logical block address (obsolete).
        buffer[8] = 0x00; // Logical block address (obsolete).
        buffer[9] = 0x00; // Logical block address (obsolete).
        buffer[10..(10 + 4)].copy_from_slice(&self.allocation_length.to_be_bytes());
        buffer[14] = 0x00; // Reserved, PMI.
        buffer[15] = 0x00; // Control.
    }
}
