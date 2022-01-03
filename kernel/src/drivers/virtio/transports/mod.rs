//! Contains modules for different Virtio [transport mechanisms](https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html#x1-990004).
//!
//! ## PCI devices
//!
//! The [PCI](pci) module provides the PCI [`Transport`](pci::Transport), which can
//! be used to reset and configure a Virtio device using a PCI [`Device`](crate::drivers::pci::Device).
//!
//! ## MMIO devices
//!
//! Virtio [MMIO devices](https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html#x1-1440002)
//! are not yet supported.

pub mod pci;
