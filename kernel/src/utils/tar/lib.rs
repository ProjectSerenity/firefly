// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! Implements functionality to read [TAR](https://en.wikipedia.org/wiki/Tar_(computing))
//! archives from a block device.
//!
//! This module allows the caller to parse the contents of a TAR
//! archive with minimal memory overhead. In particular, the reader
//! uses a single 512-byte buffer to iterate through the contents
//! of the archive. The caller provides memory buffers to store file
//! contents.
//!
//! Note that this reader only extracts the metadata relevant to
//! Firefly. In particular, file ownership details and symbolic
//! links are ignored (or return an error, as appropriate).

#![no_std]

extern crate alloc;

use align::align_up_usize;
use alloc::boxed::Box;
use alloc::string::String;
use core::cmp::min;
use core::str;
use filesystem::{FileInfo, FileType, Permissions, SEPARATOR};
use serial::println;
use storage::block;

mod parse_utils;

/// Helper used for checking whether a filename starts or ends
/// with a separator.
///
const SEPARATOR_STR: &[u8] = &[SEPARATOR as u8];

/// The block size of a TAR archive. Each header is contained
/// in one block, and each file is contained within an integral
/// number of blocks.
///
const BLOCK_SIZE: usize = 512;

/// Represents a file, which can be read from a TAR archive.
///
pub struct File {
    info: FileInfo,
    offset: usize,
    size: usize,
    readable: bool,
}

impl File {
    /// Returns the FileInfo describing this file.
    ///
    pub fn info(&self) -> &FileInfo {
        &self.info
    }
}

/// Indicates an error encountered while parsing a TAR archive.
///
pub enum Error {
    /// Indicates that the archive contained a file whose
    /// header gave an invalid file name.
    InvalidFileName,

    /// Indicates that the archive contained a file whose
    /// header gave an invalid file permissions.
    InvalidFilePermissions,

    /// Indicates that the archive contained a file whose
    /// header gave an invalid file size.
    InvalidFileSize,
}

/// An iterator returning the file information about
/// each entry in a TAR archive.
///
pub struct Reader<'a> {
    // The buffer we use to store headers.
    header: [u8; BLOCK_SIZE],

    // The segment index of the next header
    // we will read.
    next_segment: usize,

    // Caches the number of segments in the
    // block device.
    num_segments: usize,

    // The block device we're reading from.
    device: &'a mut Box<dyn block::Device + Send>,
}

impl<'a> Reader<'a> {
    /// Read the TAR archive data from the given block
    /// device.
    ///
    /// The block device must have a segment size that
    /// is an exact multiple of 512 bytes.
    ///
    pub fn new(device: &'a mut Box<dyn block::Device + Send>) -> Self {
        Reader {
            header: [0u8; BLOCK_SIZE],
            next_segment: 0,
            num_segments: device.num_segments(),
            device,
        }
    }

    /// Read the contents of the specified file into
    /// the given buffer. The number of bytes read is
    /// returned.
    ///
    pub fn read(&mut self, file: &File, buf: &mut [u8]) -> Result<usize, block::Error> {
        if !file.readable {
            return Err(block::Error::NotSupported);
        }

        let mut written = 0;

        // Work out how many complete segments we
        // can read first.
        let total = min(file.size, buf.len());
        let complete = total & !BLOCK_SIZE; // Align down to block size.
        let complete_segments = complete / BLOCK_SIZE;
        if complete_segments > 0 {
            if self.device.read(file.offset, &mut buf[..complete])? != complete {
                // Invalid block size.
                // Shouldn't happen in practice.
                return Err(block::Error::DeviceError);
            }

            written += complete;
        }

        if written == total {
            return Ok(written);
        }

        // Read the last block into a cache buffer.
        let mut cache = [0u8; BLOCK_SIZE];
        if self
            .device
            .read(file.offset + complete_segments, &mut cache)?
            != BLOCK_SIZE
        {
            // Invalid block size.
            // Shouldn't happen in practice.
            return Err(block::Error::DeviceError);
        }

        let remaining = total - written;
        buf[written..total].copy_from_slice(&cache[..remaining]);

        Ok(total)
    }

    /// Returns the given number of bytes at the given
    /// offset.
    ///
    #[inline(always)]
    fn header(&self, offset: usize, length: usize) -> &[u8] {
        &self.header[offset..(offset + length)]
    }

    /// Returns the number at the given offset and length.
    ///
    #[inline(always)]
    fn read_number(&self, offset: usize, length: usize) -> Option<isize> {
        parse_utils::parse_number(self.header(offset, length))
    }

    /// Returns the ASCII string at the given offset
    /// and length.
    ///
    #[inline(always)]
    fn read_string(&self, offset: usize, length: usize) -> &[u8] {
        parse_utils::parse_string(self.header(offset, length))
    }

    /// Returns an error if the checksum header does not
    /// match the computed checksum.
    ///
    fn check_checksum(&self) -> Result<(), &'static str> {
        let got = self.read_number(148, 8).ok_or("invalid checksum")?;
        let (signed, unsigned) = parse_utils::parse_checksum(&self.header);
        if signed == got || unsigned == got {
            Ok(())
        } else {
            println!(
                "checksum: got {} ({:02x?}), want {} or {}",
                got,
                self.header(148, 8),
                signed,
                unsigned
            );
            Err("checksum does not match")
        }
    }

    /// Parse the next header, returning its information,
    /// or an error.
    ///
    /// You should probably use the Iterator trait and the
    /// [`next`](Reader::next) method instead. This just
    /// makes error handling more concise.
    ///
    fn next_header(&mut self) -> Result<Option<File>, &'static str> {
        if self.next_segment >= self.num_segments {
            // We've run out of archive.
            return Ok(None);
        }

        // Read the next header into our
        // header buffer.
        if self
            .device
            .read(self.next_segment, &mut self.header)
            .or(Err("block read error"))?
            != BLOCK_SIZE
        {
            // Invalid block size.
            // Shouldn't happen in practice.
            return Err("invalid block size");
        }

        // Check whether we've reached the end of
        // the archive, which should consist of
        // two zero blocks.
        if self.header.iter().all(|&x| x == 0) {
            // There should be two consecutive zero
            // headers, but we accept 1.
            self.next_segment += 1;
            if self.next_segment >= self.num_segments {
                return Ok(None);
            }

            if self
                .device
                .read(self.next_segment, &mut self.header)
                .or(Err("block read error"))?
                != BLOCK_SIZE
            {
                // Invalid block size.
                // Shouldn't happen in practice.
                return Err("invalid block size");
            }

            if self.header.iter().all(|&x| x == 0) {
                // This is what we'd expect.
                return Ok(None);
            }

            return Err("found non-zero header after zero header");
        }

        // Start by reading the file name.
        let mut name = String::new();

        // If this is a UStar archive, then we
        // copy in any name prefix.
        let is_ustar = self.header(257, 6) == b"ustar\x00" && self.header(263, 2) == b"00";
        if is_ustar {
            let prefix = self.read_string(345, 155);
            if !prefix.is_empty() {
                // Parse the prefix as UTF-8.
                name.push_str(str::from_utf8(prefix).or(Err("invalid name prefix"))?);
            }
        }

        // Retrieve the file name (suffix).
        let suffix = self.read_string(0, 100);

        // Copy the file name (suffix), parsing as
        // UTF-8.
        name.push_str(str::from_utf8(suffix).or(Err("invalid name"))?);

        // Check whether the filename ends
        // in a separator. If so, it's an
        // old archive referencing a directory.
        let mut file_type = FileType::Unknown;
        if name.as_bytes().ends_with(SEPARATOR_STR) {
            file_type = FileType::Directory;
            name.pop(); // Drop the trailing separator.
            if name.as_bytes().ends_with(SEPARATOR_STR) {
                // File name is invalid.
                return Err("name ended in two separators");
            }
        }

        // Get the file type.
        let ftype = self.header(156, 1)[0] as char;
        match ftype {
            '0' | '\x00' => {
                // Normal file, unless already known to be
                // a directory.
                if file_type == FileType::Unknown {
                    file_type = FileType::RegularFile;
                }
            }
            '5' => {
                file_type = FileType::Directory;
            }
            _ => {
                // Skip to the next entry.
                self.next_segment += 1;
                return self.next_header();
            }
        }

        // Make sure the checksum matches, once
        // we know we're processing a standard
        // header.
        //self.check_checksum()?;
        match self.check_checksum() {
            Ok(_) => {}
            Err(err) => {
                println!("bad checksum for {:?}", name);
                return Err(err);
            }
        }

        // Get the file size so we can update the
        // next segment in case the permissions
        // cut us short.
        let mut size = {
            let size = self.read_number(124, 12).ok_or("invalid size")?;
            if size < 0 {
                // This is clearly invalid.
                return Err("size is negative");
            }

            size as usize
        };

        let offset = self.next_segment + 1;
        self.next_segment += 1 + align_up_usize(size, BLOCK_SIZE) / BLOCK_SIZE;

        // Get the file mode, so we can derive
        // the permissions by taking the owner's
        // permissions.
        let mode = self.read_number(100, 8).ok_or("invalid file mode")?;

        let mut permissions =
            Permissions::from_bits_truncate((((mode as usize) & 0o700) >> 6) as u8);
        if file_type == FileType::Directory {
            size = 0;
            permissions &= !Permissions::EXECUTE; // Invalid for directories.
        }

        // We now have the information we need to return
        // the gathered file info.

        let file = if permissions.contains(Permissions::READ) {
            File {
                info: FileInfo {
                    name,
                    file_type,
                    permissions,
                    size,
                },
                offset,
                size,
                readable: file_type == FileType::RegularFile,
            }
        } else {
            // Redact everything except the name.
            File {
                info: FileInfo {
                    name,
                    file_type: FileType::Unknown,
                    permissions: Permissions::NONE,
                    size: 0,
                },
                offset: 0,
                size: 0,
                readable: false,
            }
        };

        Ok(Some(file))
    }
}

impl<'a> Iterator for Reader<'a> {
    type Item = File;

    fn next(&mut self) -> Option<Self::Item> {
        match self.next_header() {
            Ok(file) => file,
            Err(err) => {
                println!("tar: bad archive: {}", err);
                None
            }
        }
    }
}

#[cfg(test)]
mod test {
    use super::*;
    use align::align_up_usize;
    use alloc::boxed::Box;
    use alloc::vec;
    use alloc::vec::Vec;
    use hex_literal::hex;
    use sha2::digest::generic_array::GenericArray;
    use sha2::{Digest, Sha256};
    use storage::block::{Device, Error, Operations};

    const SEGMENT_SIZE: usize = 512;
    const CHECKSUM_SIZE: usize = 256 / 8;

    struct TestFile {
        info: FileInfo,
        sha256: Option<[u8; CHECKSUM_SIZE]>,
    }

    struct TestCase {
        name: &'static str,
        data: &'static [u8],
        files: Vec<TestFile>,
    }

    struct BytesBlockDevice {
        data: &'static [u8],
    }

    impl BytesBlockDevice {
        fn new(data: &'static [u8]) -> Self {
            BytesBlockDevice { data }
        }
    }

    impl Device for BytesBlockDevice {
        /// Returns the number of bytes in each segment.
        ///
        fn segment_size(&self) -> usize {
            SEGMENT_SIZE
        }

        /// Returns the device capacity as a number of
        /// segments.
        //
        fn num_segments(&self) -> usize {
            align_up_usize(self.data.len(), SEGMENT_SIZE) / SEGMENT_SIZE
        }

        /// Returns the device capacity in bytes.
        ///
        fn capacity(&self) -> usize {
            align_up_usize(self.data.len(), SEGMENT_SIZE)
        }

        /// Returns the set of operations supported by the
        /// device.
        ///
        /// If an unsupported operation is attempted, it
        /// will return [`Error::NotSupported`].
        ///
        fn operations(&self) -> Operations {
            Operations::READ
        }

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
        fn read(&mut self, segment: usize, buf: &mut [u8]) -> Result<usize, Error> {
            if segment >= self.num_segments() {
                return Err(Error::NotSupported);
            }

            let offset = segment * SEGMENT_SIZE;
            let len = buf.len();
            let remaining = self.data.len() - offset;
            if remaining >= len {
                buf.copy_from_slice(&self.data[offset..(offset + len)]);
            } else {
                buf[..remaining].copy_from_slice(&self.data[offset..]);
                buf[remaining..].fill(0);
            }

            Ok(len)
        }

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
        fn write(&mut self, _segment: usize, _buf: &mut [u8]) -> Result<usize, Error> {
            Err(Error::NotSupported)
        }

        /// Flush the buffered data at the given `segment`.
        ///
        /// `segment` indicates from which segment the data
        /// should be flushed. The data flushed will start at the
        /// offset `segment` * [`segment_size`](Self::segment_size).
        ///
        fn flush(&mut self, _segment: usize) -> Result<(), Error> {
            Err(Error::NotSupported)
        }
    }

    #[test]
    fn test_reader() {
        let cases = [
            TestCase {
                name: "gnu.tar",
                data: include_bytes!("testdata/gnu.tar"),
                files: vec![
                    TestFile {
                        info: FileInfo {
                            name: String::from("small.txt"),
                            permissions: Permissions::READ | Permissions::WRITE,
                            size: 5,
                            file_type: FileType::RegularFile, // type: '0'
                                                              // format: GNU,
                        },
                        sha256: Some(hex!(
                            "cf19779e5e822d613a32de6a69e2291d5769e77556fe95b30ba5b238d8de85cf"
                        )),
                    },
                    TestFile {
                        info: FileInfo {
                            name: String::from("small2.txt"),
                            permissions: Permissions::READ | Permissions::WRITE,
                            size: 11,
                            file_type: FileType::RegularFile, // type: '0'
                                                              // format: GNU,
                        },
                        sha256: Some(hex!(
                            "3958356ff8cf977451b12d23db29c0e822682c1ddc7e7a8dfd1b581cf5c6c5bf"
                        )),
                    },
                ],
            },
            TestCase {
                name: "star.tar",
                data: include_bytes!("testdata/star.tar"),
                files: vec![
                    TestFile {
                        info: FileInfo {
                            name: String::from("small.txt"),
                            permissions: Permissions::READ | Permissions::WRITE,
                            size: 5,
                            file_type: FileType::RegularFile, // type: '0'
                        },
                        sha256: Some(hex!(
                            "cf19779e5e822d613a32de6a69e2291d5769e77556fe95b30ba5b238d8de85cf"
                        )),
                    },
                    TestFile {
                        info: FileInfo {
                            name: String::from("small2.txt"),
                            permissions: Permissions::READ | Permissions::WRITE,
                            size: 11,
                            file_type: FileType::RegularFile, // type: '0'
                        },
                        sha256: Some(hex!(
                            "3958356ff8cf977451b12d23db29c0e822682c1ddc7e7a8dfd1b581cf5c6c5bf"
                        )),
                    },
                ],
            },
            TestCase {
                name: "v7.tar",
                data: include_bytes!("testdata/v7.tar"),
                files: vec![
                    TestFile {
                        info: FileInfo {
                            name: String::from("small.txt"),
                            permissions: Permissions::READ,
                            size: 5,
                            file_type: FileType::RegularFile, // type: '0'
                        },
                        sha256: Some(hex!(
                            "cf19779e5e822d613a32de6a69e2291d5769e77556fe95b30ba5b238d8de85cf"
                        )),
                    },
                    TestFile {
                        info: FileInfo {
                            name: String::from("small2.txt"),
                            permissions: Permissions::READ,
                            size: 11,
                            file_type: FileType::RegularFile, // type: '0'
                        },
                        sha256: Some(hex!(
                            "3958356ff8cf977451b12d23db29c0e822682c1ddc7e7a8dfd1b581cf5c6c5bf"
                        )),
                    },
                ],
            },
            TestCase {
                name: "nil-uid.tar",
                data: include_bytes!("testdata/nil-uid.tar"), // golang.org/issue/5290
                files: vec![TestFile {
                    info: FileInfo {
                        name: String::from("P1050238.JPG.log"),
                        permissions: Permissions::READ | Permissions::WRITE,
                        size: 14,
                        file_type: FileType::RegularFile, // type: '0'
                                                          // format: GNU,
                    },
                    sha256: None,
                }],
            },
            TestCase {
                // USTAR archive with a regular entry with non-zero device numbers.
                name: "ustar-file-devs.tar",
                data: include_bytes!("testdata/ustar-file-devs.tar"),
                files: vec![TestFile {
                    info: FileInfo {
                        name: String::from("file"),
                        permissions: Permissions::READ | Permissions::WRITE,
                        file_type: FileType::RegularFile, // type: '0'
                        // format: USTAR,
                        size: 0,
                    },
                    sha256: None,
                }],
            },
        ];

        let mut hasher = Sha256::new();
        let mut sha256 = [0u8; CHECKSUM_SIZE];
        for test_case in cases.iter() {
            let device = BytesBlockDevice::new(test_case.data);
            let mut boxed = Box::new(device) as Box<dyn Device + Send>;
            let mut reader = Reader::new(&mut boxed);
            for (i, want) in test_case.files.iter().enumerate() {
                match reader.next_header() {
                    Err(err) => {
                        panic!(
                            "error reading header {} for {}: {:?}",
                            i, test_case.name, err
                        );
                    }
                    Ok(None) => {
                        panic!("got no header {} for {}", i, test_case.name);
                    }
                    Ok(Some(got)) => {
                        assert_eq!(
                            got.info.name, want.info.name,
                            "header {} for {}",
                            i, test_case.name
                        );
                        assert_eq!(
                            got.info.permissions, want.info.permissions,
                            "header {} for {}",
                            i, test_case.name
                        );
                        assert_eq!(
                            got.info.file_type, want.info.file_type,
                            "header {} for {}",
                            i, test_case.name
                        );
                        assert_eq!(
                            got.info.size, want.info.size,
                            "header {} for {}",
                            i, test_case.name
                        );

                        if let Some(want_sha256) = want.sha256 {
                            // Read the file and check its SHA-256
                            // checksum.
                            let mut contents = vec![0u8; got.info.size];
                            match reader.read(&got, contents.as_mut_slice()) {
                                Ok(n) => {
                                    assert_eq!(
                                        n, got.info.size,
                                        "file {} for {}",
                                        i, test_case.name
                                    );
                                }
                                Err(err) => {
                                    panic!(
                                        "failed to read {} for {}: {:?}",
                                        i, test_case.name, err
                                    );
                                }
                            }

                            // Calculate the checksum.
                            hasher.update(contents);
                            hasher.finalize_into_reset(&mut GenericArray::from_mut_slice(
                                &mut sha256[..],
                            ));
                            assert_eq!(sha256, want_sha256, "file {} for {}", i, test_case.name);
                        }
                    }
                }
            }

            // Check the iterator has finished.
            match reader.next_header() {
                Err(err) => {
                    panic!(
                        "error reading unexpected header for {}: {:?}",
                        test_case.name, err
                    );
                }
                Ok(None) => {}
                Ok(Some(got)) => {
                    panic!(
                        "unexpected header for {} (name: {:?})",
                        test_case.name, got.info.name
                    );
                }
            }
        }
    }
}
