//! network implements virtio network cards.

use crate::drivers::virtio;
use crate::drivers::virtio::virtqueue;
use crate::memory::pmm;
use crate::println;
use alloc::vec::Vec;
use bitflags::bitflags;
use smoltcp::wire::EthernetAddress;
use x86_64::structures::paging::frame::PhysFrame;
use x86_64::PhysAddr;

// These constants are used to ensure we
// always receive packets from Virtqueue
// 0 and send them from Virtqueue 1.
//
const RECV_VIRTQUEUE: u16 = 0;
const SEND_VIRTQUEUE: u16 = 1;

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
