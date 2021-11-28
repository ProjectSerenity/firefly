//! drivers handles the initialisation of devices, using the
//! driver child modules.

use crate::pci;

/// PciDeviceDriver takes ownership of a PCI device.
///
pub type PciDeviceDriver = fn(device: pci::Device);

/// device_supported is a callback called by the PCI module
/// for each PCI device discovered. If the device is supported
/// by a device driver, device_supported returns true and
/// the device is passed to register_device, so the driver
/// can take ownership.
///
pub fn device_supported(_device: &pci::Device) -> Option<PciDeviceDriver> {
    // TODO: once we have device drivers, ask them here,
    // returning their driver if so.

    None
}
