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
	"math"
	"os"
	"path/filepath"
	"slices"
)

const blockSize = 512

var zeros [blockSize]uint8

const bootStage1 = "boot-stage-1"

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix("")
}

func main() {
	var help bool
	var bootloaderName, kernelName, outName string
	flag.BoolVar(&help, "h", false, "Print this help message and exit.")
	flag.StringVar(&bootloaderName, "bootloader", "", "Path to the bootloader binary.")
	flag.StringVar(&kernelName, "kernel", "", "Path to the kernel binary.")
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
		sectorSize         = 512
		maxStageTwoSectors = 127
	)

	sections := make([]*elf.Section, 0, len(bootloader.Sections))
	for _, sect := range bootloader.Sections {
		// Only consider parts of the binary that
		// end up in memory.
		if sect.Type != elf.SHT_PROGBITS {
			continue
		}

		sections = append(sections, sect)
	}

	if len(sections) == 0 {
		log.Fatalf("No valid program sections found.")
	}

	slices.SortFunc(sections, func(a, b *elf.Section) int {
		// The stage 1 boot section always goes first,
		// as it must be in the first disk sector.

		if a.Name == bootStage1 {
			return -1
		}

		if b.Name == bootStage1 {
			return +1
		}

		// Otherwise, order by address.

		if a.Addr < b.Addr {
			return -1
		}

		if a.Addr < b.Addr {
			return +1
		}

		return 0
	})

	if sections[0].Name != bootStage1 {
		log.Fatalf("Failed to find boot section %q.", bootStage1)
	}

	if sections[0].Addr > math.MaxUint16 {
		log.Fatalf("Invalid bootloader has start address %#x, which is outside the 16-bit address space.", sections[0].Addr)
	}

	// The boot section must fit in the first
	// sector, with enough space for the trailing
	// MBR marker 0xaa55.
	if sections[0].Size != sectorSize {
		log.Fatalf("Bootloader stage 1 does not fit in one disk sector: got %d bytes, need %d.", sections[0].Size, sectorSize)
	}

	var buf bytes.Buffer
	for _, section := range sections {
		// For every section, we add padding to
		// maintain the relative address spaces.
		padding := section.Addr - (sections[0].Addr + uint64(buf.Len()))
		for padding > 0 {
			n := padding
			if n > uint64(len(zeros)) {
				n = uint64(len(zeros))
			}

			buf.Write(zeros[:n])
			padding -= n
		}

		_, err := io.Copy(&buf, section.Open())
		if err != nil {
			log.Fatalf("Failed to copy section %q: %v", section.Name, err)
		}
	}

	// Ensure the bootloader fills an
	// integral number of disk sectors.
	written := int64(buf.Len())
	bootPadding := sectorSize - (written % sectorSize)
	if bootPadding == sectorSize {
		bootPadding = 0
	}

	written += bootPadding
	buf.Write(zeros[:bootPadding])

	symbols, err := bootloader.Symbols()
	if err != nil {
		log.Fatalf("Failed to read bootloader symbol table: %v", err)
	}

	var bootloaderEndSymAddr, kernelSizeAddr uint64
	const bootloaderEndSym = "firefly-os.dev/bootloader.bootloader-end"
	const kernelSizeSym = "firefly-os.dev/bootloader.kernel-size"
	for _, sym := range symbols {
		if sym.Name == bootloaderEndSym {
			if sym.Size != 4 {
				log.Fatalf("Symbol %s has size %d, want %d", sym.Name, sym.Size, 4)
			}

			bootloaderEndSymAddr = sym.Value
			continue
		}

		if sym.Name == kernelSizeSym {
			if sym.Size != 4 {
				log.Fatalf("Symbol %s has size %d, want %d", sym.Name, sym.Size, 4)
			}

			kernelSizeAddr = sym.Value
			continue
		}
	}

	if bootloaderEndSymAddr == 0 {
		log.Fatalf("Failed to find symbol %s in bootloader", bootloaderEndSym)
	}

	if kernelSizeAddr == 0 {
		log.Fatalf("Failed to find symbol %s in bootloader", kernelSizeSym)
	}

	// Overwrite the 32-bit address where
	// the bootloader ends.
	bootloaderEndAddr := sections[0].Addr + uint64(written)
	binary.LittleEndian.PutUint32(buf.Bytes()[bootloaderEndSymAddr-sections[0].Addr:], uint32(bootloaderEndAddr))

	stageTwoSize := written - sectorSize
	stageTwoSectors := (stageTwoSize + sectorSize - 1) / sectorSize
	if stageTwoSectors > maxStageTwoSectors {
		log.Fatalf("Stage two bootloader is too large: %d bytes (%d sectors)", stageTwoSize, stageTwoSectors)
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
	written += kernelSize
	kernelPadding := sectorSize - (written % sectorSize)
	if kernelPadding == sectorSize {
		kernelPadding = 0
	}

	written += kernelPadding

	// Overwrite the 32-bit address where
	// the kernel ends.
	binary.LittleEndian.PutUint32(buf.Bytes()[kernelSizeAddr-sections[0].Addr:], uint32(kernelSize+kernelPadding))

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

	err = out.Close()
	if err != nil {
		log.Fatalf("Failed to close %s: %v", outName, err)
	}
}
