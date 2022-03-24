// Copyright 2021 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// This is a small tool essentially copied from
// https://github.com/rust-osdev/bootimage/blob/dae6e75abfd60dfe66d46a1566840b5595b53b69/src/builder/disk_image.rs.
//
// It takes a bootable kernel in ELF format and
// uses llvm-objcopy to translate it to a raw
// binary and pad it so it aligns to 512-byte
// blocks.

use llvm_tools::LlvmTools;
use std::env;
use std::fs::{File, OpenOptions};
use std::io;
use std::path::Path;
use std::process::Command;

fn main() {
    // We expect our input and output args.
    let args: Vec<String> = env::args().collect();
    if args.len() != 3 {
        panic!("Usage:\n  {} INPUT OUTPUT", args[0]);
    }

    let input1 = Path::new(&args[1]);
    let input2 = Path::new("/tmp/input");
    let output = Path::new(&args[2]);

    // Check out input actually exists.
    assert!(input1.exists());

    // Copy the input in case it's not
    // writable, as llvm-objcopy mirrors
    // the permissions on the input to
    // the output.
    copy_file(input1, input2).unwrap();

    // Build the disk image.
    create_disk_image(input2, output).unwrap();
}

fn copy_file(src: &Path, dst: &Path) -> Result<(), io::Error> {
    let mut r = File::open(src)?;
    let mut w = File::create(dst)?;
    io::copy(&mut r, &mut w)?;

    Ok(())
}

fn create_disk_image(
    bootloader_elf_path: &Path,
    output_bin_path: &Path,
) -> Result<(), DiskImageError> {
    let llvm_tools = LlvmTools::new()?;
    let objcopy = llvm_tools
        .tool(&llvm_tools::exe("llvm-objcopy"))
        .ok_or(DiskImageError::LlvmObjcopyNotFound)?;

    // convert bootloader to binary
    let mut cmd = Command::new(objcopy);
    cmd.arg("-I").arg("elf64-x86-64");
    cmd.arg("-O").arg("binary");
    cmd.arg("--binary-architecture=i386:x86-64");
    cmd.arg(bootloader_elf_path);
    cmd.arg(output_bin_path);
    let output = cmd.output().map_err(|err| DiskImageError::Io {
        message: "failed to execute llvm-objcopy command",
        error: err,
    })?;
    if !output.status.success() {
        return Err(DiskImageError::ObjcopyFailed {
            stderr: output.stderr,
        });
    }

    pad_to_nearest_block_size(output_bin_path)?;

    Ok(())
}

fn pad_to_nearest_block_size(output_bin_path: &Path) -> Result<(), DiskImageError> {
    const BLOCK_SIZE: u64 = 512;
    let file = OpenOptions::new()
        .write(true)
        .open(&output_bin_path)
        .map_err(|err| DiskImageError::Io {
            message: "failed to open boot image",
            error: err,
        })?;

    let file_size = file
        .metadata()
        .map_err(|err| DiskImageError::Io {
            message: "failed to get size of boot image",
            error: err,
        })?
        .len();

    let remainder = file_size % BLOCK_SIZE;
    let padding = if remainder > 0 {
        BLOCK_SIZE - remainder
    } else {
        0
    };

    file.set_len(file_size + padding)
        .map_err(|err| DiskImageError::Io {
            message: "failed to pad boot image to a multiple of the block size",
            error: err,
        })
}

/// Creating the disk image failed.
#[derive(Debug)]
pub enum DiskImageError {
    /// The `llvm-tools-preview` rustup component was not found
    LlvmToolsNotFound,

    /// There was another problem locating the `llvm-tools-preview` rustup component
    LlvmTools(llvm_tools::Error),

    /// The llvm-tools component did not contain the required `llvm-objcopy` executable
    LlvmObjcopyNotFound,

    /// The `llvm-objcopy` command failed
    ObjcopyFailed {
        /// The output of `llvm-objcopy` to standard error
        stderr: Vec<u8>,
    },

    /// An unexpected I/O error occurred
    Io {
        /// Desciption of the failed I/O operation
        message: &'static str,
        /// The I/O error that occured
        error: io::Error,
    },
}

impl From<llvm_tools::Error> for DiskImageError {
    fn from(err: llvm_tools::Error) -> Self {
        match err {
            llvm_tools::Error::NotFound => DiskImageError::LlvmToolsNotFound,
            other => DiskImageError::LlvmTools(other),
        }
    }
}
