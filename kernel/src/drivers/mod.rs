// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Contains modules for the implementation and initialisation of device drivers.
//!
//! ## PCI devices
//!
//! PCI [`Device`](pci::Device)s can be used to access the resources and data of a
//! PCI device. A device driver can be implemented by adding a [`PciDriverCheck`]
//! to [`PCI_DRIVERS`], then iteratively installing supported devices:
//!
//! ```
//! for device in devices.iter() {
//!     for check in drivers::PCI_DRIVERS.iter() {
//!         if let Some(install) = check(&device) {
//!             install(device);
//!             break;
//!         }
//!     }
//! }
//! ```
//!
//! ## VirtIO
//!
//! The [VirtIO](virtio) module is a partial implementation of the [Virtual I/O
//! (VirtIO) Device version 1.1 specification](https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html).
//! This is used to provide efficient access to virtual devices implemented by
//! the hypervisor.

pub mod virtio;

use pci;

/// A PciDriver takes ownership of a PCI device.
///
pub type PciDriver = fn(device: pci::Device);

/// This check determines whether a PCI
/// device driver supports the given device.
///
/// If a driver returns some device driver,
/// that driver is called to take ownership
/// of the device.
///
pub type PciDriverCheck = fn(device: &pci::Device) -> Option<PciDriver>;

/// This is the set of configured PCI device drivers.
///
/// For each PCI device discovered, each callback listed
/// here will be checked to determine whether the driver
/// supports the device. The first device that returns a
/// driver will then take ownership of the device.
///
pub const PCI_DRIVERS: &[PciDriverCheck] = &[virtio::pci_device_check];
