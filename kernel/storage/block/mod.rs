// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements block storage devices for the kernel.

use alloc::boxed::Box;
use alloc::vec::Vec;
use bitflags::bitflags;
use spin::{lock, Mutex};
use x86_64::instructions::interrupts::without_interrupts;

/// The list of block storage devices.
///
static DEVICES: Mutex<Vec<Box<dyn Device + Send>>> = Mutex::new(Vec::new());

/// Registers a new block storage device.
///
pub fn add_device(device: Box<dyn Device + Send>) {
    without_interrupts(|| {
        let mut devices = lock!(DEVICES);
        devices.push(device);
    });
}

/// Iterate through the set of block storage devices,
/// calling f on each device.
///
pub fn iter<F>(f: F)
where
    F: FnOnce(&mut Box<dyn Device + Send>) + Copy,
{
    let mut devices = without_interrupts(|| lock!(DEVICES));
    for dev in devices.iter_mut() {
        f(dev);
    }
}

bitflags! {
    /// The list of operations that can be supported by
    /// a device.
    ///
    pub struct Operations: usize {
        /// Read data from a device.
        const READ = 1 << 0;

        /// Write data to a device.
        const WRITE = 1 << 1;

        /// Flush cached writes to a device.
        const FLUSH = 1 << 2;
    }
}

/// Describes an error encountered while operating
/// on the device.
///
#[derive(Clone, Copy, Debug)]
pub enum Error {
    /// The buffer passed to the driver was not
    /// appropriate for the operation.
    InvalidBuffer,

    /// The device encountered an error while performing
    /// the requested operation.
    DeviceError,

    /// The requested operation is not supported.
    NotSupported,

    /// The device returned an invalid response.
    BadResponse,
}

/// Represents a block storage device.
///
pub trait Device {
    /// Returns the number of bytes in each segment.
    ///
    fn segment_size(&self) -> usize;

    /// Returns the device capacity as a number of
    /// segments.
    //
    fn num_segments(&self) -> usize;

    /// Returns the device capacity in bytes.
    ///
    fn capacity(&self) -> usize;

    /// Returns the set of operations supported by the
    /// device.
    ///
    /// If an unsupported operation is attempted, it
    /// will return [`Error::NotSupported`].
    ///
    fn operations(&self) -> Operations;

    /// Populates a byte slice with data from the device.
    ///
    /// `segment` indicates from which segment the data
    /// should be read. The data read will start at the
    /// offset `segment` * [`segment_size`](Self::segment_size).
    ///
    /// Note that `buf` must have a length that is an exact
    /// multiple the [`segment_size`](Self::segment_size).
    ///
    /// `read` returns the number of bytes read.
    ///
    fn read(&mut self, segment: usize, buf: &mut [u8]) -> Result<usize, Error>;

    /// Writes data from a byte slice to the device.
    ///
    /// `segment` indicates from which segment the data
    /// should be read. The data written will start at the
    /// offset `segment` * [`segment_size`](Self::segment_size).
    ///
    /// Note that `buf` must have a length that is an exact
    /// multiple the [`segment_size`](Self::segment_size).
    ///
    /// `write` returns the number of bytes written.
    ///
    /// If the device is read-only, calls to `write` will
    /// return [`Error::NotSupported`].
    ///
    fn write(&mut self, segment: usize, buf: &mut [u8]) -> Result<usize, Error>;

    /// Flush the buffered data at the given `segment`.
    ///
    /// `segment` indicates from which segment the data
    /// should be flushed. The data flushed will start at the
    /// offset `segment` * [`segment_size`](Self::segment_size).
    ///
    fn flush(&mut self, segment: usize) -> Result<(), Error>;
}
