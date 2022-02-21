// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements VirtIO [network cards](https://docs.oasis-open.org/virtio/virtio/v1.1/virtio-v1.1.html#x1-1940001).
//!
//! A network card can be used to send and receive Ethernet frames.
//! This works using a pair of virtqueues; one for receiving frames
//! from the network and a second for sending them. Frames are
//! prepended with a header structure, used to inform the network
//! card of any advanced features being used. The header has the
//! following structure in C syntax:
//!
//! ```
//! struct virtio_net_hdr {
//! #define VIRTIO_NET_HDR_F_NEEDS_CSUM    1
//! #define VIRTIO_NET_HDR_F_DATA_VALID    2
//! #define VIRTIO_NET_HDR_F_RSC_INFO      4
//!         u8 flags;
//! #define VIRTIO_NET_HDR_GSO_NONE        0
//! #define VIRTIO_NET_HDR_GSO_TCPV4       1
//! #define VIRTIO_NET_HDR_GSO_UDP         3
//! #define VIRTIO_NET_HDR_GSO_TCPV6       4
//! #define VIRTIO_NET_HDR_GSO_ECN      0x80
//!         u8 gso_type;
//!         le16 hdr_len;
//!         le16 gso_size;
//!         le16 csum_start;
//!         le16 csum_offset;
//!         le16 num_buffers;
//! };
//! ```
//!
//! This implementation does not currently use any of the advanced
//! network card features, such as checksum offloading. The [`Driver`]
//! struct takes ownership of the underlying VirtIO driver and the
//! buffers we exchange with the network card.
//!
//! ## Integration with smoltcp
//!
//! We use [smoltcp](https://docs.rs/crate/smoltcp) as the foundation
//! for our network stack. However, the network subsystem abstracts
//! away most of the smoltcp-specific aspects. Instead, we implement
//! [`network::Device`].
//!
//! When not being used directly by a separate socket, there are two
//! ways in which an interface built on a VirtIO network card is driven.
//!
//! 1. Interrupts from the device advance the network stack, processing any received packets.
//! 2. A companion kernel thread advances the network stack periodically, following the interface's interval recommendations.

use crate::features::{Network, Reserved};
use crate::{transports, Buffer, InterruptStatus};
use alloc::boxed::Box;
use alloc::collections::BTreeMap;
use alloc::sync::Arc;
use alloc::vec::Vec;
use bitflags::bitflags;
use core::mem;
use interrupts::{register_irq, Irq};
use memlayout::phys_to_virt_addr;
use multitasking::scheduler;
use multitasking::thread::{current_global_thread_id, Thread, ThreadId};
use network::{add_interface, InterfaceHandle};
use physmem::{allocate_frame, deallocate_frame};
use serial::println;
use smoltcp::phy::{DeviceCapabilities, Medium};
use smoltcp::wire::EthernetAddress;
use spin::Mutex;
use x86_64::instructions::interrupts::without_interrupts;
use x86_64::structures::idt::InterruptStackFrame;
use x86_64::structures::paging::frame::PhysFrame;
use x86_64::structures::paging::{PageSize, Size4KiB};
use x86_64::PhysAddr;

// These constants are used to ensure we
// always receive packets from Virtqueue
// 0 and send them from Virtqueue 1.
//
const RECV_VIRTQUEUE: u16 = 0;
const SEND_VIRTQUEUE: u16 = 1;

// PACKET_LEN_MAX is the maximum size of
// a buffer used to send or receive
// packets from the device. Note that
// this includes the 12-byte header
// used by the VirtIO network driver.
//
// This allows us to achieve an MTU of
// 2036 bytes with single buffers. To
// utilise a larger MTU, we would need
// to send the packets over more than
// one buffer.
//
const PACKET_LEN_MAX: usize = 2048;

/// InterfaceDriver contains an interface handle and
/// a driver handle, allowing us to access both the
/// driver's internals and the external interface.
///
struct InterfaceDriver {
    driver: Arc<Mutex<Driver>>,
    handle: InterfaceHandle,
}

/// INTERFACES maps IRQs to the drivers that use them.
///
/// When we receive interrupts, we poll the corresponding
/// interface.
///
static INTERFACES: Mutex<[Option<InterfaceDriver>; 16]> = {
    const NONE: Option<InterfaceDriver> = Option::None;
    Mutex::new([NONE; 16])
};

/// interrupt_handler receives interrupts for
/// PCI virtio devices.
///
fn interrupt_handler(_stack_frame: InterruptStackFrame, irq: Irq) {
    let int = INTERFACES.lock();
    if let Some(iface_driver) = &int[irq.as_u8() as usize] {
        let mut dev = iface_driver.driver.lock();
        if !dev
            .driver
            .interrupt_status()
            .contains(InterruptStatus::QUEUE_INTERRUPT)
        {
            // TODO: Handle configuration changes.
            irq.acknowledge();
            return;
        }

        // Return any used send buffers to the
        // queue.
        dev.reclaim_send_buffers();

        // Drop our mutex lock to unlock
        // the driver.
        drop(dev);

        // Poll the interface so it picks
        // up any received packets.
        iface_driver.handle.poll();
    }

    irq.acknowledge();
}

/// INTERFACE_HANDLES maps helper thread ids to the
/// interface handles they use to perform background
/// network management.
///
static INTERFACE_HANDLES: Mutex<BTreeMap<ThreadId, InterfaceHandle>> = Mutex::new(BTreeMap::new());

/// network_entry_point is an entry point used by a
/// network management thread to ensure an interface
/// continues to process network events.
///
fn network_entry_point() -> ! {
    let global_thread_id = current_global_thread_id();
    let iface_handle = &INTERFACE_HANDLES.lock()[&global_thread_id];
    loop {
        let wait = without_interrupts(|| iface_handle.poll());
        scheduler::sleep(wait);
    }
}

/// A `Driver` represents a virtio network card, which can
/// be used to send and receive packets.
///
pub struct Driver {
    // driver is the underlying virtio generic driver.
    driver: crate::Driver,

    // mac is the device's MAC address.
    mac: EthernetAddress,

    // recv_buffers is the list of pre-allocated
    // buffers we use to receive packets. Note that
    // we only keep this list so we can deallocate
    // them if the driver is dropped. We don't use
    // this vector during the driver's life like we
    // do with send_buffers.
    recv_buffers: Vec<PhysAddr>,

    // send_buffers is the list of pre-allocated
    // buffers we've prepared for sending packets.
    send_buffers: Vec<PhysAddr>,

    // mtu is the path MTU.
    mtu: u16,
}

impl Driver {
    /// mac_address returns the device's MAC address.
    ///
    pub fn mac_address(&self) -> EthernetAddress {
        self.mac
    }

    /// reclaim_send_buffers retrieves any returned
    /// buffers from the send queue and adds them to
    /// the list of send buffers.
    ///
    pub fn reclaim_send_buffers(&mut self) {
        loop {
            match self.driver.recv(SEND_VIRTQUEUE) {
                None => return,
                Some(bufs) => {
                    for buf in bufs.buffers.iter() {
                        let addr = match buf {
                            Buffer::DeviceCanRead { addr, len: _len } => *addr,
                            _ => panic!("invalid buffer from send queue"),
                        };

                        self.send_buffers.push(addr);
                    }
                }
            }
        }
    }
}

impl Drop for Driver {
    fn drop(&mut self) {
        // Start by resetting the device in case
        // that leads to us receiving more send
        // buffers.
        self.driver.reset();

        // As documented earlier, we use the fact that
        // PACKET_LEN_MAX is exactly half a frame, so
        // we have two buffers from each allocated
        // frame.
        //
        // What we do here is deallocate the buffers
        // that start on a frame boundary. We can safely
        // ignore those not on a frame boundary.

        self.reclaim_send_buffers();

        // De-allocate the send buffers.
        for addr in self.send_buffers.iter() {
            if let Ok(frame) = PhysFrame::from_start_address(*addr) {
                unsafe { deallocate_frame(frame) };
            }
        }

        // De-allocate the receive buffers.
        for addr in self.recv_buffers.iter() {
            if let Ok(frame) = PhysFrame::from_start_address(*addr) {
                unsafe { deallocate_frame(frame) };
            }
        }
    }
}

/// A `Device` contains a reference to a
/// [`Driver`], which can send/receive packets.
///
/// `Device` wraps a driver so we can manage access to
/// the driver, creating additional references for
/// use by `RecvBuffer` and `SendBuffer`.
///
pub struct Device {
    driver: Arc<Mutex<Driver>>,
}

impl network::Device for Device {
    /// Called to check whether the device has received any
    /// packets. If so, the next available packet buffer is
    /// returned as a pair of physical address and buffer
    /// length. If not, `None` is returned instead.
    ///
    fn recv_packet(&mut self) -> Option<(PhysAddr, usize)> {
        without_interrupts(|| {
            let mut dev = self.driver.lock();
            match dev.driver.recv(RECV_VIRTQUEUE) {
                None => None,
                Some(buf) => {
                    debug_assert!(buf.buffers.len() == 1);
                    let len = buf.written;
                    let recv_addr = match buf.buffers[0] {
                        Buffer::DeviceCanWrite { addr, .. } => addr,
                        _ => panic!("invalid buffer type returned by device"),
                    };

                    // Process and strip the VirtIO network
                    // header. We don't use any advanced
                    // features yet, so we just ignore the
                    // header for now.
                    let offset = mem::size_of::<Header>();
                    let addr = recv_addr + offset;
                    let len = len - offset;

                    Some((addr, len))
                }
            }
        })
    }

    /// After a device returns a packet buffer from `recv_packet`,
    /// the buffer is returned to the device by calling
    /// `reclaim_recv_buffer`.
    ///
    fn reclaim_recv_buffer(&mut self, addr: PhysAddr, _len: usize) {
        // Return the used buffer to the device
        // so it can use it to receive a future
        // packet.
        without_interrupts(|| {
            let mut dev = self.driver.lock();
            let len = PACKET_LEN_MAX;

            dev.driver
                .send(RECV_VIRTQUEUE, &[Buffer::DeviceCanWrite { addr, len }])
                .expect("failed to return receive buffer to device");
            dev.driver.notify(RECV_VIRTQUEUE);
        });
    }

    /// Called when the interface wishes to send a packet of the
    /// given length.
    ///
    fn get_send_buffer(&mut self, len: usize) -> Result<PhysAddr, smoltcp::Error> {
        // Check that the buffer will have
        // enough space for the requested length,
        // plus the virtio network header.
        let offset = mem::size_of::<Header>();
        if len > (PACKET_LEN_MAX - offset) {
            return Err(smoltcp::Error::Truncated);
        }

        // Retrieve our send buffer.
        let addr = match self.driver.lock().send_buffers.pop() {
            Some(addr) => addr,
            None => {
                println!("warn: network out of send buffers on send.");
                return Err(smoltcp::Error::Exhausted);
            }
        };

        // Leave space at the front for the virtio
        // network header.
        Ok(addr + offset)
    }

    /// Called to send a packet buffer that was returned by a call
    /// to `get_send_buffer`.
    ///
    fn send_packet(&mut self, addr: PhysAddr, len: usize) -> Result<(), smoltcp::Error> {
        // Go back to where the header starts,
        // before the packet contents.
        let offset = mem::size_of::<Header>();
        let addr = addr - offset;
        let virt_addr = phys_to_virt_addr(addr);

        // Prepend the buffer with the virtio
        // network header. We don't use any of
        // the advanced features yet, so we can
        // populate the fields with zeros.
        let mut header = unsafe { *virt_addr.as_mut_ptr::<Header>() };
        header.flags = HeaderFlags::NONE.bits();
        header.gso_type = GsoType::None as u8;
        header.header_len = 0u16.to_le();
        header.gso_size = 0u16.to_le();
        header.checksum_start = 0u16.to_le();
        header.checksum_offset = 0u16.to_le();
        header.num_buffers = 0u16.to_le();

        // Send the packet.
        without_interrupts(|| {
            let mut dev = self.driver.lock();
            let len = len + offset;

            dev.driver
                .send(SEND_VIRTQUEUE, &[Buffer::DeviceCanRead { addr, len }])
                .expect("failed to send packet buffer to device");
            dev.driver.notify(SEND_VIRTQUEUE);
        });

        Ok(())
    }

    /// Describes this device's capabilities.
    ///
    fn capabilities(&self) -> DeviceCapabilities {
        without_interrupts(|| {
            let dev = self.driver.lock();
            let mut caps = DeviceCapabilities::default();
            caps.medium = Medium::Ethernet;
            caps.max_transmission_unit = dev.mtu as usize;
            caps
        })
    }
}

/// Header represents the header data that must preceed
/// each buffer sent to the device.
///
#[repr(C, packed)]
#[derive(Clone, Copy)]
struct Header {
    flags: u8,
    gso_type: u8,
    header_len: u16,
    gso_size: u16,
    checksum_start: u16,
    checksum_offset: u16,
    num_buffers: u16,
}

bitflags! {
    /// HeaderFlags describes the flags that can be
    /// passed in the Header type's flags field.
    ///
    struct HeaderFlags: u8 {
        /// NONE indicates no flags.
        const NONE = 0;

        /// NEEDS_CHECKSUM indicates that the device
        /// should populate a checksum field in the
        /// buffer.
        const NEEDS_CHECKSUM = 1;

        /// DATA_VALID indicates that the device has
        /// checked the received packet's checksum
        /// and confirms that it is valid.
        const DATA_VALID = 2;

        /// RSC_INFO indicates that the device has
        /// included information on the number of
        /// coalesced TCP segments.
        const RSC_INFO = 4;
    }
}

/// GsoType represents the generic segmentation offload
/// types used in the gos_type field of the Header type.
///
#[derive(Clone, Copy, Debug)]
enum GsoType {
    None = 0,
    _TcpV4 = 1,
    _Udp = 3,
    _TcpV6 = 4,
    _Ecn = 0x80,
}

/// Config is a helper type that gives the layout
/// layout of the device-specific config type for
/// network card devices, as defined in section
/// 5.1.4:
///
/// ```
/// struct virtio_net_config {
///     u8 mac[6];
///     le16 status;
///     le16 max_virtqueue_pairs;
///     le16 mtu;
/// }
/// ```
///
#[repr(C, packed)]
#[derive(Clone, Copy)]
struct Config {
    mac: [u8; 6],
    status: u16,
    max_virtqueue_pairs: u16,
    mtu: u16,
}

/// Takes ownership of the given PCI device to reset and configure
/// a virtio network card.
///
pub fn install_pci_device(device: pci::Device) {
    let transport = match transports::pci::Transport::new(device) {
        Err(err) => {
            println!("Ignoring invalid device: {:?}.", err);
            return;
        }
        Ok(transport) => Arc::new(transport),
    };

    let must_features = Reserved::VERSION_1.bits() | Network::MAC.bits();
    let like_features = Reserved::RING_EVENT_IDX.bits();
    let mut driver = match crate::Driver::new(transport, must_features, like_features, 2) {
        Ok(driver) => driver,
        Err(err) => {
            println!("Failed to initialise network card: {:?}.", err);
            return;
        }
    };

    // Determine the MAC address.
    let mac = EthernetAddress([
        driver.read_device_config_u8(0),
        driver.read_device_config_u8(1),
        driver.read_device_config_u8(2),
        driver.read_device_config_u8(3),
        driver.read_device_config_u8(4),
        driver.read_device_config_u8(5),
    ]);

    let net_features = Network::from_bits_truncate(driver.features());
    let mtu = if net_features.contains(Network::MTU) {
        u16::from_le_bytes([
            driver.read_device_config_u8(10),
            driver.read_device_config_u8(11),
        ])
    } else {
        // Educated guess.
        1500
    };

    // Prepare the send and receive virtqueues.
    let send_queue_len = driver.num_descriptors(SEND_VIRTQUEUE);
    let recv_queue_len = driver.num_descriptors(RECV_VIRTQUEUE);

    // We make use of the fact that the max packet
    // size we use is exactly half the page size.
    // This means we can allocate a single physical
    // frame and use it for two packet buffers, each
    // of which is internally contiguous. This means
    // we don't need any frame to be contiguous with
    // any other, making things *much* easier for
    // the allocator.
    assert!(PACKET_LEN_MAX * 2 == Size4KiB::SIZE as usize);
    let mut send_buffers = Vec::new();
    while send_buffers.len() < send_queue_len {
        let frame = allocate_frame().expect("failed to allocate for device send buffer");
        send_buffers.push(frame.start_address());
        send_buffers.push(frame.start_address() + PACKET_LEN_MAX);
    }

    let mut recv_buffers = Vec::new();
    while recv_buffers.len() < recv_queue_len {
        let frame = allocate_frame().expect("failed to allocate for device recv buffer");
        recv_buffers.push(frame.start_address());
        recv_buffers.push(frame.start_address() + PACKET_LEN_MAX);
    }

    // Send the receive buffers to the device.
    for buffer in recv_buffers.iter() {
        let addr = *buffer;
        let len = PACKET_LEN_MAX;
        driver
            .send(RECV_VIRTQUEUE, &[Buffer::DeviceCanWrite { addr, len }])
            .expect("failed to send receive buffer to device");
    }
    driver.notify(RECV_VIRTQUEUE);

    // Prepare the network driver.
    let irq = driver.irq();
    let driver = Driver {
        driver,
        mac,
        recv_buffers,
        send_buffers,
        mtu,
    };

    let driver = Arc::new(Mutex::new(driver));
    let device = Device {
        driver: driver.clone(),
    };

    // Pass the device to the network stack
    // and set up its interrupts.
    let handle = add_interface(Box::new(device), mac);
    let iface_handle = InterfaceDriver { driver, handle };

    without_interrupts(|| {
        let mut int = INTERFACES.lock();
        int[irq.as_usize()] = Some(iface_handle);
    });

    register_irq(irq, interrupt_handler);

    // Create the background thread that performs
    // network activity.
    let thread_id = Thread::create_kernel_thread(network_entry_point);
    INTERFACE_HANDLES.lock().insert(thread_id, handle);
    scheduler::resume(thread_id);
}
