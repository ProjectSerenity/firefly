// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package compiler

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"firefly-os.dev/tools/ruse/internal/x86"
)

func TestEncodeMemory(t *testing.T) {
	tests := []struct {
		Mode   x86.Mode
		Data   *x86InstructionData
		Memory *x86.Memory
		Want   *x86.Code
	}{
		// Displacement only.
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Displacement: 0},
			Want:   &x86.Code{ModRM: 0x06, Displacement: [8]byte{0x00, 0x00}, DisplacementLen: 2},
		},
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Displacement: 0x1122},
			Want:   &x86.Code{ModRM: 0x06, Displacement: [8]byte{0x22, 0x11}, DisplacementLen: 2},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Displacement: 0},
			Want:   &x86.Code{ModRM: 0x05, Displacement: [8]byte{0x00, 0x00, 0x00, 0x00}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x05, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Displacement: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x25, Displacement: [8]byte{0x00, 0x00, 0x00, 0x00}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x25, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},

		// 16-bit basic addresses, derived
		// from Section 2.1.5, Table 2-1.
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.BX_SI},
			Want:   &x86.Code{ModRM: 0x00},
		},
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.BX_DI},
			Want:   &x86.Code{ModRM: 0x01},
		},
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.BP_SI},
			Want:   &x86.Code{ModRM: 0x02},
		},
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.BP_DI},
			Want:   &x86.Code{ModRM: 0x03},
		},
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.SI},
			Want:   &x86.Code{ModRM: 0x04},
		},
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.DI},
			Want:   &x86.Code{ModRM: 0x05},
		},
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Displacement: 0x1122},
			Want:   &x86.Code{ModRM: 0x06, Displacement: [8]byte{0x22, 0x11}, DisplacementLen: 2},
		},
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.BX},
			Want:   &x86.Code{ModRM: 0x07},
		},

		// 16-bit addresses with an 8-bit
		// displacement, derived from Section
		// 2.1.5, Table 2-1.
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.BX_SI, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x40, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.BX_DI, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x41, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.BP_SI, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x42, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.BP_DI, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x43, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.SI, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.DI, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x45, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.BP, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x46, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.BX, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x47, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},

		// 16-bit addresses with a 16-bit
		// displacement, derived from Section
		// 2.1.5, Table 2-1.
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.BX_SI, Displacement: 0x1122},
			Want:   &x86.Code{ModRM: 0x80, Displacement: [8]byte{0x22, 0x11}, DisplacementLen: 2},
		},
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.BX_DI, Displacement: 0x1122},
			Want:   &x86.Code{ModRM: 0x81, Displacement: [8]byte{0x22, 0x11}, DisplacementLen: 2},
		},
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.BP_SI, Displacement: 0x1122},
			Want:   &x86.Code{ModRM: 0x82, Displacement: [8]byte{0x22, 0x11}, DisplacementLen: 2},
		},
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.BP_DI, Displacement: 0x1122},
			Want:   &x86.Code{ModRM: 0x83, Displacement: [8]byte{0x22, 0x11}, DisplacementLen: 2},
		},
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.SI, Displacement: 0x1122},
			Want:   &x86.Code{ModRM: 0x84, Displacement: [8]byte{0x22, 0x11}, DisplacementLen: 2},
		},
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.DI, Displacement: 0x1122},
			Want:   &x86.Code{ModRM: 0x85, Displacement: [8]byte{0x22, 0x11}, DisplacementLen: 2},
		},
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.BP, Displacement: 0x1122},
			Want:   &x86.Code{ModRM: 0x86, Displacement: [8]byte{0x22, 0x11}, DisplacementLen: 2},
		},
		{
			Mode:   x86.Mode16,
			Memory: &x86.Memory{Base: x86.BX, Displacement: 0x1122},
			Want:   &x86.Code{ModRM: 0x87, Displacement: [8]byte{0x22, 0x11}, DisplacementLen: 2},
		},

		// 32-bit basic addresses, derived
		// from Section 2.1.5, Table 2-2.
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EAX},
			Want:   &x86.Code{ModRM: 0x00},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ECX},
			Want:   &x86.Code{ModRM: 0x01},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDX},
			Want:   &x86.Code{ModRM: 0x02},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBX},
			Want:   &x86.Code{ModRM: 0x03},
		},
		// SIB forms are covered further down.
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x05, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBP},
			Want:   &x86.Code{ModRM: 0x45, Displacement: [8]byte{0x00}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESI},
			Want:   &x86.Code{ModRM: 0x06},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDI},
			Want:   &x86.Code{ModRM: 0x07},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESP},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x24},
		},

		// 32-bit basic addresses with an 8-bit
		// displacement, derived from Section
		// 2.1.5, Table 2-2.
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EAX, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x40, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ECX, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x41, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDX, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x42, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBX, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x43, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		// SIB forms are covered further down.
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBP, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x45, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESI, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x46, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDI, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x47, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESP, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x24, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},

		// 32-bit basic addresses with a 32-bit
		// displacement, derived from Section
		// 2.1.5, Table 2-2.
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EAX, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x80, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ECX, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x81, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDX, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x82, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBX, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x83, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		// SIB forms are covered further down.
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBP, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x85, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESI, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x86, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDI, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x87, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESP, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x24, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},

		// 32-bit scaled addresses, derived
		// from Section 2.1.5, Table 2-2.
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EAX, Index: x86.EAX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x00},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ECX, Index: x86.EAX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x01},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDX, Index: x86.EAX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x02},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBX, Index: x86.EAX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x03},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESP, Index: x86.EAX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x04},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Index: x86.EAX, Scale: 1, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x05, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESI, Index: x86.EAX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x06},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDI, Index: x86.EAX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x07},
		},
		// Moving down to the next index register.
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EAX, Index: x86.ECX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x08},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ECX, Index: x86.ECX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x09},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDX, Index: x86.ECX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0a},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBX, Index: x86.ECX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0b},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESP, Index: x86.ECX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0c},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Index: x86.ECX, Scale: 1, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0d, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESI, Index: x86.ECX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0e},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDI, Index: x86.ECX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0f},
		},
		// Moving down to the next index register.
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EAX, Index: x86.EDX, Scale: 2},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x50},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ECX, Index: x86.EDX, Scale: 2},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x51},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDX, Index: x86.EDX, Scale: 2},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x52},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBX, Index: x86.EDX, Scale: 2},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x53},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESP, Index: x86.EDX, Scale: 2},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x54},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Index: x86.EDX, Scale: 2, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x55, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESI, Index: x86.EDX, Scale: 2},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x56},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDI, Index: x86.EDX, Scale: 2},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x57},
		},
		// Moving down to the next index register.
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EAX, Index: x86.EBX, Scale: 4},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x98},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ECX, Index: x86.EBX, Scale: 4},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x99},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDX, Index: x86.EBX, Scale: 4},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x9a},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBX, Index: x86.EBX, Scale: 4},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x9b},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESP, Index: x86.EBX, Scale: 4},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x9c},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Index: x86.EBX, Scale: 4, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x9d, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESI, Index: x86.EBX, Scale: 4},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x9e},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDI, Index: x86.EBX, Scale: 4},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x9f},
		},
		// Moving down to the next index register.
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EAX, Index: x86.EBP, Scale: 8},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0xe8},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ECX, Index: x86.EBP, Scale: 8},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0xe9},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDX, Index: x86.EBP, Scale: 8},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0xea},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBX, Index: x86.EBP, Scale: 8},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0xeb},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESP, Index: x86.EBP, Scale: 8},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0xec},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Index: x86.EBP, Scale: 8, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0xed, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESI, Index: x86.EBP, Scale: 8},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0xee},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDI, Index: x86.EBP, Scale: 8},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0xef},
		},

		// And with an 8-bit displacement.
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EAX, Index: x86.ESI, Scale: 1, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x30, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ECX, Index: x86.ESI, Scale: 1, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x31, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDX, Index: x86.ESI, Scale: 1, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x32, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBX, Index: x86.ESI, Scale: 1, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x33, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESP, Index: x86.ESI, Scale: 1, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x34, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBP, Index: x86.ESI, Scale: 1, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x35, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBP, Index: x86.ESI, Scale: 1, Displacement: 0x00},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x35, Displacement: [8]byte{0x00}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESI, Index: x86.ESI, Scale: 1, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x36, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDI, Index: x86.ESI, Scale: 1, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x37, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},

		// And with a 32-bit displacement.
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EAX, Index: x86.EDI, Scale: 2, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x78, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ECX, Index: x86.EDI, Scale: 2, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x79, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDX, Index: x86.EDI, Scale: 2, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x7a, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBX, Index: x86.EDI, Scale: 2, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x7b, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESP, Index: x86.EDI, Scale: 2, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x7c, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBP, Index: x86.EDI, Scale: 2, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x7d, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESI, Index: x86.EDI, Scale: 2, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x7e, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDI, Index: x86.EDI, Scale: 2, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x7f, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},

		// 32-bit scaled addresses, but with
		// an implicit scale, derived from
		// Section 2.1.5, Table 2-2.
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EAX, Index: x86.EAX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x00},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ECX, Index: x86.EAX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x01},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDX, Index: x86.EAX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x02},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBX, Index: x86.EAX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x03},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESP, Index: x86.EAX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x04},
		},
		// We skip this form, as the assembler
		// would not produce it.
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESI, Index: x86.EAX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x06},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDI, Index: x86.EAX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x07},
		},
		// Moving down to the next index register.
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EAX, Index: x86.ECX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x08},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ECX, Index: x86.ECX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x09},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDX, Index: x86.ECX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0a},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBX, Index: x86.ECX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0b},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESP, Index: x86.ECX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0c},
		},
		// We skip this form, as the assembler
		// would not produce it.
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESI, Index: x86.ECX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0e},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDI, Index: x86.ECX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0f},
		},

		// And with an 8-bit displacement.
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EAX, Index: x86.ESI, Scale: 0, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x30, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ECX, Index: x86.ESI, Scale: 0, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x31, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDX, Index: x86.ESI, Scale: 0, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x32, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBX, Index: x86.ESI, Scale: 0, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x33, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESP, Index: x86.ESI, Scale: 0, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x34, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBP, Index: x86.ESI, Scale: 0, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x35, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBP, Index: x86.ESI, Scale: 0, Displacement: 0x00},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x35, Displacement: [8]byte{0x00}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESI, Index: x86.ESI, Scale: 0, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x36, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDI, Index: x86.ESI, Scale: 0, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x37, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},

		// And with a 32-bit displacement.
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EAX, Index: x86.EDI, Scale: 0, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x38, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ECX, Index: x86.EDI, Scale: 0, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x39, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDX, Index: x86.EDI, Scale: 0, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x3a, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBX, Index: x86.EDI, Scale: 0, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x3b, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESP, Index: x86.EDI, Scale: 0, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x3c, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EBP, Index: x86.EDI, Scale: 0, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x3d, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.ESI, Index: x86.EDI, Scale: 0, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x3e, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Base: x86.EDI, Index: x86.EDI, Scale: 0, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x3f, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},

		// 32-bit scaled addresses, but with
		// an no base or displacement, derived
		// from Section 2.1.5, Table 2-2.
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Index: x86.EAX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x05, Displacement: [8]byte{0x00, 0x00, 0x00, 0x00}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Index: x86.ECX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0d, Displacement: [8]byte{0x00, 0x00, 0x00, 0x00}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Index: x86.EDX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x15, Displacement: [8]byte{0x00, 0x00, 0x00, 0x00}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Index: x86.EBX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x1d, Displacement: [8]byte{0x00, 0x00, 0x00, 0x00}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Index: x86.EBP, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x2d, Displacement: [8]byte{0x00, 0x00, 0x00, 0x00}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Index: x86.ESI, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x35, Displacement: [8]byte{0x00, 0x00, 0x00, 0x00}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode32,
			Memory: &x86.Memory{Index: x86.EDI, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x3d, Displacement: [8]byte{0x00, 0x00, 0x00, 0x00}, DisplacementLen: 4},
		},

		// 64-bit basic addresses, derived
		// from Section 2.1.5, Table 2-2.
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RAX},
			Want:   &x86.Code{ModRM: 0x00},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RCX},
			Want:   &x86.Code{ModRM: 0x01},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDX},
			Want:   &x86.Code{ModRM: 0x02},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBX},
			Want:   &x86.Code{ModRM: 0x03},
		},
		// SIB forms are covered further down.
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x25, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBP},
			Want:   &x86.Code{ModRM: 0x45, Displacement: [8]byte{0x00}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSI},
			Want:   &x86.Code{ModRM: 0x06},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDI},
			Want:   &x86.Code{ModRM: 0x07},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSP},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x24},
		},

		// 64-bit basic addresses with an 8-bit
		// displacement, derived from Section
		// 2.1.5, Table 2-2.
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RAX, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x40, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RCX, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x41, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDX, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x42, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBX, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x43, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		// SIB forms are covered further down.
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBP, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x45, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSI, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x46, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDI, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x47, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSP, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x24, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},

		// 64-bit basic addresses with a 64-bit
		// displacement, derived from Section
		// 2.1.5, Table 2-2.
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RAX, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x80, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RCX, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x81, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDX, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x82, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBX, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x83, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		// SIB forms are covered further down.
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBP, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x85, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSI, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x86, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDI, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x87, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSP, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x24, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},

		// 64-bit scaled addresses, derived
		// from Section 2.1.5, Table 2-2.
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RAX, Index: x86.RAX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x00},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RCX, Index: x86.RAX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x01},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDX, Index: x86.RAX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x02},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBX, Index: x86.RAX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x03},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSP, Index: x86.RAX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x04},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Index: x86.RAX, Scale: 1, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x05, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSI, Index: x86.RAX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x06},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDI, Index: x86.RAX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x07},
		},
		// Moving down to the next index register.
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RAX, Index: x86.RCX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x08},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RCX, Index: x86.RCX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x09},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDX, Index: x86.RCX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0a},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBX, Index: x86.RCX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0b},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSP, Index: x86.RCX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0c},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Index: x86.RCX, Scale: 1, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0d, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSI, Index: x86.RCX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0e},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDI, Index: x86.RCX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0f},
		},
		// Moving down to the next index register.
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RAX, Index: x86.RDX, Scale: 2},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x50},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RCX, Index: x86.RDX, Scale: 2},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x51},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDX, Index: x86.RDX, Scale: 2},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x52},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBX, Index: x86.RDX, Scale: 2},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x53},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSP, Index: x86.RDX, Scale: 2},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x54},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Index: x86.RDX, Scale: 2, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x55, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSI, Index: x86.RDX, Scale: 2},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x56},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDI, Index: x86.RDX, Scale: 2},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x57},
		},
		// Moving down to the next index register.
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RAX, Index: x86.RBX, Scale: 4},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x98},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RCX, Index: x86.RBX, Scale: 4},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x99},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDX, Index: x86.RBX, Scale: 4},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x9a},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBX, Index: x86.RBX, Scale: 4},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x9b},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSP, Index: x86.RBX, Scale: 4},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x9c},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Index: x86.RBX, Scale: 4, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x9d, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSI, Index: x86.RBX, Scale: 4},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x9e},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDI, Index: x86.RBX, Scale: 4},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x9f},
		},
		// Moving down to the next index register.
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RAX, Index: x86.RBP, Scale: 8},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0xe8},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RCX, Index: x86.RBP, Scale: 8},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0xe9},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDX, Index: x86.RBP, Scale: 8},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0xea},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBX, Index: x86.RBP, Scale: 8},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0xeb},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSP, Index: x86.RBP, Scale: 8},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0xec},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Index: x86.RBP, Scale: 8, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0xed, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSI, Index: x86.RBP, Scale: 8},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0xee},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDI, Index: x86.RBP, Scale: 8},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0xef},
		},

		// And with an 8-bit displacement.
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RAX, Index: x86.RSI, Scale: 1, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x30, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RCX, Index: x86.RSI, Scale: 1, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x31, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDX, Index: x86.RSI, Scale: 1, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x32, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBX, Index: x86.RSI, Scale: 1, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x33, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSP, Index: x86.RSI, Scale: 1, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x34, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBP, Index: x86.RSI, Scale: 1, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x35, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBP, Index: x86.RSI, Scale: 1, Displacement: 0x00},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x35, Displacement: [8]byte{0x00}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSI, Index: x86.RSI, Scale: 1, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x36, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDI, Index: x86.RSI, Scale: 1, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x37, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},

		// And with a 64-bit displacement.
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RAX, Index: x86.RDI, Scale: 2, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x78, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RCX, Index: x86.RDI, Scale: 2, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x79, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDX, Index: x86.RDI, Scale: 2, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x7a, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBX, Index: x86.RDI, Scale: 2, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x7b, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSP, Index: x86.RDI, Scale: 2, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x7c, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBP, Index: x86.RDI, Scale: 2, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x7d, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSI, Index: x86.RDI, Scale: 2, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x7e, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDI, Index: x86.RDI, Scale: 2, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x7f, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},

		// 64-bit scaled addresses, but with
		// an implicit scale, derived from
		// Section 2.1.5, Table 2-2.
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RAX, Index: x86.RAX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x00},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RCX, Index: x86.RAX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x01},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDX, Index: x86.RAX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x02},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBX, Index: x86.RAX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x03},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSP, Index: x86.RAX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x04},
		},
		// We skip this form, as the assembler
		// would not produce it.
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSI, Index: x86.RAX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x06},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDI, Index: x86.RAX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x07},
		},
		// Moving down to the next index register.
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RAX, Index: x86.RCX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x08},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RCX, Index: x86.RCX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x09},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDX, Index: x86.RCX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0a},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBX, Index: x86.RCX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0b},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSP, Index: x86.RCX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0c},
		},
		// We skip this form, as the assembler
		// would not produce it.
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSI, Index: x86.RCX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0e},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDI, Index: x86.RCX, Scale: 0},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0f},
		},

		// And with an 8-bit displacement.
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RAX, Index: x86.RSI, Scale: 0, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x30, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RCX, Index: x86.RSI, Scale: 0, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x31, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDX, Index: x86.RSI, Scale: 0, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x32, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBX, Index: x86.RSI, Scale: 0, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x33, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSP, Index: x86.RSI, Scale: 0, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x34, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBP, Index: x86.RSI, Scale: 0, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x35, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBP, Index: x86.RSI, Scale: 0, Displacement: 0x00},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x35, Displacement: [8]byte{0x00}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSI, Index: x86.RSI, Scale: 0, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x36, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDI, Index: x86.RSI, Scale: 0, Displacement: 0x11},
			Want:   &x86.Code{ModRM: 0x44, SIB: 0x37, Displacement: [8]byte{0x11}, DisplacementLen: 1},
		},

		// And with a 64-bit displacement.
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RAX, Index: x86.RDI, Scale: 0, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x38, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RCX, Index: x86.RDI, Scale: 0, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x39, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDX, Index: x86.RDI, Scale: 0, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x3a, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBX, Index: x86.RDI, Scale: 0, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x3b, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSP, Index: x86.RDI, Scale: 0, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x3c, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RBP, Index: x86.RDI, Scale: 0, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x3d, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RSI, Index: x86.RDI, Scale: 0, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x3e, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Base: x86.RDI, Index: x86.RDI, Scale: 0, Displacement: 0x11223344},
			Want:   &x86.Code{ModRM: 0x84, SIB: 0x3f, Displacement: [8]byte{0x44, 0x33, 0x22, 0x11}, DisplacementLen: 4},
		},

		// 64-bit scaled addresses, but with
		// an no base or displacement, derived
		// from Section 2.1.5, Table 2-2.
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Index: x86.RAX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x05, Displacement: [8]byte{0x00, 0x00, 0x00, 0x00}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Index: x86.RCX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x0d, Displacement: [8]byte{0x00, 0x00, 0x00, 0x00}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Index: x86.RDX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x15, Displacement: [8]byte{0x00, 0x00, 0x00, 0x00}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Index: x86.RBX, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x1d, Displacement: [8]byte{0x00, 0x00, 0x00, 0x00}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Index: x86.RBP, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x2d, Displacement: [8]byte{0x00, 0x00, 0x00, 0x00}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Index: x86.RSI, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x35, Displacement: [8]byte{0x00, 0x00, 0x00, 0x00}, DisplacementLen: 4},
		},
		{
			Mode:   x86.Mode64,
			Memory: &x86.Memory{Index: x86.RDI, Scale: 1},
			Want:   &x86.Code{ModRM: 0x04, SIB: 0x3d, Displacement: [8]byte{0x00, 0x00, 0x00, 0x00}, DisplacementLen: 4},
		},
	}

	for _, test := range tests {
		var code x86.Code
		code.VEX.Default()
		code.EVEX.Default()
		data := test.Data
		if data == nil {
			data = &x86InstructionData{Inst: x86.AAD}
		}

		err := data.encodeMemory(&code, test.Mode, test.Memory)
		if err != nil {
			t.Errorf("%d-bit mode: encodeMemory(%s): %v", test.Mode.Int, test.Memory, err)
			continue
		}

		// Reset (E)VEX as we do in x86Instruction.Encode.
		if !code.VEX.On() {
			code.VEX.Reset()
		}
		if !code.EVEX.On() {
			code.EVEX.Reset()
		}

		if diff := cmp.Diff(test.Want, &code); diff != "" {
			t.Errorf("%d-bit mode: encodeMemory(%s): (-want, +got)\n%s", test.Mode.Int, test.Memory, diff)
			continue
		}
	}
}
