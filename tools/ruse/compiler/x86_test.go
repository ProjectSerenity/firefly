// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package compiler

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/internal/x86"
	"firefly-os.dev/tools/ruse/parser"
	"firefly-os.dev/tools/ruse/ssafir"
	"firefly-os.dev/tools/ruse/sys"
	"firefly-os.dev/tools/ruse/token"
	"firefly-os.dev/tools/ruse/types"
)

// x86REX is a helper function for creating
// REX prefixes in tests.
func x86REX(s string) x86.REX {
	var out x86.REX
	out.SetOn()
	for _, r := range s {
		switch r {
		case 'W':
			out.SetW(true)
		case 'R':
			out.SetR(true)
		case 'X':
			out.SetX(true)
		case 'B':
			out.SetB(true)
		default:
			panic(fmt.Sprintf("invalid REX value %c", r))
		}
	}

	return out
}

type x86TestCase struct {
	Name           string
	Mode           x86.Mode
	Assembly       string
	AssemblyError  string
	Op             ssafir.Op
	Data           *x86InstructionData
	EncodingErrror string
	Code           *x86.Code
}

var x86TestCases = []*x86TestCase{
	{
		Name:     "ret",
		Mode:     x86.Mode64,
		Assembly: "(ret)",
		Op:       ssafir.OpX86RET,
		Data: &x86InstructionData{
			Length: 1,
		},
		Code: &x86.Code{
			Opcode:    [3]byte{0xc3},
			OpcodeLen: 1,
		},
	},
	{
		Name:     "shift right",
		Mode:     x86.Mode64,
		Assembly: "(shr ecx 18)",
		Op:       ssafir.OpX86SHR_Rmr32_Imm8u,
		Data: &x86InstructionData{
			Args: [4]any{
				x86.ECX,
				uint64(18),
			},
			Length: 3,
		},
		Code: &x86.Code{
			Opcode:       [3]byte{0xc1},
			OpcodeLen:    1,
			UseModRM:     true,
			ModRM:        x86.ModRMmodRegister | x86.ModRMreg101 | x86.ModRMrm001,
			Immediate:    [8]byte{0x12},
			ImmediateLen: 1,
		},
	},
	{
		Name:     "small displaced adc register pair",
		Mode:     x86.Mode64,
		Assembly: "(adc '(bits 8)(+ bx si) cl)",
		Op:       ssafir.OpX86ADC_M8_R8,
		Data: &x86InstructionData{
			Args:   [4]any{&x86.Memory{Base: x86.BX_SI}, x86.CL},
			Length: 3,
		},
		Code: &x86.Code{
			Prefixes:  [14]x86.Prefix{x86.PrefixAddressSize},
			Opcode:    [3]byte{0x10},
			OpcodeLen: 1,
			UseModRM:  true,
			ModRM:     x86.ModRMmodDereferenceRegister | x86.ModRMreg001 | x86.ModRMrm000,
		},
	},
	{
		Name:     "small displaced adc segment offset",
		Mode:     x86.Mode64,
		Assembly: "(adc '(bytes 1)(+ es bp 0x7) cl)",
		Op:       ssafir.OpX86ADC_M8_R8,
		Data: &x86InstructionData{
			Args:   [4]any{&x86.Memory{Segment: x86.ES, Base: x86.BP, Displacement: 7}, x86.CL},
			Length: 5,
		},
		Code: &x86.Code{
			Prefixes:        [14]x86.Prefix{x86.PrefixAddressSize, x86.PrefixES},
			Opcode:          [3]byte{0x10},
			OpcodeLen:       1,
			UseModRM:        true,
			ModRM:           x86.ModRMmodSmallDisplacedRegister | x86.ModRMreg001 | x86.ModRMrm110,
			Displacement:    [8]byte{0x07},
			DisplacementLen: 1,
		},
	},
	{
		Name:     "large add",
		Mode:     x86.Mode64,
		Assembly: "(add r8 (rdi))",
		Op:       ssafir.OpX86ADD_R64_M64_REX,
		Data: &x86InstructionData{
			Args:   [4]any{x86.R8, &x86.Memory{Base: x86.RDI}},
			Length: 3,
		},
		Code: &x86.Code{
			REX:       x86REX("WR"),
			Opcode:    [3]byte{0x03},
			OpcodeLen: 1,
			UseModRM:  true,
			ModRM:     x86.ModRMmodDereferenceRegister | x86.ModRMreg000 | x86.ModRMrm111,
		},
	},
	{
		Name:     "large displaced add",
		Mode:     x86.Mode64,
		Assembly: "(add r8 (+ rdi 7))",
		Op:       ssafir.OpX86ADD_R64_M64_REX,
		Data: &x86InstructionData{
			Args:   [4]any{x86.R8, &x86.Memory{Base: x86.RDI, Displacement: 7}},
			Length: 4,
		},
		Code: &x86.Code{
			REX:             x86REX("WR"),
			Opcode:          [3]byte{0x03},
			OpcodeLen:       1,
			UseModRM:        true,
			ModRM:           x86.ModRMmodSmallDisplacedRegister | x86.ModRMreg000 | x86.ModRMrm111,
			Displacement:    [8]byte{7},
			DisplacementLen: 1,
		},
	},
	{
		Name:     "move to register from ES",
		Mode:     x86.Mode32,
		Assembly: "(mov ah (es eax))",
		Op:       ssafir.OpX86MOV_R8_M8,
		Data: &x86InstructionData{
			Args:   [4]any{x86.AH, &x86.Memory{Segment: x86.ES, Base: x86.EAX}},
			Length: 3,
		},
		Code: &x86.Code{
			Prefixes:  [14]x86.Prefix{x86.PrefixES},
			Opcode:    [3]byte{0x8a},
			OpcodeLen: 1,
			UseModRM:  true,
			ModRM:     x86.ModRMmodDereferenceRegister | x86.ModRMreg100 | x86.ModRMrm000,
		},
	},
	{
		Name:     "move to register from CS",
		Mode:     x86.Mode32,
		Assembly: "(mov ah (cs eax))",
		Op:       ssafir.OpX86MOV_R8_M8,
		Data: &x86InstructionData{
			Args:   [4]any{x86.AH, &x86.Memory{Segment: x86.CS, Base: x86.EAX}},
			Length: 3,
		},
		Code: &x86.Code{
			Prefixes:  [14]x86.Prefix{x86.PrefixCS},
			Opcode:    [3]byte{0x8a},
			OpcodeLen: 1,
			UseModRM:  true,
			ModRM:     x86.ModRMmodDereferenceRegister | x86.ModRMreg100 | x86.ModRMrm000,
		},
	},
	{
		Name:     "move to register from SS",
		Mode:     x86.Mode32,
		Assembly: "(mov ah (ss eax))",
		Op:       ssafir.OpX86MOV_R8_M8,
		Data: &x86InstructionData{
			Args:   [4]any{x86.AH, &x86.Memory{Segment: x86.SS, Base: x86.EAX}},
			Length: 3,
		},
		Code: &x86.Code{
			Prefixes:  [14]x86.Prefix{x86.PrefixSS},
			Opcode:    [3]byte{0x8a},
			OpcodeLen: 1,
			UseModRM:  true,
			ModRM:     x86.ModRMmodDereferenceRegister | x86.ModRMreg100 | x86.ModRMrm000,
		},
	},
	{
		Name:     "move to register from DS",
		Mode:     x86.Mode32,
		Assembly: "(mov ah (ds eax))",
		Op:       ssafir.OpX86MOV_R8_M8,
		Data: &x86InstructionData{
			Args:   [4]any{x86.AH, &x86.Memory{Segment: x86.DS, Base: x86.EAX}},
			Length: 3,
		},
		Code: &x86.Code{
			Prefixes:  [14]x86.Prefix{x86.PrefixDS},
			Opcode:    [3]byte{0x8a},
			OpcodeLen: 1,
			UseModRM:  true,
			ModRM:     x86.ModRMmodDereferenceRegister | x86.ModRMreg100 | x86.ModRMrm000,
		},
	},
	{
		Name:     "move to register from FS",
		Mode:     x86.Mode32,
		Assembly: "(mov ah (fs eax))",
		Op:       ssafir.OpX86MOV_R8_M8,
		Data: &x86InstructionData{
			Args:   [4]any{x86.AH, &x86.Memory{Segment: x86.FS, Base: x86.EAX}},
			Length: 3,
		},
		Code: &x86.Code{
			Prefixes:  [14]x86.Prefix{x86.PrefixFS},
			Opcode:    [3]byte{0x8a},
			OpcodeLen: 1,
			UseModRM:  true,
			ModRM:     x86.ModRMmodDereferenceRegister | x86.ModRMreg100 | x86.ModRMrm000,
		},
	},
	{
		Name:     "move to register from GS",
		Mode:     x86.Mode32,
		Assembly: "(mov ah (gs eax))",
		Op:       ssafir.OpX86MOV_R8_M8,
		Data: &x86InstructionData{
			Args:   [4]any{x86.AH, &x86.Memory{Segment: x86.GS, Base: x86.EAX}},
			Length: 3,
		},
		Code: &x86.Code{
			Prefixes:  [14]x86.Prefix{x86.PrefixGS},
			Opcode:    [3]byte{0x8a},
			OpcodeLen: 1,
			UseModRM:  true,
			ModRM:     x86.ModRMmodDereferenceRegister | x86.ModRMreg100 | x86.ModRMrm000,
		},
	},
	{
		Name:     "size override mov",
		Mode:     x86.Mode64,
		Assembly: "(mov eax (edx))",
		Op:       ssafir.OpX86MOV_R32_M32,
		Data: &x86InstructionData{
			Args:   [4]any{x86.EAX, &x86.Memory{Base: x86.EDX}},
			Length: 3,
		},
		Code: &x86.Code{
			Prefixes:  [14]x86.Prefix{x86.PrefixAddressSize},
			Opcode:    [3]byte{0x8b},
			OpcodeLen: 1,
			UseModRM:  true,
			ModRM:     x86.ModRMmodDereferenceRegister | x86.ModRMreg000 | x86.ModRMrm010,
		},
	},
	{
		Name:     "memory base index displacement",
		Mode:     x86.Mode64,
		Assembly: "(mov rcx (+ rdx r9 17))",
		Op:       ssafir.OpX86MOV_R64_M64_REX,
		Data: &x86InstructionData{
			Args: [4]any{
				x86.RCX,
				&x86.Memory{
					Base:         x86.RDX,
					Index:        x86.R9,
					Displacement: 17,
				},
			},
			Length: 5,
		},
		Code: &x86.Code{
			REX:             x86REX("WX"),
			Opcode:          [3]byte{0x8b},
			OpcodeLen:       1,
			UseModRM:        true,
			ModRM:           x86.ModRMmodSmallDisplacedRegister | x86.ModRMreg001 | x86.ModRMrmSIB,
			SIB:             x86.SIBscale00 | x86.SIBindex001 | x86.SIBbase010,
			Displacement:    [8]byte{0x11},
			DisplacementLen: 1,
		},
	},
	{
		Name:     "memory base displacement 16-bit",
		Mode:     x86.Mode16,
		Assembly: "(mov cx (+ bx di 17))",
		Op:       ssafir.OpX86MOV_R16_M16,
		Data: &x86InstructionData{
			Args: [4]any{
				x86.CX,
				&x86.Memory{
					Base:         x86.BX_DI,
					Displacement: 17,
				},
			},
			Length: 3,
		},
		Code: &x86.Code{
			Opcode:          [3]byte{0x8b},
			OpcodeLen:       1,
			UseModRM:        true,
			ModRM:           x86.ModRMmodSmallDisplacedRegister | x86.ModRMreg001 | x86.ModRMrm001,
			Displacement:    [8]byte{0x11},
			DisplacementLen: 1,
		},
	},
	{
		Name:     "memory base index scale displacement",
		Mode:     x86.Mode64,
		Assembly: "(mov rcx (+ r12 (* rbx 4) 17))",
		Op:       ssafir.OpX86MOV_R64_M64_REX,
		Data: &x86InstructionData{
			Args: [4]any{
				x86.RCX,
				&x86.Memory{
					Base:         x86.R12,
					Index:        x86.RBX,
					Scale:        4,
					Displacement: 17,
				},
			},
			Length: 5,
		},
		Code: &x86.Code{
			REX:             x86REX("WB"),
			Opcode:          [3]byte{0x8b},
			OpcodeLen:       1,
			UseModRM:        true,
			ModRM:           x86.ModRMmodSmallDisplacedRegister | x86.ModRMreg001 | x86.ModRMrmSIB,
			SIB:             x86.SIBscale4 | x86.SIBindex011 | x86.SIBbase100,
			Displacement:    [8]byte{0x11},
			DisplacementLen: 1,
		},
	},
	{
		Name:     "memory base displacement",
		Mode:     x86.Mode32,
		Assembly: "(mov ecx (+ edx 256))",
		Op:       ssafir.OpX86MOV_R32_M32,
		Data: &x86InstructionData{
			Args: [4]any{
				x86.ECX,
				&x86.Memory{
					Base:         x86.EDX,
					Displacement: 256,
				},
			},
			Length: 6,
		},
		Code: &x86.Code{
			Opcode:          [3]byte{0x8b},
			OpcodeLen:       1,
			UseModRM:        true,
			ModRM:           x86.ModRMmodLargeDisplacedRegister | x86.ModRMreg001 | x86.ModRMrm010,
			Displacement:    [8]byte{0x00, 0x01, 0x00, 0x00},
			DisplacementLen: 4,
		},
	},
	{
		Name:     "memory base index",
		Mode:     x86.Mode32,
		Assembly: "(mov ecx (+ edx ebx))",
		Op:       ssafir.OpX86MOV_R32_M32,
		Data: &x86InstructionData{
			Args: [4]any{
				x86.ECX,
				&x86.Memory{
					Base:  x86.EDX,
					Index: x86.EBX,
				},
			},
			Length: 3,
		},
		Code: &x86.Code{
			Opcode:    [3]byte{0x8b},
			OpcodeLen: 1,
			UseModRM:  true,
			ModRM:     x86.ModRMmodDereferenceRegister | x86.ModRMreg001 | x86.ModRMrmSIB,
			SIB:       x86.SIBscale1 | x86.SIBindex011 | x86.SIBbase010,
		},
	},
	{
		Name:     "memory index scale",
		Mode:     x86.Mode64,
		Assembly: "(mov rcx (* rbx 8))",
		Op:       ssafir.OpX86MOV_R64_M64_REX,
		Data: &x86InstructionData{
			Args: [4]any{
				x86.RCX,
				&x86.Memory{
					Index: x86.RBX,
					Scale: 8,
				},
			},
			Length: 8,
		},
		Code: &x86.Code{
			REX:             x86REX("W"),
			Opcode:          [3]byte{0x8b},
			OpcodeLen:       1,
			UseModRM:        true,
			ModRM:           x86.ModRMmodDereferenceRegister | x86.ModRMreg001 | x86.ModRMrmSIB,
			SIB:             x86.SIBscale8 | x86.SIBindex011 | x86.SIBbaseNone,
			DisplacementLen: 4, // We have to include a (zero) displacement with no base register.
		},
	},
	{
		Name:     "memory index scale displacement",
		Mode:     x86.Mode64,
		Assembly: "(mov rcx (+ (* rbx 8) 17))",
		Op:       ssafir.OpX86MOV_R64_M64_REX,
		Data: &x86InstructionData{
			Args: [4]any{
				x86.RCX,
				&x86.Memory{
					Index:        x86.RBX,
					Scale:        8,
					Displacement: 17,
				},
			},
			Length: 8,
		},
		Code: &x86.Code{
			REX:             x86REX("W"),
			Opcode:          [3]byte{0x8b},
			OpcodeLen:       1,
			UseModRM:        true,
			ModRM:           x86.ModRMmodDereferenceRegister | x86.ModRMreg001 | x86.ModRMrmSIB,
			SIB:             x86.SIBscale8 | x86.SIBindex011 | x86.SIBbaseNone,
			Displacement:    [8]byte{0x11},
			DisplacementLen: 4,
		},
	},
	{
		Name:     "memory base index scale",
		Mode:     x86.Mode32,
		Assembly: "(mov ecx (+ edx (* ebx 2)))",
		Op:       ssafir.OpX86MOV_R32_M32,
		Data: &x86InstructionData{
			Args: [4]any{
				x86.ECX,
				&x86.Memory{
					Base:  x86.EDX,
					Index: x86.EBX,
					Scale: 2,
				},
			},
			Length: 3,
		},
		Code: &x86.Code{
			Opcode:    [3]byte{0x8b},
			OpcodeLen: 1,
			UseModRM:  true,
			ModRM:     x86.ModRMmodDereferenceRegister | x86.ModRMreg001 | x86.ModRMrmSIB,
			SIB:       x86.SIBscale2 | x86.SIBindex011 | x86.SIBbase010,
		},
	},
	{
		Name:     "memory base",
		Mode:     x86.Mode32,
		Assembly: "(mov ecx (edx))",
		Op:       ssafir.OpX86MOV_R32_M32,
		Data: &x86InstructionData{
			Args: [4]any{
				x86.ECX,
				&x86.Memory{
					Base: x86.EDX,
				},
			},
			Length: 2,
		},
		Code: &x86.Code{
			Opcode:    [3]byte{0x8b},
			OpcodeLen: 1,
			UseModRM:  true,
			ModRM:     x86.ModRMmodDereferenceRegister | x86.ModRMreg001 | x86.ModRMrm010,
		},
	},
	{
		Name:     "memory displacement",
		Mode:     x86.Mode32,
		Assembly: "(mov ecx (17))",
		Op:       ssafir.OpX86MOV_R32_M32,
		Data: &x86InstructionData{
			Args: [4]any{
				x86.ECX,
				&x86.Memory{
					Displacement: 17,
				},
			},
			Length: 6,
		},
		Code: &x86.Code{
			Opcode:          [3]byte{0x8b},
			OpcodeLen:       1,
			UseModRM:        true,
			ModRM:           x86.ModRMmodDereferenceRegister | x86.ModRMreg001 | x86.ModRMrmDisplacementOnly32,
			Displacement:    [8]byte{0x11},
			DisplacementLen: 4,
		},
	},
	{
		Name:     "memory displacement 16-bit",
		Mode:     x86.Mode16,
		Assembly: "(mov cx (17))",
		Op:       ssafir.OpX86MOV_R16_M16,
		Data: &x86InstructionData{
			Args: [4]any{
				x86.CX,
				&x86.Memory{
					Displacement: 17,
				},
			},
			Length: 4,
		},
		Code: &x86.Code{
			Opcode:          [3]byte{0x8b},
			OpcodeLen:       1,
			UseModRM:        true,
			ModRM:           x86.ModRMmodDereferenceRegister | x86.ModRMreg001 | x86.ModRMrmDisplacementOnly16,
			Displacement:    [8]byte{0x11},
			DisplacementLen: 2,
		},
	},
	{
		Name:     "memory segment offset",
		Mode:     x86.Mode32,
		Assembly: "(mov al (ss 17))",
		Op:       ssafir.OpX86MOV_AL_Moffs8,
		Data: &x86InstructionData{
			Args: [4]any{
				x86.AL,
				&x86.Memory{
					Segment:      x86.SS,
					Displacement: 17,
				},
			},
			Length: 6,
		},
		Code: &x86.Code{
			Prefixes:        [14]x86.Prefix{x86.PrefixSS},
			Opcode:          [3]byte{0xa0},
			OpcodeLen:       1,
			Displacement:    [8]byte{0x11},
			DisplacementLen: 4,
		},
	},
	{
		Name:     "memory absolute offset",
		Mode:     x86.Mode64,
		Assembly: "(mov rax (0x1122334455667788))",
		Op:       ssafir.OpX86MOV_RAX_Moffs64_REX,
		Data: &x86InstructionData{
			Args: [4]any{
				x86.RAX,
				&x86.Memory{
					Displacement: 0x1122334455667788,
				},
			},
			Length: 10,
		},
		Code: &x86.Code{
			REX:             x86REX("W"),
			Opcode:          [3]byte{0xa1},
			OpcodeLen:       1,
			Displacement:    [8]byte{0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11},
			DisplacementLen: 8,
		},
	},
	{
		Name:     "memory strings",
		Mode:     x86.Mode32,
		Assembly: "(movs '(bits 8)(edi) (esi))",
		Op:       ssafir.OpX86MOVS_StrDst8_StrSrc8,
		Data: &x86InstructionData{
			Args: [4]any{
				x86.EDI,
				x86.ESI,
			},
			Length: 1,
		},
		Code: &x86.Code{
			Opcode:    [3]byte{0xa4},
			OpcodeLen: 1,
		},
	},
	{
		Name:     "memory explicit strings",
		Mode:     x86.Mode32,
		Assembly: "(movs '(bits 32)(es edi) '(bytes 4)(ds esi))",
		Op:       ssafir.OpX86MOVS_StrDst32_StrSrc32,
		Data: &x86InstructionData{
			Args: [4]any{
				x86.EDI,
				x86.ESI,
			},
			Length: 1,
		},
		Code: &x86.Code{
			Opcode:    [3]byte{0xa5},
			OpcodeLen: 1,
		},
	},
	{
		Name:     "memory strings 32-bit",
		Mode:     x86.Mode32,
		Assembly: "(movsd (edi) (esi))",
		Op:       ssafir.OpX86MOVSD,
		Data: &x86InstructionData{
			Args: [4]any{
				x86.EDI,
				x86.ESI,
			},
			Length: 1,
		},
		Code: &x86.Code{
			Opcode:    [3]byte{0xa5},
			OpcodeLen: 1,
		},
	},
	{
		Name:     "memory strings 16-bit",
		Mode:     x86.Mode16,
		Assembly: "(movsd (edi) (esi))",
		Op:       ssafir.OpX86MOVSD,
		Data: &x86InstructionData{
			Args: [4]any{
				x86.EDI,
				x86.ESI,
			},
			Length: 3,
		},
		Code: &x86.Code{
			Prefixes:  [14]x86.Prefix{x86.PrefixAddressSize, x86.PrefixOperandSize},
			Opcode:    [3]byte{0xa5},
			OpcodeLen: 1,
		},
	},
	{
		Name:     "call absolute address",
		Mode:     x86.Mode32,
		Assembly: "(call-far (0x1122 0x33445566))",
		Op:       ssafir.OpX86CALL_FAR_Ptr16v32,
		Data: &x86InstructionData{
			Args:   [4]any{uint64(0x112233445566)},
			Length: 7,
		},
		Code: &x86.Code{
			Opcode:        [3]byte{0x9a},
			OpcodeLen:     1,
			CodeOffset:    [8]byte{0x66, 0x55, 0x44, 0x33, 0x22, 0x11},
			CodeOffsetLen: 6,
		},
	},
	{
		Name:     "specialised cmppd",
		Mode:     x86.Mode16,
		Assembly: "(cmpeqpd xmm0 (0xb))",
		Op:       ssafir.OpX86CMPEQPD_XMM1_M128,
		Data: &x86InstructionData{
			Args:   [4]any{x86.XMM0, &x86.Memory{Displacement: 0xb}},
			Length: 7,
		},
		Code: &x86.Code{
			Prefixes:        [14]x86.Prefix{x86.PrefixOperandSize},
			Opcode:          [3]byte{0x0f, 0xc2},
			OpcodeLen:       2,
			UseModRM:        true,
			ModRM:           x86.ModRMmodDereferenceRegister | x86.ModRMreg000 | x86.ModRMrm110,
			Displacement:    [8]byte{0x0b, 0x00},
			DisplacementLen: 2,
			Immediate:       [8]byte{0x00},
			ImmediateLen:    1,
		},
	},
	{
		Name:     "x87 add",
		Mode:     x86.Mode64,
		Assembly: "(fadd st0 st)",
		Op:       ssafir.OpX86FADD_STi_ST, // The order matters.
		Data: &x86InstructionData{
			Args:   [4]any{x86.ST0, struct{}{}},
			Length: 2,
		},
		Code: &x86.Code{
			Opcode:    [3]byte{0xdc, 0xc0},
			OpcodeLen: 2,
		},
	},
	{
		Name:     "old fsave",
		Mode:     x86.Mode32,
		Assembly: "(fsave (ecx))",
		Op:       ssafir.OpX86FSAVE_M94l108byte,
		Data: &x86InstructionData{
			Args:   [4]any{&x86.Memory{Base: x86.ECX}},
			Length: 3,
		},
		Code: &x86.Code{
			PrefixOpcodes: [5]byte{0x9b},
			Opcode:        [3]byte{0xdd},
			OpcodeLen:     1,
			UseModRM:      true,
			ModRM:         x86.ModRMmodDereferenceRegister | x86.ModRMreg110 | x86.ModRMrm001,
		},
	},
	{
		Name:     "sysret to 32-bit mode",
		Mode:     x86.Mode64,
		Assembly: "(sysret)",
		Op:       ssafir.OpX86SYSRET,
		Data: &x86InstructionData{
			Length: 2,
		},
		Code: &x86.Code{
			Opcode:    [3]byte{0x0f, 0x07},
			OpcodeLen: 2,
		},
	},
	{
		Name:     "sysret to 64-bit mode",
		Mode:     x86.Mode64,
		Assembly: "(rex.w sysret)",
		Op:       ssafir.OpX86SYSRET,
		Data: &x86InstructionData{
			Length: 3,
			REX_W:  true,
		},
		Code: &x86.Code{
			REX:       x86REX("W"),
			Opcode:    [3]byte{0x0f, 0x07},
			OpcodeLen: 2,
		},
	},
	{
		Name:     "stosb",
		Mode:     x86.Mode64,
		Assembly: "(stosb)",
		Op:       ssafir.OpX86STOSB,
		Data: &x86InstructionData{
			Length: 1,
		},
		Code: &x86.Code{
			Opcode:    [3]byte{0xaa},
			OpcodeLen: 1,
		},
	},
	{
		Name:     "rep stosb",
		Mode:     x86.Mode64,
		Assembly: "(rep stosb)",
		Op:       ssafir.OpX86STOSB,
		Data: &x86InstructionData{
			Prefixes:  [5]x86.Prefix{x86.PrefixRepeat},
			PrefixLen: 1,
			Length:    2,
		},
		Code: &x86.Code{
			Prefixes:  [14]x86.Prefix{x86.PrefixRepeat},
			Opcode:    [3]byte{0xaa},
			OpcodeLen: 1,
		},
	},
	{
		Name:     "VEX extended register",
		Mode:     x86.Mode64,
		Assembly: "(vaddpd ymm3 ymm2 ymm8)",
		Op:       ssafir.OpX86VADDPD_YMM1_YMMV_YMM2_VEX,
		Data: &x86InstructionData{
			Args:   [4]any{x86.YMM3, x86.YMM2, x86.YMM8},
			Length: 5,
		},
		Code: &x86.Code{
			VEX: x86.VEX{
				0b1100_0001, // 0xc1: R:1, X:1, B:0, m-mmmm:00001.
				0b0110_1101, // 0x6d: W:0, vvvv:1101, L:1, pp:01.
			},
			Opcode:    [3]byte{0x58},
			OpcodeLen: 1,
			ModRM:     x86.ModRMmodRegister | x86.ModRMreg011 | x86.ModRMrm000,
			UseModRM:  true,
		},
	},
	{
		Name:     "VEX is4",
		Mode:     x86.Mode64,
		Assembly: "(vblendvps xmm12 xmm13 xmm14 xmm15)",
		Op:       ssafir.OpX86VBLENDVPS_XMM1_XMMV_XMM2_XMMIH_VEX,
		Data: &x86InstructionData{
			Args: [4]any{
				x86.XMM12,
				x86.XMM13,
				x86.XMM14,
				x86.XMM15,
			},
			Length: 6,
		},
		Code: &x86.Code{
			VEX: x86.VEX{
				0b0100_0011, // 0x43: R:0, X:1, B:0, m-mmmm:00011.
				0b0001_0001, // 0x11: W:0, vvvv:0010, L:0, pp:01.
			},
			Opcode:       [3]byte{0x4a},
			OpcodeLen:    1,
			ModRM:        x86.ModRMmodRegister | x86.ModRMreg100 | x86.ModRMrm110,
			UseModRM:     true,
			Immediate:    [8]byte{0b1111_0000},
			ImmediateLen: 1,
		},
	},
	{
		Name:     "EVEX extended register",
		Mode:     x86.Mode64,
		Assembly: "(vaddpd ymm14 ymm3 ymm31)",
		Op:       ssafir.OpX86VADDPD_YMM1_YMMV_YMM2_EVEX256,
		Data: &x86InstructionData{
			Args:   [4]any{x86.YMM14, x86.YMM3, x86.YMM31},
			Length: 6,
		},
		Code: &x86.Code{
			EVEX: x86.EVEX{
				0b0001_0001, // 0x11: R:0, X:0, B:0, R':1, mm:01.
				0b1110_0101, // 0xe5: W:0, vvvv:1100, pp:01.
				0b0010_1000, // 0x28: z:0, L':0, L:1, b:0, V':1, aaa:000.
			},
			Opcode:    [3]byte{0x58},
			OpcodeLen: 1,
			ModRM:     x86.ModRMmodRegister | x86.ModRMreg110 | x86.ModRMrm111,
			UseModRM:  true,
		},
	},
	{
		Name:     "EVEX uncompressed displacement",
		Mode:     x86.Mode64,
		Assembly: "(vaddpd ymm19 ymm3 (+ rax 513))",
		Op:       ssafir.OpX86VADDPD_YMM1_YMMV_M256_EVEX256,
		Data: &x86InstructionData{
			Args:   [4]any{x86.YMM19, x86.YMM3, &x86.Memory{Base: x86.RAX, Displacement: 513}},
			Length: 10,
		},
		Code: &x86.Code{
			EVEX: x86.EVEX{
				0b1110_0001, // 0xe1: R:1, X:1, B:1, R':0, mm:01.
				0b1110_0101, // 0xe5: W:0, vvvv:1100, pp:01.
				0b0010_1000, // 0x28: z:0, L':0, L:1, b:0, V':1, aaa:000.
			},
			Opcode:          [3]byte{0x58},
			OpcodeLen:       1,
			ModRM:           x86.ModRMmodLargeDisplacedRegister | x86.ModRMreg011 | x86.ModRMrm000,
			UseModRM:        true,
			Displacement:    [8]byte{0x01, 0x02, 0x00, 0x00},
			DisplacementLen: 4,
		},
	},
	{
		Name:     "EVEX compressed displacement",
		Mode:     x86.Mode64,
		Assembly: "(vaddpd ymm19 ymm3 (+ rax 512))",
		Op:       ssafir.OpX86VADDPD_YMM1_YMMV_M256_EVEX256,
		Data: &x86InstructionData{
			Args:   [4]any{x86.YMM19, x86.YMM3, &x86.Memory{Base: x86.RAX, Displacement: 512}},
			Length: 7,
		},
		Code: &x86.Code{
			EVEX: x86.EVEX{
				0b1110_0001, // 0xe1: R:1, X:1, B:1, R':0, mm:01.
				0b1110_0101, // 0xe5: W:0, vvvv:1100, pp:01.
				0b0010_1000, // 0x28: z:0, L':0, L:1, b:0, V':1, aaa:000.
			},
			Opcode:          [3]byte{0x58},
			OpcodeLen:       1,
			ModRM:           x86.ModRMmodSmallDisplacedRegister | x86.ModRMreg011 | x86.ModRMrm000,
			UseModRM:        true,
			Displacement:    [8]byte{0x10},
			DisplacementLen: 1,
		},
	},
	{
		Name:     "EVEX implicit opmask",
		Mode:     x86.Mode64,
		Assembly: "(vaddpd ymm14 ymm3 ymm31)",
		Op:       ssafir.OpX86VADDPD_YMM1_YMMV_YMM2_EVEX256,
		Data: &x86InstructionData{
			Args:   [4]any{x86.YMM14, x86.YMM3, x86.YMM31},
			Length: 6,
			Mask:   0,
		},
		Code: &x86.Code{
			EVEX: x86.EVEX{
				0b0001_0001, // 0x11: R:0, X:0, B:0, R':1, mm:01.
				0b1110_0101, // 0xe5: W:0, vvvv:1100, pp:01.
				0b0010_1000, // 0x28: z:0, L':0, L:1, b:0, V':1, aaa:000.
			},
			Opcode:    [3]byte{0x58},
			OpcodeLen: 1,
			ModRM:     x86.ModRMmodRegister | x86.ModRMreg110 | x86.ModRMrm111,
			UseModRM:  true,
		},
	},
	{
		Name:     "EVEX explicit opmask",
		Mode:     x86.Mode64,
		Assembly: "'(mask k7)(vaddpd ymm14 ymm3 ymm31)",
		Op:       ssafir.OpX86VADDPD_YMM1_YMMV_YMM2_EVEX256,
		Data: &x86InstructionData{
			Args:   [4]any{x86.YMM14, x86.YMM3, x86.YMM31},
			Length: 6,
			Mask:   7,
		},
		Code: &x86.Code{
			EVEX: x86.EVEX{
				0b0001_0001, // 0x11: R:0, X:0, B:0, R':1, mm:01.
				0b1110_0101, // 0xe5: W:0, vvvv:1100, pp:01.
				0b0010_1111, // 0x2f: z:0, L':0, L:1, b:0, V':1, aaa:111.
			},
			Opcode:    [3]byte{0x58},
			OpcodeLen: 1,
			ModRM:     x86.ModRMmodRegister | x86.ModRMreg110 | x86.ModRMrm111,
			UseModRM:  true,
		},
	},
	{
		Name:     "EVEX implicit zeroing",
		Mode:     x86.Mode64,
		Assembly: "'(zero false)(vaddpd ymm14 ymm3 ymm31)",
		Op:       ssafir.OpX86VADDPD_YMM1_YMMV_YMM2_EVEX256,
		Data: &x86InstructionData{
			Args:   [4]any{x86.YMM14, x86.YMM3, x86.YMM31},
			Length: 6,
			Zero:   false,
		},
		Code: &x86.Code{
			EVEX: x86.EVEX{
				0b0001_0001, // 0x11: R:0, X:0, B:0, R':1, mm:01.
				0b1110_0101, // 0xe5: W:0, vvvv:1100, pp:01.
				0b0010_1000, // 0x28: z:0, L':0, L:1, b:0, V':1, aaa:000.
			},
			Opcode:    [3]byte{0x58},
			OpcodeLen: 1,
			ModRM:     x86.ModRMmodRegister | x86.ModRMreg110 | x86.ModRMrm111,
			UseModRM:  true,
		},
	},
	{
		Name:     "EVEX explicit zeroing",
		Mode:     x86.Mode64,
		Assembly: "'(zero true)(vaddpd ymm14 ymm3 ymm31)",
		Op:       ssafir.OpX86VADDPD_YMM1_YMMV_YMM2_EVEX256,
		Data: &x86InstructionData{
			Args:   [4]any{x86.YMM14, x86.YMM3, x86.YMM31},
			Length: 6,
			Zero:   true,
		},
		Code: &x86.Code{
			EVEX: x86.EVEX{
				0b0001_0001, // 0x11: R:0, X:0, B:0, R':1, mm:01.
				0b1110_0101, // 0xe5: W:0, vvvv:1100, pp:01.
				0b1010_1000, // 0xa8: z:1, L':0, L:1, b:0, V':1, aaa:000.
			},
			Opcode:    [3]byte{0x58},
			OpcodeLen: 1,
			ModRM:     x86.ModRMmodRegister | x86.ModRMreg110 | x86.ModRMrm111,
			UseModRM:  true,
		},
	},
	{
		Name:     "force selection of a longer encoding",
		Mode:     x86.Mode64,
		Assembly: "'(match ADD_Rmr8_Imm8)(add al 1)",
		Op:       ssafir.OpX86ADD_Rmr8_Imm8,
		Data: &x86InstructionData{
			Args:   [4]any{x86.AL, uint64(1)},
			Length: 3,
		},
		Code: &x86.Code{
			Opcode:       [3]byte{0x80},
			OpcodeLen:    1,
			UseModRM:     true,
			ModRM:        x86.ModRMmodRegister | x86.ModRMreg000 | x86.ModRMrm000,
			Immediate:    [8]byte{0x01},
			ImmediateLen: 1,
		},
	},
	{
		Name:          "illegal prefix",
		Mode:          x86.Mode32,
		Assembly:      "(rep rdrand eax)",
		AssemblyError: "mnemonic \"rdrand\" cannot be used with repeat prefixes",
	},
	{
		Name:          "illegal register",
		Mode:          x86.Mode32,
		Assembly:      "(vaddpd ymm3 ymm2 ymm8)",
		AssemblyError: "register ymm8 cannot be used in 32-bit mode",
	},
	{
		Name:          "wrong arity single",
		Mode:          x86.Mode16,
		Assembly:      "(add cx ax bx sp)",
		AssemblyError: "expected 2 arguments, found 4",
	},
	{
		Name:          "wrong arity pair",
		Mode:          x86.Mode32,
		Assembly:      "(ret 1 2 3)",
		AssemblyError: "expected 0 or 1 arguments, found 3",
	},
	{
		Name:          "wrong arity many",
		Mode:          x86.Mode64,
		Assembly:      "(movsd xmm1 xmm2 xmm3 xmm4 xmm5)",
		AssemblyError: "expected 0, 1, or 2 arguments, found 5",
	},
	{
		Name:          "unrecognised instruction",
		Mode:          x86.Mode64,
		Assembly:      "(not-a-real-instruction)",
		AssemblyError: "mnemonic \"not-a-real-instruction\" not recognised",
	},
	{
		Name:          "mismatched instruction",
		Mode:          x86.Mode64,
		Assembly:      "(add 1 2)",
		AssemblyError: "no matching instruction found for (add 1 2)",
	},
	{
		Name:          "mismatched specific instruction",
		Mode:          x86.Mode64,
		Assembly:      "'(match ADD_R32_Rmr32)(add rax rcx)",
		AssemblyError: "(add rax rcx) does not match instruction ADD_R32_Rmr32",
	},
	{
		Name:          "unmatched label",
		Mode:          x86.Mode64,
		Assembly:      "'foo",
		AssemblyError: `label "foo" is not referenced by any instructions`,
	},
}

func TestX86OpToInstruction(t *testing.T) {
	// Test that ssafir.Op opcodes for x86
	// and x86 instruction data match.
	for i, inst := range x86.Instructions {
		op := firstX86Op + ssafir.Op(i)
		if op.String() != inst.UID {
			t.Errorf("opcode mismatch: opcode %d (%s) does not match instruction %s", i, op, inst.UID)
		}
	}
}

func TestAssembleX86(t *testing.T) {
	// Use x86-64.
	arch := sys.X86_64
	sizes := types.SizesFor(arch)

	for _, test := range x86TestCases {
		t.Run(test.Name, func(t *testing.T) {
			fset := token.NewFileSet()
			mode := test.Mode.Int
			if mode == 0 {
				mode = 64
			}

			text := fmt.Sprintf("(package test)\n\n'(mode %d)\n(asm-func (test) %s)", mode, test.Assembly)
			file, err := parser.ParseFile(fset, "test.ruse", text, 0)
			if err != nil {
				t.Fatalf("failed to parse text: %v", err)
			}

			files := []*ast.File{file}
			info := &types.Info{
				Types:       make(map[ast.Expression]types.TypeAndValue),
				Definitions: make(map[*ast.Identifier]types.Object),
				Uses:        make(map[*ast.Identifier]types.Object),
			}

			pkg, err := types.Check("test", fset, files, arch, info)
			if err != nil {
				t.Fatalf("failed to type-check package: %v", err)
			}

			p, err := Compile(fset, arch, pkg, files, info, sizes)
			if test.AssemblyError != "" {
				if err == nil {
					t.Fatalf("unexpected success, wanted error %q", test.AssemblyError)
				}

				e := err.Error()
				if !strings.Contains(e, test.AssemblyError) {
					t.Fatalf("got error %q, want %q", e, test.AssemblyError)
				}

				return
			}

			if err != nil {
				inst := x86OpToInstruction(test.Op)
				t.Fatalf("unexpected error: %v (%d-%d)", err, inst.MinArgs, inst.MaxArgs)
			}

			// The package should have one function with
			// two values; a memory state and an instruction,
			// which we compare with test.Want.
			if len(p.Functions) != 1 {
				t.Fatalf("got %d functions, want 1: %#v", len(p.Functions), p.Functions)
			}

			fun := p.Functions[0]
			if len(fun.Entry.Values) != 1 {
				t.Fatalf("got %d values, want 1: %#v", len(fun.Entry.Values), fun.Entry.Values)
			}

			v := fun.Entry.Values[0]
			if v.Op != test.Op {
				t.Fatalf("Compile:\n  Got op  %s\n  Want op %s", v.Op, test.Op)
			}

			if diff := cmp.Diff(test.Data, v.Extra); diff != "" {
				t.Fatalf("Compile(): (-want, +got)\n%s", diff)
			}
		})
	}
}

func TestEncodeInstructionX86(t *testing.T) {
	var got x86.Code
	for _, test := range x86TestCases {
		t.Run(test.Name, func(t *testing.T) {
			if test.AssemblyError != "" {
				t.Skipf("skipping test case expecting assembly error")
			}

			err := x86EncodeInstruction(&got, test.Mode, test.Op, test.Data)
			if err != nil {
				t.Fatalf("%s.Encode(): %v", test.Assembly, err)
			}

			if diff := cmp.Diff(test.Code, &got); diff != "" {
				t.Fatalf("%s.Encode(): (-want, +got)\n%s", test.Assembly, diff)
			}
		})
	}
}

func BenchmarkX86(b *testing.B) {
	// Use x86-64.
	arch := sys.X86_64
	sizes := types.SizesFor(arch)

	var got x86.Code
	for _, test := range x86TestCases {
		b.Run(test.Name, func(b *testing.B) {
			if test.AssemblyError != "" {
				b.Skipf("skipping test case expecting assembly error")
			}

			fset := token.NewFileSet()
			mode := test.Mode.Int
			if mode == 0 {
				mode = 64
			}

			text := fmt.Sprintf("(package test)\n\n'(mode %d)\n(asm-func (test) %s)", mode, test.Assembly)
			file, err := parser.ParseFile(fset, "test.ruse", text, 0)
			if err != nil {
				b.Fatalf("failed to parse text: %v", err)
			}

			files := []*ast.File{file}
			info := &types.Info{
				Types:       make(map[ast.Expression]types.TypeAndValue),
				Definitions: make(map[*ast.Identifier]types.Object),
				Uses:        make(map[*ast.Identifier]types.Object),
			}

			pkg, err := types.Check("test", fset, files, arch, info)
			if err != nil {
				b.Fatalf("failed to type-check package: %v", err)
			}

			b.Run("assembly", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					_, err = Compile(fset, arch, pkg, files, info, sizes)
					if err != nil {
						b.Fatalf("unexpected error: %v", err)
					}
				}
			})

			b.Run("encoding", func(b *testing.B) {
				b.ReportAllocs()
				for i := 0; i < b.N; i++ {
					err = x86EncodeInstruction(&got, test.Mode, test.Op, test.Data)
					if err != nil {
						b.Fatalf("unexpected error: %v", err)
					}
				}
			})
		})
	}
}

func TestEncodeX86(t *testing.T) {
	tests := []struct {
		Name  string
		Ruse  string
		Want  []byte
		Links []*ssafir.Link
	}{
		{
			Name: "simple",
			Ruse: `
				'(arch x86-64)
				'(mode 64)
				(asm-func (test)
					(mov cl 1)
					(xchg rax rax)
					(syscall))
			`,
			Want: []byte{
				0xb1, 0x01, // MOV cl, 1
				0x48, 0x90, // XCHG rax, rax
				0x0f, 0x05, // SYSCALL
			},
		},
		{
			Name: "backwards jumps",
			Ruse: `
				'(arch x86-64)
				'(mode 64)
				(asm-func (test)
					'bar
					(mov cl 1)
					'foo
					(xchg rax rax)
					(je 'foo)
					(jmp 'bar))
			`,
			Want: []byte{
				0xb1, 0x01, // MOV cl, 1
				0x48, 0x90, // XCHG rax, rax
				0x74, 0xfc, // JE -4
				0xeb, 0xf8, // JMP -8
			},
		},
		{
			Name: "forwards jumps",
			Ruse: `
				'(arch x86-64)
				'(mode 64)
				(asm-func (test)
					(je 'foo)
					(jmp 'bar)
					(mov cl 1)
					'bar
					(xchg rax rax)
					'foo)
			`,
			Want: []byte{
				0x74, 0x06, // JE +6
				0xeb, 0x02, // JMP +2
				0xb1, 0x01, // MOV cl, 1
				0x48, 0x90, // XCHG rax, rax
			},
		},
		{
			Name: "string constant length",
			Ruse: `
				(let hello-world "Hello, world!")

				'(arch x86-64)
				(asm-func (test)
					(mov ecx (len hello-world)))
			`,
			Want: []byte{
				0xb9, 0x0d, 0x00, 0x00, 0x00, // MOV ecx, 13.
			},
		},
		{
			Name: "64 bit string constant link",
			Ruse: `
				(let hello-world "Hello, world!")

				'(arch x86-64)
				(asm-func (test)
					(nop)
					(mov rcx (string-pointer hello-world))
					(nop))
			`,
			Want: []byte{
				0x90,                                                       // NOP.
				0x48, 0xb9, 0x88, 0x77, 0x66, 0x55, 0x44, 0x33, 0x22, 0x11, // MOV rcx, 0x1122334455667788.
				0x90, // NOP.
			},
			Links: []*ssafir.Link{
				{
					Name:    "test.hello-world",
					Type:    ssafir.LinkFullAddress,
					Size:    64,
					Offset:  3,
					Address: 11,
				},
			},
		},
		{
			Name: "32 bit string constant link",
			Ruse: `
				(let hello-world "Hello, world!")

				'(arch x86-64)
				'(mode 32)
				(asm-func (test)
					(nop)
					(mov ecx (string-pointer hello-world))
					(nop))
			`,
			Want: []byte{
				0x90,                         // NOP.
				0xb9, 0x44, 0x33, 0x22, 0x11, // MOV rcx, 0x11223344.
				0x90, // NOP.
			},
			Links: []*ssafir.Link{
				{
					Name:    "test.hello-world",
					Type:    ssafir.LinkFullAddress,
					Size:    32,
					Offset:  2,
					Address: 6,
				},
			},
		},
		{
			Name: "64 bit relative function link",
			Ruse: `
				(let hello-world "Hello, world!") ; This should be a function, but we've set up the test to expect only one function.

				'(arch x86-64)
				(asm-func (test)
					(nop)
					(call (string-pointer hello-world))
					(nop))
			`,
			Want: []byte{
				0x90,                         // NOP.
				0xe8, 0x3f, 0x33, 0x22, 0x11, // CALL +0x11223344.
				0x90, // NOP.
			},
			Links: []*ssafir.Link{
				{
					Name:    "test.hello-world",
					Type:    ssafir.LinkRelativeAddress,
					Size:    32,
					Offset:  2,
					Address: 6,
				},
			},
		},
	}

	compareOptions := []cmp.Option{
		cmpopts.IgnoreTypes(token.Pos(0)), // Ignore token.Pos.
	}

	// Use x86-64.
	arch := sys.X86_64
	sizes := types.SizesFor(arch)

	var b bytes.Buffer
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "test.ruse", "(package test)\n\n"+test.Ruse, 0)
			if err != nil {
				t.Fatalf("failed to parse:\n  Ruse: %s\n    %v", test.Ruse, err)
			}

			files := []*ast.File{file}
			info := &types.Info{
				Types:       make(map[ast.Expression]types.TypeAndValue),
				Definitions: make(map[*ast.Identifier]types.Object),
				Uses:        make(map[*ast.Identifier]types.Object),
			}

			pkg, err := types.Check("test", fset, files, arch, info)
			if err != nil {
				t.Fatalf("failed to type-check:\n  Ruse: %s\n    %v", test.Ruse, err)
			}

			defer func() {
				v := recover()
				if v != nil {
					var b strings.Builder
					fmt.Fprintf(&b, "failed to compile:\n")
					fmt.Fprintf(&b, "  Ruse:  %s\n", test.Ruse)
					fmt.Fprintf(&b, "    panic: %v\n", v)
					fmt.Fprintf(&b, "    Want: % x", test.Want)
					t.Fatal(b.String())
				}
			}()

			p, err := Compile(fset, arch, pkg, files, info, sizes)
			if err != nil {
				var b strings.Builder
				fmt.Fprintf(&b, "failed to compile:\n")
				fmt.Fprintf(&b, "  Ruse:  %s\n", test.Ruse)
				fmt.Fprintf(&b, "    %v\n", err)
				fmt.Fprintf(&b, "    Want: % x", test.Want)
				t.Fatal(b.String())
			}

			// The package should have one function with
			// two values; a memory state and an instruction,
			// which we compare with test.Want.
			if len(p.Functions) != 1 {
				t.Fatalf("bad compile of %s: got %d functions, want 1: %#v", test.Ruse, len(p.Functions), p.Functions)
			}

			fun := p.Functions[0]

			b.Reset()
			err = EncodeTo(&b, fset, arch, fun)
			if err != nil {
				var b strings.Builder
				fmt.Fprintf(&b, "wrong encoding:\n")
				fmt.Fprintf(&b, "  Ruse:   %s\n", test.Ruse)
				fmt.Fprintf(&b, "    %v\n", err)
				fmt.Fprintf(&b, "    Want: % x", test.Want)
				t.Fatal(b.String())
			}

			got := b.Bytes()
			if !bytes.Equal(got, test.Want) {
				var b strings.Builder
				fmt.Fprintf(&b, "wrong encoding:\n")
				fmt.Fprintf(&b, "  Ruse:   %s\n", test.Ruse)
				fmt.Fprintf(&b, "    Got:  % x\n", got)
				fmt.Fprintf(&b, "    Want: % x", test.Want)
				t.Fatal(b.String())
			}

			if diff := cmp.Diff(test.Links, fun.Links, compareOptions...); diff != "" {
				t.Fatalf("Compile(): (-want, +got)\n%s", diff)
			}
		})
	}
}
