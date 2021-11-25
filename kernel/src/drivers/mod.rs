//! drivers handles the initialisation of devices, using the
//! driver child modules.

use crate::pci;

/// device_supported is a callback called by the PCI module
/// for each PCI device discovered. If the device is supported
/// by a device driver, device_supported returns true and
/// the device is passed to register_device, so the driver
/// can take ownership.
///
pub fn device_supported(device: &pci::Device) -> bool {
    // TODO: once we have device drivers, ask them here.
    false
}

/// register_device is a callback called by the PCI module
/// for each PCI device discovered. If the device can be
/// identified as a supported device, the corresponding
/// driver is used to initialise the device.
///
pub fn register_device(device: pci::Device) {
    // TODO: once we have device drivers, initialise them here.
}
