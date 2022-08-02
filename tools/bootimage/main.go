// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command bootimage turns the bootloader and kernel into a bootable disk image.
//
// This has four responsibilities. Firstly, we parse the bootloader binary and
// check that stages 2 onward do not exceed 127 disk sectors, so that stage 1
// can load them successfully. Secondly, we identify the addresses where the
// kernel size should be stored and the kernel should begin. Thirdly, we write
// the size of the kernel in bytes into the relevant part of the bootloader.
// Finally, we write the modified bootloader's segments and the entire kernel
// binary to the output file. Note that we strip the ELF headers from the
// bootloader, just writing the segments that are loaded into memory.
package main

import (
	"bytes"
	"debug/elf"
	"encoding/binary"
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
)

const blockSize = 512

var zeros [blockSize]uint8

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix("")
}

func main() {
	var help bool
	var bootloaderName, kernelName, userName, outName string
	flag.BoolVar(&help, "h", false, "Print this help message and exit.")
	flag.StringVar(&bootloaderName, "bootloader", "", "Path to the bootloader binary.")
	flag.StringVar(&kernelName, "kernel", "", "Path to the kernel binary.")
	flag.StringVar(&userName, "user", "", "Path to the user data.")
	flag.StringVar(&outName, "out", "", "Path to where the bootable image should be written.")
	flag.Usage = func() {
		log.Printf("Usage:\n  %s [OPTIONS]\n\nOptions:", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}

	flag.Parse()

	if help {
		flag.Usage()
		os.Exit(2)
	}

	if bootloaderName == "" {
		log.Println("Missing -bootloader argument.")
		flag.Usage()
		os.Exit(1)
	}

	if kernelName == "" {
		log.Println("Missing -kernel argument.")
		flag.Usage()
		os.Exit(1)
	}

	if userName == "" {
		log.Println("Missing -user argument.")
		flag.Usage()
		os.Exit(1)
	}

	if outName == "" {
		log.Println("Missing -out argument.")
		flag.Usage()
		os.Exit(1)
	}

	bootloader, err := elf.Open(bootloaderName)
	if err != nil {
		log.Fatalf("Failed to parse bootloader: %v", err)
	}

	defer bootloader.Close()

	// Check that the stage two bootloader
	// is small enough to be loaded into
	// memory. This means checking that it
	// fits in fewer than 128 512-byte disk
	// sectors.
	const (
		stageTwoStart      = "_rest_of_bootloader_start_addr"
		stageTwoEnd        = "_rest_of_bootloader_end_addr"
		kernelStartAddr    = "_kernel_start_addr"
		kernelSizeAddr     = "_kernel_size_addr"
		sectorSize         = 512
		maxStageTwoSectors = 127
	)

	symbols, err := bootloader.Symbols()
	if err != nil {
		log.Fatalf("Failed to parse symbol table: %v", err)
	}

	var startSymbol, endSymbol, kernelStartSymbol, kernelSizeSymbol elf.Symbol
	for _, sym := range symbols {
		switch sym.Name {
		case stageTwoStart:
			startSymbol = sym
		case stageTwoEnd:
			endSymbol = sym
		case kernelStartAddr:
			kernelStartSymbol = sym
		case kernelSizeAddr:
			kernelSizeSymbol = sym
		}
	}

	if startSymbol.Value == 0 || endSymbol.Value == 0 {
		log.Fatalf("Failed to find stage two bootloader")
	}
	if startSymbol.Value > endSymbol.Value {
		log.Fatalf("Invalid stage two bootloader: region %#x-%#x", startSymbol.Value, endSymbol.Value)
	}
	if kernelStartSymbol.Value == 0 || kernelSizeSymbol.Value == 0 {
		log.Fatalf("Failed to find kernel size symbol in the bootloader")
	}

	stageTwoSize := endSymbol.Value - startSymbol.Value
	stageTwoSectors := (stageTwoSize + sectorSize - 1) / sectorSize
	if stageTwoSectors > maxStageTwoSectors {
		log.Fatalf("Stage two bootloader is too large: %d bytes (%d sectors)", stageTwoSize, stageTwoSectors)
	}

	seenHeaders := false
	segments := make([]*elf.Prog, 0, len(bootloader.Progs))
	for _, prog := range bootloader.Progs {
		// Only consider parts of the binary that
		// end up in memory.
		if prog.Type != elf.PT_LOAD {
			continue
		}

		// Ignore the first load segment, as it just has the ELF headers.
		if !seenHeaders {
			seenHeaders = true
			continue
		}

		segments = append(segments, prog)
	}

	if len(segments) == 0 {
		log.Fatalf("No valid program segments found.")
	}

	sort.Slice(segments, func(i, j int) bool { return segments[i].Vaddr < segments[j].Vaddr })

	var buf bytes.Buffer
	for _, segment := range segments {
		_, err := io.Copy(&buf, segment.Open())
		if err != nil {
			log.Fatalf("Failed to copy segment: %v", err)
		}
	}

	kernel, err := os.Open(kernelName)
	if err != nil {
		log.Fatalf("Failed to open kernel at %s: %v", kernelName, err)
	}

	defer kernel.Close()

	kernelInfo, err := kernel.Stat()
	if err != nil {
		log.Fatalf("Failed to stat kernel: %v", err)
	}

	kernelSize := kernelInfo.Size()

	// Round the image size up to the next multiple
	// of 512.
	written := int64(buf.Len()) + kernelSize
	kernelPadding := blockSize - (written % blockSize)
	if kernelPadding == blockSize {
		kernelPadding = 0
	}

	written += kernelPadding

	user, err := os.Open(userName)
	if err != nil {
		log.Fatalf("Failed to open user at %s: %v", userName, err)
	}

	defer user.Close()

	userInfo, err := user.Stat()
	if err != nil {
		log.Fatalf("Failed to stat user: %v", err)
	}

	userSize := userInfo.Size()

	// Round the image size up to the next multiple
	// of 512.
	written += userSize
	userPadding := blockSize - (written % blockSize)
	if userPadding == blockSize {
		userPadding = 0
	}

	written += userPadding

	// Write the kernel's size to the symbol address.
	// First we work out the virtual address at the
	// start of the first segment, as we need to
	// subtract that offset from the address of the
	// kernel size so we overwrite the right bit of
	// memory.
	offset := segments[0].Vaddr
	binary.LittleEndian.PutUint32(buf.Bytes()[kernelSizeSymbol.Value-offset:], uint32(kernelSize))

	// We also write out the offset into the image
	// where user beings, offset from the end of
	// the first segment (512 bytes) by 6 bytes.
	// This makes the 32-bit value the last contents
	// of the segment, except the 16-bit MBR magic.
	binary.LittleEndian.PutUint32(buf.Bytes()[512-6:], uint32(written-(userSize+userPadding)))

	// Check that we're putting the kernel where the
	// bootloader expects it.
	if uint64(buf.Len()) != kernelStartSymbol.Value-offset {
		log.Fatalf("appending kernel at %#x but bootloader expects it at %#x", uint64(buf.Len()), kernelStartSymbol.Value-offset)
	}

	// Write out the modified bootloader.
	out, err := os.Create(outName)
	if err != nil {
		log.Fatalf("Failed to create out at %s: %v", outName, err)
	}

	_, err = out.Write(buf.Bytes())
	if err != nil {
		log.Fatalf("Failed to write bootloader to %s: %v", outName, err)
	}

	// Then the kernel.
	_, err = io.Copy(out, kernel)
	if err != nil {
		log.Fatalf("Failed to write kernel to %s: %v", outName, err)
	}

	if kernelPadding != 0 {
		_, err = out.Write(zeros[:kernelPadding])
		if err != nil {
			log.Fatalf("Failed to write padding to %s: %v", outName, err)
		}
	}

	// Then the user.
	_, err = io.Copy(out, user)
	if err != nil {
		log.Fatalf("Failed to write user to %s: %v", outName, err)
	}

	if userPadding != 0 {
		_, err = out.Write(zeros[:userPadding])
		if err != nil {
			log.Fatalf("Failed to write padding to %s: %v", outName, err)
		}
	}

	err = out.Close()
	if err != nil {
		log.Fatalf("Failed to close %s: %v", outName, err)
	}
}
