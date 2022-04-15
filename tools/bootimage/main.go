// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command bootimage turns the bootloader and kernel into a bootable disk image.
//
package main

import (
	"debug/elf"
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
	if len(os.Args) != 3 {
		log.Printf("Usage:\n  %s IN OUT", filepath.Base(os.Args[0]))
		os.Exit(2)
	}

	in := os.Args[1]
	out := os.Args[2]
	f, err := elf.Open(in)
	if err != nil {
		log.Fatalf("failed to read %q: %v", in, err)
	}

	defer f.Close()

	image, err := os.Create(out)
	if err != nil {
		log.Fatalf("failed to create %q: %v", out, err)
	}

	// Check that the stage two bootloader
	// is small enough to be loaded into
	// memory. This means checking that it
	// fits in fewer than 128 512-byte disk
	// sectors.
	const (
		stageTwoStart      = "_rest_of_bootloader_start_addr"
		stageTwoEnd        = "_rest_of_bootloader_end_addr"
		sectorSize         = 512
		maxStageTwoSectors = 127
	)

	symbols, err := f.Symbols()
	if err != nil {
		log.Fatalf("failed to parse symbol table: %v", err)
	}

	var startSymbol, endSymbol elf.Symbol
	for _, sym := range symbols {
		if sym.Name == stageTwoStart {
			startSymbol = sym
		} else if sym.Name == stageTwoEnd {
			endSymbol = sym
		}
	}

	if startSymbol.Value == 0 || endSymbol.Value == 0 {
		log.Fatalf("failed to find stage two bootloader")
	}
	if startSymbol.Value > endSymbol.Value {
		log.Fatalf("invalid stage two bootloader: region %#x-%#x", startSymbol.Value, endSymbol.Value)
	}

	stageTwoSize := endSymbol.Value - startSymbol.Value
	stageTwoSectors := (stageTwoSize + sectorSize - 1) / sectorSize
	if stageTwoSectors > maxStageTwoSectors {
		log.Fatalf("stage two bootloader is too large: %d bytes (%d sectors)", stageTwoSize, stageTwoSectors)
	}

	entry := f.Entry
	segments := make([]*elf.Prog, 0, len(f.Progs))
	for _, prog := range f.Progs {
		// Only consider parts of the binary that
		// end up in memory.
		if prog.Type != elf.PT_LOAD {
			continue
		}

		// Ignore segments prior to the entry point.
		if prog.Vaddr < entry {
			continue
		}

		segments = append(segments, prog)
	}

	if len(segments) == 0 {
		log.Fatalf("no valid program segments found")
	}

	sort.Slice(segments, func(i, j int) bool { return segments[i].Vaddr < segments[j].Vaddr })

	var written int64
	for _, segment := range segments {
		n, err := io.Copy(image, segment.Open())
		if err != nil {
			log.Fatalf("failed to copy segment: %v", err)
		}

		written += n
	}

	// Round the image size up to the next multiple
	// of 512.
	remaining := blockSize - (written % blockSize)
	if remaining != blockSize {
		_, err = image.Write(zeros[:remaining])
		if err != nil {
			log.Fatalf("failed to pad image: %v", err)
		}
	}

	err = image.Close()
	if err != nil {
		log.Fatalf("failed to close image: %v", err)
	}
}
