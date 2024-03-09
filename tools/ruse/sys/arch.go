// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package sys defines the characteristics of machine architectures.
package sys

import (
	"encoding/binary"
	"fmt"

	"firefly-os.dev/tools/ruse/internal/x86"
)

// Arch defines the characteristics of a machine architecture.
//
// An architecture with no Arch data is not implemented by
// this toolchain.
type Arch struct {
	Name   string
	Family ArchFamily

	PointerSize  int // The size of a memory address in bytes.
	RegisterSize int // The capacity of a general-purpose register in bytes.
	LocationSize int // The capacity of each memory location in bytes.
	MaxAlignment int
	ByteOrder    binary.ByteOrder

	// ABI details.

	// The set of all registers available to the
	// ABI. Typically, this will consist of the
	// architecture's full-size general purpose
	// registers, not including the instruction
	// pointer or stack pointer.
	ABIRegisters []Location

	// A mapping of child registers to their
	// parent registers. That is, writes to
	// the key register will affect the contents
	// of all value registers.
	ParentRegisters map[Location][]Location

	// The architecture's stack register.
	StackPointer Location

	// Whether the stack grows downward. If
	// true, successive stack locations will
	// have smaller addresses.
	StackGrowsDown bool

	// The alignment of the stack at the point
	// of a function call in bytes. This may
	// then be aligned differently if the architecture
	// pushes the return address onto the stack.
	//
	// If there is no guaranteed stack alignment,
	// the alignment will be zero.
	StackAlignment int

	// The ABI to use if none is specified.
	DefaultABI ABI
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
	LocationSize: 4,
	MaxAlignment: 1,
	ByteOrder:    binary.LittleEndian,
	ABIRegisters: []Location{x86.EAX, x86.ECX, x86.EDX, x86.EBX, x86.EBP, x86.ESI, x86.EDI},
	ParentRegisters: map[Location][]Location{
		// 8-bit registers.
		x86.AL: {x86.AX, x86.EAX},
		x86.CL: {x86.CX, x86.ECX},
		x86.DL: {x86.DX, x86.EDX},
		x86.BL: {x86.BX, x86.EBX},
		x86.AH: {x86.AX, x86.EAX},
		x86.CH: {x86.CX, x86.ECX},
		x86.DH: {x86.DX, x86.EDX},
		x86.BH: {x86.BX, x86.EBX},

		// 16-bit registers.
		x86.AX: {x86.EAX},
		x86.CX: {x86.ECX},
		x86.DX: {x86.EDX},
		x86.BX: {x86.EBX},
		x86.SP: {x86.ESP},
		x86.BP: {x86.EBP},
		x86.SI: {x86.ESI},
		x86.DI: {x86.EDI},
		x86.IP: {x86.EIP},
	},
	StackPointer:   x86.ESP,
	StackGrowsDown: true,
	StackAlignment: 0,
	DefaultABI: ABI{
		ParamRegisters:   nil, // All parameters are passed on the stack.
		ResultRegisters:  []Location{x86.EAX, x86.EDX},
		ScratchRegisters: []Location{x86.EAX, x86.ECX, x86.EDX},
		UnusedRegisters:  []Location{x86.EBX, x86.ESI, x86.EDI, x86.EBP, x86.ESP},
	},
}

var X86_64 = &Arch{
	Name:         "x86-64",
	Family:       FamilyX86_64,
	PointerSize:  8,
	RegisterSize: 8,
	LocationSize: 8,
	MaxAlignment: 1,
	ByteOrder:    binary.LittleEndian,
	ABIRegisters: []Location{x86.RAX, x86.RCX, x86.RDX, x86.RBX, x86.RBP, x86.RSI, x86.RDI, x86.R8, x86.R9, x86.R10, x86.R11, x86.R12, x86.R13, x86.R14, x86.R15},
	ParentRegisters: map[Location][]Location{
		// 8-bit registers.
		x86.AL:   {x86.AX, x86.EAX, x86.RAX},
		x86.CL:   {x86.CX, x86.ECX, x86.RCX},
		x86.DL:   {x86.DX, x86.EDX, x86.RDX},
		x86.BL:   {x86.BX, x86.EBX, x86.RBX},
		x86.AH:   {x86.AX, x86.EAX, x86.RAX},
		x86.CH:   {x86.CX, x86.ECX, x86.RCX},
		x86.DH:   {x86.DX, x86.EDX, x86.RDX},
		x86.BH:   {x86.BX, x86.EBX, x86.RBX},
		x86.SPL:  {x86.SP, x86.ESP, x86.RSP},
		x86.BPL:  {x86.BP, x86.EBP, x86.RBP},
		x86.SIL:  {x86.SI, x86.ESI, x86.RSI},
		x86.DIL:  {x86.DI, x86.EDI, x86.RDI},
		x86.R8L:  {x86.R8W, x86.R8D, x86.R8},
		x86.R9L:  {x86.R9W, x86.R9D, x86.R9},
		x86.R10L: {x86.R10W, x86.R10D, x86.R10},
		x86.R11L: {x86.R11W, x86.R11D, x86.R11},
		x86.R12L: {x86.R12W, x86.R12D, x86.R12},
		x86.R13L: {x86.R13W, x86.R13D, x86.R13},
		x86.R14L: {x86.R14W, x86.R14D, x86.R14},
		x86.R15L: {x86.R15W, x86.R15D, x86.R15},

		// 16-bit registers.
		x86.AX:   {x86.EAX, x86.RAX},
		x86.CX:   {x86.ECX, x86.RCX},
		x86.DX:   {x86.EDX, x86.RDX},
		x86.BX:   {x86.EBX, x86.RBX},
		x86.SP:   {x86.ESP, x86.RSP},
		x86.BP:   {x86.EBP, x86.RBP},
		x86.SI:   {x86.ESI, x86.RSI},
		x86.DI:   {x86.EDI, x86.RDI},
		x86.R8W:  {x86.R8D, x86.R8},
		x86.R9W:  {x86.R9D, x86.R9},
		x86.R10W: {x86.R10D, x86.R10},
		x86.R11W: {x86.R11D, x86.R11},
		x86.R12W: {x86.R12D, x86.R12},
		x86.R13W: {x86.R13D, x86.R13},
		x86.R14W: {x86.R14D, x86.R14},
		x86.R15W: {x86.R15D, x86.R15},
		x86.IP:   {x86.EIP, x86.RIP},

		// 32-bit registers.
		x86.EAX:  {x86.RAX},
		x86.ECX:  {x86.RCX},
		x86.EDX:  {x86.RDX},
		x86.EBX:  {x86.RBX},
		x86.ESP:  {x86.RSP},
		x86.EBP:  {x86.RBP},
		x86.ESI:  {x86.RSI},
		x86.EDI:  {x86.RDI},
		x86.R8D:  {x86.R8},
		x86.R9D:  {x86.R9},
		x86.R10D: {x86.R10},
		x86.R11D: {x86.R11},
		x86.R12D: {x86.R12},
		x86.R13D: {x86.R13},
		x86.R14D: {x86.R14},
		x86.R15D: {x86.R15},
		x86.EIP:  {x86.RIP},
	},
	StackPointer:   x86.RSP,
	StackGrowsDown: true,
	StackAlignment: 16,
	DefaultABI: ABI{
		ParamRegisters:   []Location{x86.RDI, x86.RSI, x86.RDX, x86.RCX, x86.R8, x86.R9},
		ResultRegisters:  []Location{x86.RAX, x86.RDX},
		ScratchRegisters: []Location{x86.RAX, x86.RDI, x86.RSI, x86.RDX, x86.RCX, x86.R8, x86.R9, x86.R10, x86.R11},
		UnusedRegisters:  []Location{x86.RBX, x86.RSP, x86.RBP, x86.R12, x86.R13, x86.R14, x86.R15},
	},
}

// All is a list of all supported architectures.
var All = [...]*Arch{
	X86,
	X86_64,
}

// ArchByName maps architecture names to their
// metadata.
var ArchByName = map[string]*Arch{
	X86.Name:    X86,
	X86_64.Name: X86_64,
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
