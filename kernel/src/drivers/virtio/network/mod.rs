//! network implements virtio network cards.

use crate::drivers::virtio;
use crate::drivers::virtio::virtqueue;
use crate::interrupts::Irq;
use crate::memory::{phys_to_virt_addr, pmm};
use crate::multitasking::cpu_local;
use crate::multitasking::thread::ThreadId;
use crate::network::InterfaceHandle;
use crate::{println, time};
use alloc::collections::BTreeMap;
use alloc::sync::Arc;
use alloc::vec::Vec;
use bitflags::bitflags;
use core::{mem, slice};
use smoltcp::phy::{DeviceCapabilities, Medium, RxToken, TxToken};
use smoltcp::time::Instant;
use smoltcp::wire::EthernetAddress;
use x86_64::instructions::interrupts;
use x86_64::structures::idt::InterruptStackFrame;
use x86_64::structures::paging::frame::PhysFrame;
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
// used by the Virtio network driver.
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
    driver: Arc<spin::Mutex<Driver>>,
    handle: InterfaceHandle,
}

/// INTERFACES maps IRQs to the drivers that use them.
///
/// When we receive interrupts, we poll the corresponding
/// interface.
///
static INTERFACES: spin::Mutex<[Option<InterfaceDriver>; 16]> = {
    const NONE: Option<InterfaceDriver> = Option::None;
    spin::Mutex::new([NONE; 16])
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
            .contains(virtio::InterruptStatus::QUEUE_INTERRUPT)
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
        mem::drop(dev);

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
static INTERFACE_HANDLES: spin::Mutex<BTreeMap<ThreadId, InterfaceHandle>> =
    spin::Mutex::new(BTreeMap::new());

/// network_entry_point is an entry point used by a
/// network management thread to ensure an interface
/// continues to process network events.
///
fn network_entry_point() -> ! {
    let thread_id = cpu_local::current_thread().thread_id();
    let iface_handle = &INTERFACE_HANDLES.lock()[&thread_id];
    loop {
        let wait = interrupts::without_interrupts(|| iface_handle.poll());

        //println!("Waiting for {:?}.", wait);
        time::sleep(wait);
    }
}

/// Driver represents a virtio network card, which can
/// be used to send and receive packets.
///
pub struct Driver {
    // driver is the underlying virtio generic driver.
    driver: virtio::Driver,

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
                            virtqueue::Buffer::DeviceCanRead { addr, len: _len } => *addr,
                            _ => panic!("invalid buffer from send queue"),
                        };

                        self.send_buffers.push(addr);
                        println!("Send buffer reclaimed.");
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
            match PhysFrame::from_start_address(*addr) {
                Ok(frame) => {
                    unsafe { pmm::deallocate_frame(frame) };
                }
                Err(_e) => {}
            }
        }

        // De-allocate the receive buffers.
        for addr in self.recv_buffers.iter() {
            match PhysFrame::from_start_address(*addr) {
                Ok(frame) => {
                    unsafe { pmm::deallocate_frame(frame) };
                }
                Err(_e) => {}
            }
        }
    }
}

/// RecvBuffer is returned to the interface by our
/// driver when we receive a packet for it to
/// process. The buffer is a recv buffer we just
/// received from the device. The buffer can only
/// be used once (enforced by the fact it's a method
/// on self, not &self).
///
pub struct RecvBuffer {
    addr: PhysAddr,
    len: usize,
    driver: Arc<spin::Mutex<Driver>>,
}

impl<'a> RxToken for RecvBuffer {
    fn consume<R, F>(self, _timestamp: Instant, f: F) -> smoltcp::Result<R>
    where
        F: FnOnce(&mut [u8]) -> smoltcp::Result<R>,
    {
        // Process and strip the virtio network header.
        // We don't use any advanced features yet, so
        // the virtio network header fields can be ignored.
        let offset = mem::size_of::<Header>();

        // Pass our buffer to the callback to
        // process the packet.
        let virt_addr = phys_to_virt_addr(self.addr) + offset;
        let len = self.len - offset;
        let mut buf = unsafe { slice::from_raw_parts_mut(virt_addr.as_mut_ptr(), len) };
        let ret = f(&mut buf);

        // Return the used buffer to the device
        // so it can use it to receive a future
        // packet.
        interrupts::without_interrupts(|| {
            let mut dev = self.driver.lock();
            let addr = self.addr;
            let len = PACKET_LEN_MAX;

            dev.driver
                .send(
                    RECV_VIRTQUEUE,
                    &[virtqueue::Buffer::DeviceCanWrite { addr, len }],
                )
                .expect("failed to return receive buffer to device");
            dev.driver.notify(RECV_VIRTQUEUE);
        });

        ret
    }
}

/// SendBuffer is returned to the interface by our
/// driver when the interface wants to send a
/// packet. The buffer is a send buffer we've made
/// available and won't be used elsewhere. The
/// buffer can only be used once (enforced by the
/// fact it's a method on self, not &self).
///
pub struct SendBuffer {
    driver: Arc<spin::Mutex<Driver>>,
}

impl<'a> TxToken for SendBuffer {
    fn consume<R, F>(self, _timestamp: Instant, len: usize, f: F) -> smoltcp::Result<R>
    where
        F: FnOnce(&mut [u8]) -> smoltcp::Result<R>,
    {
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

        let virt_addr = virt_addr + offset;
        let mut buf = unsafe { slice::from_raw_parts_mut(virt_addr.as_mut_ptr(), len) };
        let ret = f(&mut buf)?;

        // Send the packet.
        interrupts::without_interrupts(|| {
            let mut dev = self.driver.lock();
            let len = len + offset;

            dev.driver
                .send(
                    SEND_VIRTQUEUE,
                    &[virtqueue::Buffer::DeviceCanRead { addr, len }],
                )
                .expect("failed to send packet buffer to device");
            dev.driver.notify(SEND_VIRTQUEUE);
        });

        Ok(ret)
    }
}

/// Device wraps a driver so we can manage access to
/// the driver, creating additional references for
/// use by RecvBuffer and SendBuffer.
///
pub struct Device {
    driver: Arc<spin::Mutex<Driver>>,
}

impl<'a> smoltcp::phy::Device<'a> for Device {
    type RxToken = RecvBuffer;
    type TxToken = SendBuffer;

    /// receive is called by the interface to check
    /// whether we have any packets to receive. We
    /// pop off the next packet from the receive
    /// queue and return it, or return None if not.
    ///
    fn receive(&'a mut self) -> Option<(Self::RxToken, Self::TxToken)> {
        interrupts::without_interrupts(|| {
            let mut dev = self.driver.lock();
            match dev.driver.recv(RECV_VIRTQUEUE) {
                None => None,
                Some(buf) => {
                    debug_assert!(buf.buffers.len() == 1);
                    let len = buf.written;
                    let recv_addr = match buf.buffers[0] {
                        virtqueue::Buffer::DeviceCanWrite { addr, .. } => addr,
                        _ => panic!("invalid buffer type returned by device"),
                    };

                    Some((
                        RecvBuffer {
                            addr: recv_addr,
                            len,
                            driver: self.driver.clone(),
                        },
                        SendBuffer {
                            driver: self.driver.clone(),
                        },
                    ))
                }
            }
        })
    }

    /// transmit is called by the interface when
    /// it wants to send a packet.
    fn transmit(&'a mut self) -> Option<Self::TxToken> {
        Some(SendBuffer {
            driver: self.driver.clone(),
        })
    }

    /// capabilities describes this deivce's
    /// capabilities.
    ///
    fn capabilities(&self) -> DeviceCapabilities {
        interrupts::without_interrupts(|| {
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
    TcpV4 = 1,
    Udp = 3,
    TcpV6 = 4,
    Ecn = 0x80,
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
