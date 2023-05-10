// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package sys defines the characteristics of machine architectures.
package sys

import (
	"encoding/binary"
	"fmt"
)

// Arch defines the characteristics of a machine architecture.
//
// An architecture with no Arch data is not implemented by
// this toolchain.
type Arch struct {
	Name   string
	Family ArchFamily

	PointerSize  int
	RegisterSize int
	MaxAlignment int
	ByteOrder    binary.ByteOrder
}

// ReadPointer returns a pointer from the given machine
// code, according to the architecture's pointer size and
// byte order.
func (a *Arch) ReadPointer(b []byte) uintptr {
	switch a.PointerSize {
	case 4:
		return uintptr(a.ByteOrder.Uint32(b))
	case 8:
		return uintptr(a.ByteOrder.Uint64(b))
	default:
		panic(fmt.Sprintf("architecture %s has unexpected pointer size %d", a.Name, a.PointerSize))
	}
}

// WritePointer writes a pointer to the given machine code,
// according to the architecture's pointer size and byte
// order.
func (a *Arch) WritePointer(b []byte, ptr uintptr) {
	switch a.PointerSize {
	case 4:
		a.ByteOrder.PutUint32(b, uint32(ptr))
	case 8:
		a.ByteOrder.PutUint64(b, uint64(ptr))
	default:
		panic(fmt.Sprintf("architecture %s has unexpected pointer size %d", a.Name, a.PointerSize))
	}
}

var X86 = &Arch{
	Name:         "x86",
	Family:       FamilyX86,
	PointerSize:  4,
	RegisterSize: 4,
	MaxAlignment: 1,
	ByteOrder:    binary.LittleEndian,
}

var X86_64 = &Arch{
	Name:         "x86-64",
	Family:       FamilyX86_64,
	PointerSize:  8,
	RegisterSize: 8,
	MaxAlignment: 1,
	ByteOrder:    binary.LittleEndian,
}

// All is a list of all supported architectures.
var All = [...]*Arch{
	X86,
	X86_64,
}

// ArchFamily represents a group of related machine
// architectures. For example, ppc64 and ppc64le are
// in the same group.
type ArchFamily uint8

const (
	FamilyNone ArchFamily = iota
	FamilyX86
	FamilyX86_64
)
