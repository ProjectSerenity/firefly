// Forked from bootloader 0.9.22, copyright 2018 Philipp Oppermann.
//
// Use of the original source code is governed by the MIT
// license that can be found in the LICENSE.orig file.
//
// Subsequent work copyright 2022 The Firefly Authors.
//
// Use of new and modified source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

#[cfg(not(feature = "vga_320x200"))]
pub use self::vga_text_80x25::*;

#[cfg(feature = "vga_320x200")]
pub use self::vga_320x200::*;

#[cfg(feature = "vga_320x200")]
mod vga_320x200;

#[cfg(not(feature = "vga_320x200"))]
mod vga_text_80x25;
