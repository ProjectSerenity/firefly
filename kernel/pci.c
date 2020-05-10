#include "std.h"
#include "pci.h"
#include "port.h"

void pci_getVendorDevice(uint8 bus, uint8 slot, uint16* vendor, uint16* device);
void pci_getMACAddress(uint8* mac);
uint8 pci_getInterruptLine(uint8 bus, uint8 slot);
void pci_checkSlot(uint8 bus, uint8 slot);
void pci_setAddress(uint8 bus, uint8 slot, uint8 func, uint8 offset);
uint8 pci_getUint8(uint8 offset);
uint16 pci_getUint16(uint8 offset);
uint32 pci_getUint32(uint8 offset);
bool pci_hasEEPROM();
uint32 pci_readEEPROM(uint8 address);

const uint16 CONFIG_ADDRESS = 0xCF8;
const uint16 CONFIG_DATA = 0xCFC;
const uint8 PCI_INTERRUPT_LINE = 0x3C;
const uint16 REG_EEPROM = 0x0014;

bool pci_Init() {
	std_Printk("PCI init start\n");
	uint16 bus;
	uint8 slot;

	for (bus = 0; bus < 256; bus++) {
		for (slot = 0; slot < 32; slot++) {
			pci_checkSlot(bus, slot);
		}
	}

	std_Printk("PCI init end\n");
	return true;
}

void pci_setAddress(uint8 bus, uint8 slot, uint8 func, uint8 offset) {
	uint32 address;
	uint32 lbus  = (uint32)bus;
	uint32 lslot = (uint32)slot;
	uint32 lfunc = (uint32)func;

	// See https://wiki.osdev.org/PCI.
	address = (lbus << 16) | (lslot << 11) | (lfunc << 8) | (offset & 0xfc) | ((uint32)0x80000000);

	port_Out32(CONFIG_ADDRESS, address);
}

uint8 pci_getUint8(uint8 offset) {
	return port_In8(CONFIG_DATA + (offset & 3));
}

uint16 pci_getUint16(uint8 offset) {
	return port_In16(CONFIG_DATA + (offset & 2));
}

uint32 pci_getUint32(uint8 offset) {
	return port_In32(CONFIG_DATA + (offset & 0));
}

void pci_getVendorDevice(uint8 bus, uint8 slot, uint16* vendor, uint16* device) {
	pci_setAddress(bus, slot, 0, 0);
	uint32 v = pci_getUint32(0);
	*vendor = (uint16)(v & 0xffff);
	*device = (uint16)(v >> 16);
}

uint8 pci_getInterruptLine(uint8 bus, uint8 slot) {
	pci_setAddress(bus, slot, 0, PCI_INTERRUPT_LINE);
	return pci_getUint8(PCI_INTERRUPT_LINE);
}

void pci_getMACAddress(uint8* mac) {
	uint32 v = pci_readEEPROM(0);
	mac[0] = v & 0xff;
	mac[1] = v >> 8;
	v = pci_readEEPROM(1);
	mac[2] = v & 0xff;
	mac[3] = v >> 8;
	v = pci_readEEPROM(2);
	mac[4] = v & 0xff;
	mac[5] = v >> 8;
}

const uint16 vendorIntel = 0x8086;
const uint16 deviceE1000 = 0x100e;

void pci_checkSlot(uint8 bus, uint8 slot) {
	uint16 vendorID, deviceID;
	pci_getVendorDevice(bus, slot, &vendorID, &deviceID);
	if (vendorID == 0xFFFF) {
		return; // Device doesn't exist.
	}

	std_Printk("vendorID: %u16x, deviceID: %u16x\n", vendorID, deviceID);

	if (vendorID != vendorIntel || deviceID != deviceE1000) {
		return;
	}

	uint8 interruptLine = pci_getInterruptLine(bus, slot);
	(void)interruptLine;
	bool hasEEPROM = pci_hasEEPROM();
	if (!hasEEPROM) {
		std_Printk("no EEPROM detected\n");
		return;
	}

	uint8 mac[6];
	pci_getMACAddress(mac);

	std_Printk("detected MAC address: %m6 x\n", mac);
}

bool pci_hasEEPROM() {
	port_Out32(REG_EEPROM, 1);
	volatile int i;
	for (i = 0; i < 999; ++i) {
		uint32 v = port_In32(REG_EEPROM);
		if (v & 0x10) {
			return true;
		}
	}

	return false;
}

uint32 pci_readEEPROM(uint8 address) {
	uint16 data = 0;
	uint32 v = 0;

	port_Out32(REG_EEPROM, (((uint32)address) << 8) | 1);
	while ((v & (1<<4)) == 0) {
		v = port_In32(REG_EEPROM);
	}

	data = (v >> 16) & 0xffff;
	return data;
}
