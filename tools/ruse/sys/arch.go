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

	// Contains the full set of registers in
	// this architecture. The order of these
	// registers is arbitrary and may change.
	Registers []Location

	// Maps register names to their structured
	// data.
	RegisterNames map[string]Location

	// The set of all registers available to the
	// ABI. Typically, this will consist of the
	// architecture's full-size general purpose
	// registers, not including the instruction
	// pointer or stack pointer.
	ABIRegisters []Location

	// Internal cache of the ABI registers.
	abiRegisters map[Location]bool

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
	Registers: []Location{
		x86.AL, x86.CL, x86.DL, x86.BL, x86.AH, x86.CH, x86.DH, x86.BH,
		x86.AX, x86.CX, x86.DX, x86.BX, x86.BP, x86.SP, x86.DI, x86.SI,
		x86.EAX, x86.ECX, x86.EDX, x86.EBX, x86.EBP, x86.ESP, x86.EDI, x86.ESI,
		x86.BX_SI, x86.BX_DI, x86.BP_SI, x86.BP_DI,
		x86.IP, x86.EIP,
		x86.ES, x86.CS, x86.SS, x86.DS, x86.FS, x86.GS,
		x86.ST0, x86.ST1, x86.ST2, x86.ST3, x86.ST4, x86.ST5, x86.ST6, x86.ST7,
		x86.CR0, x86.CR1, x86.CR2, x86.CR3, x86.CR4, x86.CR5, x86.CR6, x86.CR7,
		x86.DR0, x86.DR1, x86.DR2, x86.DR3, x86.DR4, x86.DR5, x86.DR6, x86.DR7,
		x86.K0, x86.K1, x86.K2, x86.K3, x86.K4, x86.K5, x86.K6, x86.K7,
		x86.F0, x86.F1, x86.F2, x86.F3, x86.F4, x86.F5, x86.F6, x86.F7,
		x86.BND0, x86.BND1, x86.BND2,
		x86.MMX0, x86.MMX1, x86.MMX2, x86.MMX3, x86.MMX4, x86.MMX5, x86.MMX6, x86.MMX7,
		x86.TMM0, x86.TMM1, x86.TMM2, x86.TMM3, x86.TMM4, x86.TMM5, x86.TMM6, x86.TMM7,
		x86.XMM0, x86.XMM1, x86.XMM2, x86.XMM3, x86.XMM4, x86.XMM5, x86.XMM6, x86.XMM7,
		x86.YMM0, x86.YMM1, x86.YMM2, x86.YMM3, x86.YMM4, x86.YMM5, x86.YMM6, x86.YMM7,
	},
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
	Registers: []Location{
		x86.AL, x86.CL, x86.DL, x86.BL, x86.AH, x86.CH, x86.DH, x86.BH, x86.BPL, x86.SPL, x86.DIL, x86.SIL, x86.R8L, x86.R9L, x86.R10L, x86.R11L, x86.R12L, x86.R13L, x86.R14L, x86.R15L,
		x86.AX, x86.CX, x86.DX, x86.BX, x86.BP, x86.SP, x86.DI, x86.SI, x86.R8W, x86.R9W, x86.R10W, x86.R11W, x86.R12W, x86.R13W, x86.R14W, x86.R15W,
		x86.EAX, x86.ECX, x86.EDX, x86.EBX, x86.EBP, x86.ESP, x86.EDI, x86.ESI, x86.R8D, x86.R9D, x86.R10D, x86.R11D, x86.R12D, x86.R13D, x86.R14D, x86.R15D,
		x86.RAX, x86.RCX, x86.RDX, x86.RBX, x86.RBP, x86.RSP, x86.RDI, x86.RSI, x86.R8, x86.R9, x86.R10, x86.R11, x86.R12, x86.R13, x86.R14, x86.R15,
		x86.BX_SI, x86.BX_DI, x86.BP_SI, x86.BP_DI,
		x86.IP, x86.EIP, x86.RIP,
		x86.ES, x86.CS, x86.SS, x86.DS, x86.FS, x86.GS,
		x86.ST0, x86.ST1, x86.ST2, x86.ST3, x86.ST4, x86.ST5, x86.ST6, x86.ST7,
		x86.CR0, x86.CR1, x86.CR2, x86.CR3, x86.CR4, x86.CR5, x86.CR6, x86.CR7, x86.CR8, x86.CR9, x86.CR10, x86.CR11, x86.CR12, x86.CR13, x86.CR14, x86.CR15,
		x86.DR0, x86.DR1, x86.DR2, x86.DR3, x86.DR4, x86.DR5, x86.DR6, x86.DR7, x86.DR8, x86.DR9, x86.DR10, x86.DR11, x86.DR12, x86.DR13, x86.DR14, x86.DR15,
		x86.K0, x86.K1, x86.K2, x86.K3, x86.K4, x86.K5, x86.K6, x86.K7,
		x86.F0, x86.F1, x86.F2, x86.F3, x86.F4, x86.F5, x86.F6, x86.F7,
		x86.BND0, x86.BND1, x86.BND2,
		x86.MMX0, x86.MMX1, x86.MMX2, x86.MMX3, x86.MMX4, x86.MMX5, x86.MMX6, x86.MMX7,
		x86.TMM0, x86.TMM1, x86.TMM2, x86.TMM3, x86.TMM4, x86.TMM5, x86.TMM6, x86.TMM7,
		x86.XMM0, x86.XMM1, x86.XMM2, x86.XMM3, x86.XMM4, x86.XMM5, x86.XMM6, x86.XMM7,
		x86.XMM8, x86.XMM9, x86.XMM10, x86.XMM11, x86.XMM12, x86.XMM13, x86.XMM14, x86.XMM15,
		x86.XMM16, x86.XMM17, x86.XMM18, x86.XMM19, x86.XMM20, x86.XMM21, x86.XMM22, x86.XMM23,
		x86.XMM24, x86.XMM25, x86.XMM26, x86.XMM27, x86.XMM28, x86.XMM29, x86.XMM30, x86.XMM31,
		x86.YMM0, x86.YMM1, x86.YMM2, x86.YMM3, x86.YMM4, x86.YMM5, x86.YMM6, x86.YMM7,
		x86.YMM8, x86.YMM9, x86.YMM10, x86.YMM11, x86.YMM12, x86.YMM13, x86.YMM14, x86.YMM15,
		x86.YMM16, x86.YMM17, x86.YMM18, x86.YMM19, x86.YMM20, x86.YMM21, x86.YMM22, x86.YMM23,
		x86.YMM24, x86.YMM25, x86.YMM26, x86.YMM27, x86.YMM28, x86.YMM29, x86.YMM30, x86.YMM31,
		x86.ZMM0, x86.ZMM1, x86.ZMM2, x86.ZMM3, x86.ZMM4, x86.ZMM5, x86.ZMM6, x86.ZMM7,
		x86.ZMM8, x86.ZMM9, x86.ZMM10, x86.ZMM11, x86.ZMM12, x86.ZMM13, x86.ZMM14, x86.ZMM15,
		x86.ZMM16, x86.ZMM17, x86.ZMM18, x86.ZMM19, x86.ZMM20, x86.ZMM21, x86.ZMM22, x86.ZMM23,
		x86.ZMM24, x86.ZMM25, x86.ZMM26, x86.ZMM27, x86.ZMM28, x86.ZMM29, x86.ZMM30, x86.ZMM31,
	},
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

func init() {
	// Populate arch.abiRegisters and arch.RegisterNames.
	for _, arch := range All {
		arch.abiRegisters = make(map[Location]bool)
		for _, reg := range arch.ABIRegisters {
			arch.abiRegisters[reg] = true
		}

		arch.RegisterNames = make(map[string]Location)
		for _, reg := range arch.Registers {
			arch.RegisterNames[reg.String()] = reg
		}
	}
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
