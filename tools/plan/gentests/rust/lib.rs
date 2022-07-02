// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

#![no_std]
#![deny(clippy::float_arithmetic)]
#![deny(clippy::inline_asm_x86_att_syntax)]
#![deny(clippy::missing_panics_doc)]
#![deny(clippy::panic)]
#![deny(clippy::return_self_not_must_use)]
#![deny(clippy::single_char_lifetime_names)]
#![allow(clippy::wildcard_imports)] // To import the generated structures.
#![deny(unused_crate_dependencies)]
#![allow(unsafe_code)] // Testss use `transmute`.

#[cfg(not(test))]
use gentests as _; // Ignore the unused crate dependency outside test mode.

#[cfg(test)]
#[allow(unsafe_code)]
mod manual_tests {
    use core::mem::{size_of, transmute};
    use gentests::*;

    // We test various properties here:
    //
    // 1. Check that the helper methods that produce integers produce the right values (for when passed in parameters).
    // 2. Check that the types have the right representation in memory (for when stored in structures/arrays).
    // 3. Check that parsing numerical values produce the right Rust represntation.

    #[test]
    fn test_array_layout() {
        // Nothing to do here. Rust's and our definition of array layouts
        // is identical, so there's no way to test the layour of Rust's
        // arrays independently.
    }

    #[test]
    fn test_bitfield_layout() {
        // 1: Helpers.

        assert_eq!(FilePermissions::empty().bits(), 0u8);
        assert_eq!(FilePermissions::EXECUTE.bits(), 1u8);
        assert_eq!(FilePermissions::WRITE.bits(), 2u8);
        assert_eq!(FilePermissions::READ.bits(), 4u8);

        // 3: Parsing.

        assert_eq!(
            unsafe { transmute::<FilePermissions, u8>(FilePermissions::empty()) },
            0u8
        );
        assert_eq!(
            unsafe { transmute::<FilePermissions, u8>(FilePermissions::EXECUTE) },
            1u8
        );
        assert_eq!(
            unsafe { transmute::<FilePermissions, u8>(FilePermissions::WRITE) },
            2u8
        );
        assert_eq!(
            unsafe { transmute::<FilePermissions, u8>(FilePermissions::READ) },
            4u8
        );
    }

    #[test]
    fn test_enumeration_layout() {
        // 1: Helpers.

        assert_eq!(TransportLayerProtocol::Tcp.as_u8(), 0u8);
        assert_eq!(TransportLayerProtocol::Udp.as_u8(), 1u8);

        assert_eq!(Error::NoError.as_u64(), 0u64);
        assert_eq!(Error::BadSyscall.as_u64(), 1u64);
        assert_eq!(Error::IllegalArg1.as_u64(), 2u64);
        assert_eq!(Error::IllegalArg2.as_u64(), 3u64);
        assert_eq!(Error::IllegalArg3.as_u64(), 4u64);
        assert_eq!(Error::IllegalArg4.as_u64(), 5u64);
        assert_eq!(Error::IllegalArg5.as_u64(), 6u64);
        assert_eq!(Error::IllegalArg6.as_u64(), 7u64);

        // 2: In memory.

        assert_eq!(
            unsafe { transmute::<TransportLayerProtocol, u8>(TransportLayerProtocol::Tcp) },
            0u8
        );
        assert_eq!(
            unsafe { transmute::<TransportLayerProtocol, u8>(TransportLayerProtocol::Udp) },
            1u8
        );

        assert_eq!(unsafe { transmute::<Error, u64>(Error::NoError) }, 0u64);
        assert_eq!(unsafe { transmute::<Error, u64>(Error::BadSyscall) }, 1u64);
        assert_eq!(unsafe { transmute::<Error, u64>(Error::IllegalArg1) }, 2u64);
        assert_eq!(unsafe { transmute::<Error, u64>(Error::IllegalArg2) }, 3u64);
        assert_eq!(unsafe { transmute::<Error, u64>(Error::IllegalArg3) }, 4u64);
        assert_eq!(unsafe { transmute::<Error, u64>(Error::IllegalArg4) }, 5u64);
        assert_eq!(unsafe { transmute::<Error, u64>(Error::IllegalArg5) }, 6u64);
        assert_eq!(unsafe { transmute::<Error, u64>(Error::IllegalArg6) }, 7u64);

        // 3: Parsing.

        assert_eq!(
            TransportLayerProtocol::from_u8(0u8).unwrap(),
            TransportLayerProtocol::Tcp
        );
        assert_eq!(
            TransportLayerProtocol::from_u8(1u8).unwrap(),
            TransportLayerProtocol::Udp
        );

        assert_eq!(Error::from_u64(0u64).unwrap(), Error::NoError);
        assert_eq!(Error::from_u64(1u64).unwrap(), Error::BadSyscall);
        assert_eq!(Error::from_u64(2u64).unwrap(), Error::IllegalArg1);
        assert_eq!(Error::from_u64(3u64).unwrap(), Error::IllegalArg2);
        assert_eq!(Error::from_u64(4u64).unwrap(), Error::IllegalArg3);
        assert_eq!(Error::from_u64(5u64).unwrap(), Error::IllegalArg4);
        assert_eq!(Error::from_u64(6u64).unwrap(), Error::IllegalArg5);
        assert_eq!(Error::from_u64(7u64).unwrap(), Error::IllegalArg6);
    }

    #[test]
    fn test_integer_layout() {
        // 2: In memory.

        assert_eq!(unsafe { transmute::<PortNumber, u16>(PortNumber(1)) }, 1u16);
    }

    #[test]
    fn test_structure_layout() {
        // 2: In memory.

        assert_eq!(
            unsafe {
                transmute::<FileInfo, [u8; size_of::<FileInfo>()]>(FileInfo {
                    name_pointer: 0x1122_3344_5566_7788_usize as *const u8,
                    name_size: 0x99aa_bbcc_ddee_ff12_u64,
                    permissions: FilePermissions::READ,
                    _trailing_padding: [0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf1],
                })
            },
            [
                // name pointer (little-endian).
                0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11,
                // name size (little-endian).
                0x12, 0xff, 0xee, 0xdd, 0xcc, 0xbb, 0xaa, 0x99, // permissions.
                0x04, // trailing padding.
                0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf1,
            ],
        );
    }
}
