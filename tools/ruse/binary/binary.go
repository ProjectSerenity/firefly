// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package binary describes the structure of a
// Ruse binary, irrespective of encoding format.
package binary

import (
	"firefly-os.dev/tools/ruse/sys"
)

// Binary represents a compiled Ruse binary.
type Binary struct {
	Arch     *sys.Arch
	BaseAddr uintptr // Binary base address.
	Sections []*Section
}

// Section describes a single logical section
// in a compile Ruse binary.
type Section struct {
	Name        string      // The section name.
	Address     uintptr     // The section's address in memory.
	IsZeroed    bool        // Whether the section's contents are all zeros.
	Permissions Permissions // The section's runtime permissions.
	Data        []byte      // The section data.
}

// Permissions indicate the runtime permissions
// of a binary section.
type Permissions uint8

const (
	Read Permissions = 1 << iota
	Write
	Execute
)

func (p Permissions) Read() bool    { return p&Read != 0 }
func (p Permissions) Write() bool   { return p&Write != 0 }
func (p Permissions) Execute() bool { return p&Execute != 0 }

func (p Permissions) String() string {
	s := [3]byte{'-', '-', '-'}
	if p.Read() {
		s[0] = 'R'
	}
	if p.Write() {
		s[1] = 'W'
	}
	if p.Execute() {
		s[2] = 'X'
	}

	return string(s[:])
}