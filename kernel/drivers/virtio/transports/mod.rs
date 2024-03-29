// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Contains modules for different VirtIO [transport mechanisms](https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html#x1-990004).
//!
//! ## PCI devices
//!
//! The [PCI](pci) module provides the PCI [`Transport`](pci::Transport), which can
//! be used to reset and configure a VirtIO device using a PCI [`Device`](::pci::Device).
//!
//! ## MMIO devices
//!
//! VirtIO [MMIO devices](https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html#x1-1440002)
//! are not yet supported.

pub mod legacy_pci;
pub mod pci;
