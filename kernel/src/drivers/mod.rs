// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Contains modules for the implementation and initialisation of device drivers.
//!
//! ## PCI devices
//!
//! The [PCI](pci) module provides the [`pci::init`] function to scan the set of
//! attached PCI buses for supported devices. Any supported devices are initialised
//! by a driver and put into use.
//!
//! PCI [`Device`](pci::Device)s can be used to access the resources and data of a
//! PCI device. A device driver can be implemented by adding a
//! [`DeviceDriverSupport`](pci::DeviceDriverSupport) to
//! [`DEVICE_DRIVERS`](pci::DEVICE_DRIVERS).
//!
//! [`pci::debug`] can be called after [`pci::init`] to print debug information
//! about detected devices that were not adopted by any device drivers.
//!
//! ## VirtIO
//!
//! The [VirtIO](virtio) module is a partial implementation of the [Virtual I/O
//! (VirtIO) Device version 1.1 specification](https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html).
//! This is used to provide efficient access to virtual devices implemented by
//! the hypervisor.

pub mod pci;
pub mod virtio;
