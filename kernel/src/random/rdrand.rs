// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

//! rdrand provides an entropy source using the RDRAND instruction, if available.

use crate::random::{register_entropy_source, EntropySource};
use alloc::boxed::Box;
use x86_64::instructions::random::RdRand;

/// RdRandEntropySource is an entropy source using the RDRAND instruction.
///
pub struct RdRandEntropySource {
    rdrand: RdRand,
}

/// fill_slot uses the 64-bit version of RDRAND
/// to fill a 64-bit slot in an array. fill_slot
/// may call RDRAND multiple times.
///
macro_rules! fill_slot {
    ($rdrand:expr, $buf:expr, $offset:expr) => {
        loop {
            let val = match $rdrand.get_u64() {
                None => continue,
                Some(val) => val,
            };

            $buf[$offset + 0] = val as u8;
            $buf[$offset + 1] = (val >> 8) as u8;
            $buf[$offset + 2] = (val >> 16) as u8;
            $buf[$offset + 3] = (val >> 24) as u8;
            $buf[$offset + 4] = (val >> 32) as u8;
            $buf[$offset + 5] = (val >> 40) as u8;
            $buf[$offset + 6] = (val >> 48) as u8;
            $buf[$offset + 7] = (val >> 56) as u8;

            break;
        }
    };
}

impl EntropySource for RdRandEntropySource {
    fn get_entropy(&mut self, buf: &mut [u8; 32]) {
        // We can use the 64-bit version 4 times,
        // rather than needing to call the 8-bit
        // version 8 32 times.
        fill_slot!(self.rdrand, buf, 0);
        fill_slot!(self.rdrand, buf, 8);
        fill_slot!(self.rdrand, buf, 16);
        fill_slot!(self.rdrand, buf, 24);
    }
}

/// init determines whether the RDRAND instruction is
/// available, and if so registers an entropy source
/// using it.
///
pub fn init() {
    match RdRand::new() {
        None => return,
        Some(rdrand) => {
            let source = RdRandEntropySource { rdrand };
            register_entropy_source(Box::new(source));
        }
    }
}
