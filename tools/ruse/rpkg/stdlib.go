// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package rpkg

import (
	"fmt"
	"io"
	"slices"
	"sort"

	"golang.org/x/crypto/cryptobyte"

	"firefly-os.dev/tools/ruse/compiler"
	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/types"
)

const (
	rstdMagic   uint32 = 0x72737464 // "rstd"
	rstdVersion uint8  = 1
)

// StdlibHeader contains the information from an
// rstd header.
type StdlibHeader struct {
	// Details about the rpkg file.
	Magic        uint32 // The magic value that identifies an rstd file. (value: "rstd")
	Architecture Arch   // The architecture this file targets (defined below).
	Version      uint8  // The rstd file format version (value: rpkg.rstdVersion).
	NumPackages  uint16 // The number of packages.
}

const rstdHeaderSize = 4 + // 32-bit magic.
	1 + // 8-bit architecture.
	1 + // 8-bit version.
	2 // 16-bit package count.

// StdlibPackageHeader describes a standard library
// package in an rstd file.
type StdlibPackageHeader struct {
	PackageName string
	DataOffset  uint64
	DataLength  uint64
}

// EncodeStdlib is used to create a composite
// rstd file containing the rpkg files for
// each package.
func EncodeStdlib(w io.Writer, arch *sys.Arch, rpkgs [][]byte) error {
	var architecture Arch
	switch arch {
	case sys.X86_64:
		architecture = ArchX86_64
	default:
		return fmt.Errorf("unsupported architecture: %v", arch)
	}

	// First, we check the header for each
	// rpkg file so that we can be sure they
	// are valid and so we can sort them by
	// package path.
	type File struct {
		Path string
		Data []byte
	}

	sumNames := 0
	files := make([]File, len(rpkgs))
	seenPackage := make(map[string]bool)
	for i, rpkg := range rpkgs {
		d, err := NewDecoder(rpkg)
		if err != nil {
			return fmt.Errorf("failed to parse rpkg file %d: %v", i, err)
		}

		if seenPackage[d.packageName] {
			return fmt.Errorf("invalid rpkg file %d: package %q already seen", i, d.packageName)
		}

		sumNames += 2 + // Package name length.
			len(d.packageName) + // Package name.
			8 + // Package rpkg offset.
			8 // Package rpkg length.
		if sumNames%8 != 0 {
			sumNames += 8 - (sumNames % 8) // Padding.
		}

		seenPackage[d.packageName] = true
		files[i] = File{
			Path: d.packageName,
			Data: rpkg,
		}
	}

	// Sort the packages.
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	// Build up the header.
	offset := rstdHeaderSize + uint64(sumNames)
	b := cryptobyte.NewFixedBuilder(make([]byte, 0, offset))
	b.AddUint32(rstdMagic)
	b.AddUint8(uint8(architecture))
	b.AddUint8(rstdVersion)
	b.AddUint16(uint16(len(files)))
	for _, file := range files {
		b.AddUint16LengthPrefixed(func(b *cryptobyte.Builder) {
			b.AddBytes([]byte(file.Path))
		})

		unpadded := 2 + len(file.Path)
		if unpadded%8 != 0 {
			padding := 8 - (unpadded % 8)
			for i := 0; i < padding; i++ {
				b.AddUint8(0)
			}
		}

		b.AddUint64(offset)
		b.AddUint64(uint64(len(file.Data)))
		offset += uint64(len(file.Data))
	}

	// Write the header and the rpkg files.
	header := b.BytesOrPanic()
	_, err := w.Write(header)
	if err != nil {
		return fmt.Errorf("failed to write rstd header: %v", err)
	}

	for _, file := range files {
		_, err = w.Write(file.Data)
		if err != nil {
			return fmt.Errorf("failed to write rpkg for %q: %v", file.Path, err)
		}
	}

	return nil
}

// StdlibDecoder can be used to extract the
// package information from an rstd file.
type StdlibDecoder struct {
	b []byte

	arch *sys.Arch

	header  StdlibHeader
	headers []StdlibPackageHeader
}

// Header returns the rstd header.
func (d *StdlibDecoder) Header() *StdlibHeader {
	return &d.header
}

// Packages returns the list of packages.
func (d *StdlibDecoder) Packages() []StdlibPackageHeader {
	return d.headers
}

// Extract returns the rpkg data for the given
// package.
func (d *StdlibDecoder) Extract(hdr StdlibPackageHeader) []byte {
	return d.b[hdr.DataOffset : hdr.DataOffset+hdr.DataLength]
}

// Decode parses the rpkg data for the given
// package. The types in the package are populated in
// the type information. Specifically, the List and
// Indices fields.
func (d *StdlibDecoder) Decode(info *types.Info, hdr StdlibPackageHeader) (arch *sys.Arch, pkg *compiler.Package, checksum []byte, err error) {
	b := d.Extract(hdr)
	return Decode(info, b)
}

// NewStdlibDecoder helps parse an rstd file into
// compiled packages.
func NewStdlibDecoder(b []byte) (*StdlibDecoder, error) {
	d := &StdlibDecoder{
		b: b,
	}

	if len(b) < rstdHeaderSize {
		return nil, fmt.Errorf("invalid rstd header: %w", io.ErrUnexpectedEOF)
	}

	s := cryptobyte.String(b)

	// Decode the header.
	var arch uint8
	if !s.ReadUint32(&d.header.Magic) ||
		!s.ReadUint8(&arch) ||
		!s.ReadUint8(&d.header.Version) ||
		!s.ReadUint16(&d.header.NumPackages) {
		return nil, fmt.Errorf("rpkg: internal error: failed to read rstd header: %w", io.ErrUnexpectedEOF)
	}

	// Sanity-check the header.
	if d.header.Magic != rstdMagic {
		return nil, fmt.Errorf("invalid rstd header: got magic %x, want %x", d.header.Magic, rstdMagic)
	}

	d.header.Architecture = Arch(arch)
	switch d.header.Architecture {
	case ArchX86_64:
		d.arch = sys.X86_64
	default:
		return nil, fmt.Errorf("invalid rstd header: unrecognised architecture %d", d.header.Architecture)
	}

	if d.header.Version != rstdVersion {
		return nil, fmt.Errorf("unsupported rstd header: got version %d, but only %d is supported", d.header.Version, rstdVersion)
	}

	if d.header.NumPackages == 0 {
		return nil, fmt.Errorf("invalid rstd header: no packages")
	}

	d.headers = make([]StdlibPackageHeader, d.header.NumPackages)
	for i := 0; i < int(d.header.NumPackages); i++ {
		var name cryptobyte.String
		if !s.ReadUint16LengthPrefixed(&name) {
			return nil, fmt.Errorf("invalid rstd header: failed to read package %d name: %w", i+1, io.ErrUnexpectedEOF)
		}

		// Read any padding.
		unpadded := 2 + len(name)
		if unpadded%8 != 0 {
			padding := 8 - (unpadded % 8)
			var got []byte
			if !s.ReadBytes(&got, padding) {
				return nil, fmt.Errorf("invalid rstd header: failed to read package %d name padding: %w", i+1, io.ErrUnexpectedEOF)
			}

			if !slices.Equal(got, make([]byte, padding)) {
				return nil, fmt.Errorf("invalid rstd header: invalid package %d name padding: non-zero padding % x", i+1, got)
			}
		}

		var offset, length uint64
		if !s.ReadUint64(&offset) ||
			!s.ReadUint64(&length) {
			return nil, fmt.Errorf("invalid rstd header: failed to read package %d location: %w", i+1, io.ErrUnexpectedEOF)
		}

		d.headers[i] = StdlibPackageHeader{
			PackageName: string(name),
			DataOffset:  offset,
			DataLength:  length,
		}
	}

	// Check the offsets are contiguous and sequential,
	// with no gap afterwards.
	want := uint64(len(b) - len(s)) // The first byte after the package headers.
	for _, hdr := range d.headers {
		if hdr.DataOffset != want {
			return nil, fmt.Errorf("invalid rstd header: got package %q offset %#x, want %#x", hdr.PackageName, hdr.DataOffset, want)
		}

		want += hdr.DataLength
	}

	if want != uint64(len(b)) {
		return nil, fmt.Errorf("invalid rstd header: got final offset %#x, but %#x bytes of data", want, len(b))
	}

	return d, nil
}
