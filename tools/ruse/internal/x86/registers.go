// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package x86

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Register contains information about
// an x86 register, including its size
// in bits (for fixed-size register
// groups) and whether it belongs to
// any register groups.
type Register struct {
	Name    string       `json:"name"`
	Type    RegisterType `json:"-"`
	Bits    int          `json:"-"`
	Reg     byte         `json:"-"` // The 4-bit encoding of the register for ModR/M.reg.
	Addr    byte         `json:"-"` // The 4-bit encoding of the register in address form.
	MinMode uint8        `json:"-"` // Any CPU mode requirements as a number of bits.
	EVEX    bool         `json:"-"` // Whether the register can only be used with EVEX encoding.
	Aliases []string     `json:"-"`
}

func (r *Register) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Name)
}

func (r *Register) UnmarshalJSON(data []byte) error {
	var s string
	err := json.Unmarshal(data, &s)
	if err != nil {
		return err
	}

	got, ok := RegistersByName[s]
	if !ok {
		return fmt.Errorf("invalid register %q", s)
	}

	*r = *got

	return nil
}

func (r *Register) IsRegister() bool  { return true }
func (r *Register) String() string    { return r.Name }
func (r *Register) UpperName() string { return strings.ToUpper(r.Name) }

// Encoding helpers.

// ModRM returns the encoding form to
// store the register in the ModR/M
// byte.
//
// `rex` indicates whether the
// register requires a REX prefix,
// which is necessary for SPL, BPL,
// SIL, DIL, and the extended
// register sets, such as R8-R15
// and XMM8-XMM15.
//
// `field` indicates whether a REX
// field is needed to encode the
// register.
//
// `reg` is the 3-bit identifier for
// the register.
func (r *Register) ModRM() (evex, rex, field bool, reg byte) {
	rex = r.Reg > 7
	switch r {
	case SPL, BPL, SIL, DIL:
		rex = true
	}

	bit4 := (r.Reg & 0b10000) != 0
	bit3 := (r.Reg & 0b01000) != 0

	return !bit4, rex, bit3, r.Reg & 7
}

// Base returns the encoding form to
// store the base register in a memory
// operand..
//
// `rex` indicates whether the
// register requires a REX prefix,
// which is necessary for SPL, BPL,
// SIL, DIL, and the extended
// register sets, such as R8-R15
// and XMM8-XMM15.
//
// `field` indicates whether a REX
// field is needed to encode the
// register.
//
// `reg` is the 3-bit identifier for
// the register.
func (r *Register) Base() (rex, field bool, reg byte) {
	rex = r.Addr > 7
	switch r {
	case SPL, BPL, SIL, DIL:
		rex = true
	}

	bit3 := (r.Reg & 0b01000) != 0

	return rex, bit3, r.Addr & 7
}

// VEXvvvv returns the 4-bit identifier
// for the register, as used in the
// VEX.vvvv field, plus the fifth bit
// for EVEX.V'.
func (r *Register) VEXvvvv() (vp bool, vvvv byte) {
	return r.Reg < 16, ^r.Reg & 0xf
}

// VEXis4 returns the 4-bit identifier
// for the register, as used in the
// VEX /is4 field.
func (r *Register) VEXis4() byte {
	return r.Reg << 4
}

var (
	// 8-bit registers.
	AL   = &Register{Name: "al", Type: TypeGeneralPurpose, Reg: 0x0, Addr: 0x0, Bits: 8}
	CL   = &Register{Name: "cl", Type: TypeGeneralPurpose, Reg: 0x1, Addr: 0x1, Bits: 8}
	DL   = &Register{Name: "dl", Type: TypeGeneralPurpose, Reg: 0x2, Addr: 0x2, Bits: 8}
	BL   = &Register{Name: "bl", Type: TypeGeneralPurpose, Reg: 0x3, Addr: 0x3, Bits: 8}
	AH   = &Register{Name: "ah", Type: TypeGeneralPurpose, Reg: 0x4, Addr: 0x4, Bits: 8}
	CH   = &Register{Name: "ch", Type: TypeGeneralPurpose, Reg: 0x5, Addr: 0x5, Bits: 8}
	DH   = &Register{Name: "dh", Type: TypeGeneralPurpose, Reg: 0x6, Addr: 0x6, Bits: 8}
	BH   = &Register{Name: "bh", Type: TypeGeneralPurpose, Reg: 0x7, Addr: 0x7, Bits: 8}
	SPL  = &Register{Name: "spl", Type: TypeGeneralPurpose, Reg: 0x4, Addr: 0x4, Bits: 8, MinMode: 64}
	BPL  = &Register{Name: "bpl", Type: TypeGeneralPurpose, Reg: 0x5, Addr: 0x5, Bits: 8, MinMode: 64}
	SIL  = &Register{Name: "sil", Type: TypeGeneralPurpose, Reg: 0x6, Addr: 0x6, Bits: 8, MinMode: 64}
	DIL  = &Register{Name: "dil", Type: TypeGeneralPurpose, Reg: 0x7, Addr: 0x7, Bits: 8, MinMode: 64}
	R8L  = &Register{Name: "r8l", Type: TypeGeneralPurpose, Reg: 0x8, Addr: 0x8, Bits: 8, MinMode: 64, Aliases: []string{"r8b"}}
	R9L  = &Register{Name: "r9l", Type: TypeGeneralPurpose, Reg: 0x9, Addr: 0x9, Bits: 8, MinMode: 64, Aliases: []string{"r9b"}}
	R10L = &Register{Name: "r10l", Type: TypeGeneralPurpose, Reg: 0xa, Addr: 0xa, Bits: 8, MinMode: 64, Aliases: []string{"r10b"}}
	R11L = &Register{Name: "r11l", Type: TypeGeneralPurpose, Reg: 0xb, Addr: 0xb, Bits: 8, MinMode: 64, Aliases: []string{"r11b"}}
	R12L = &Register{Name: "r12l", Type: TypeGeneralPurpose, Reg: 0xc, Addr: 0xc, Bits: 8, MinMode: 64, Aliases: []string{"r12b"}}
	R13L = &Register{Name: "r13l", Type: TypeGeneralPurpose, Reg: 0xd, Addr: 0xd, Bits: 8, MinMode: 64, Aliases: []string{"r13b"}}
	R14L = &Register{Name: "r14l", Type: TypeGeneralPurpose, Reg: 0xe, Addr: 0xe, Bits: 8, MinMode: 64, Aliases: []string{"r14b"}}
	R15L = &Register{Name: "r15l", Type: TypeGeneralPurpose, Reg: 0xf, Addr: 0xf, Bits: 8, MinMode: 64, Aliases: []string{"r15b"}}

	// 16-bit registers.
	AX   = &Register{Name: "ax", Type: TypeGeneralPurpose, Reg: 0x0, Addr: 0x0, Bits: 16}
	CX   = &Register{Name: "cx", Type: TypeGeneralPurpose, Reg: 0x1, Addr: 0x1, Bits: 16}
	DX   = &Register{Name: "dx", Type: TypeGeneralPurpose, Reg: 0x2, Addr: 0x2, Bits: 16}
	BX   = &Register{Name: "bx", Type: TypeGeneralPurpose, Reg: 0x3, Addr: 0x7, Bits: 16}
	SP   = &Register{Name: "sp", Type: TypeGeneralPurpose, Reg: 0x4, Addr: 0x4, Bits: 16}
	BP   = &Register{Name: "bp", Type: TypeGeneralPurpose, Reg: 0x5, Addr: 0x6, Bits: 16}
	SI   = &Register{Name: "si", Type: TypeGeneralPurpose, Reg: 0x6, Addr: 0x4, Bits: 16}
	DI   = &Register{Name: "di", Type: TypeGeneralPurpose, Reg: 0x7, Addr: 0x5, Bits: 16}
	R8W  = &Register{Name: "r8w", Type: TypeGeneralPurpose, Reg: 0x8, Addr: 0x8, Bits: 16, MinMode: 64}
	R9W  = &Register{Name: "r9w", Type: TypeGeneralPurpose, Reg: 0x9, Addr: 0x9, Bits: 16, MinMode: 64}
	R10W = &Register{Name: "r10w", Type: TypeGeneralPurpose, Reg: 0xa, Addr: 0xa, Bits: 16, MinMode: 64}
	R11W = &Register{Name: "r11w", Type: TypeGeneralPurpose, Reg: 0xb, Addr: 0xb, Bits: 16, MinMode: 64}
	R12W = &Register{Name: "r12w", Type: TypeGeneralPurpose, Reg: 0xc, Addr: 0xc, Bits: 16, MinMode: 64}
	R13W = &Register{Name: "r13w", Type: TypeGeneralPurpose, Reg: 0xd, Addr: 0xd, Bits: 16, MinMode: 64}
	R14W = &Register{Name: "r14w", Type: TypeGeneralPurpose, Reg: 0xe, Addr: 0xe, Bits: 16, MinMode: 64}
	R15W = &Register{Name: "r15w", Type: TypeGeneralPurpose, Reg: 0xf, Addr: 0xf, Bits: 16, MinMode: 64}

	// 32-bit registers.
	EAX  = &Register{Name: "eax", Type: TypeGeneralPurpose, Reg: 0x0, Addr: 0x0, Bits: 32}
	ECX  = &Register{Name: "ecx", Type: TypeGeneralPurpose, Reg: 0x1, Addr: 0x1, Bits: 32}
	EDX  = &Register{Name: "edx", Type: TypeGeneralPurpose, Reg: 0x2, Addr: 0x2, Bits: 32}
	EBX  = &Register{Name: "ebx", Type: TypeGeneralPurpose, Reg: 0x3, Addr: 0x3, Bits: 32}
	ESP  = &Register{Name: "esp", Type: TypeGeneralPurpose, Reg: 0x4, Addr: 0x4, Bits: 32}
	EBP  = &Register{Name: "ebp", Type: TypeGeneralPurpose, Reg: 0x5, Addr: 0x5, Bits: 32}
	ESI  = &Register{Name: "esi", Type: TypeGeneralPurpose, Reg: 0x6, Addr: 0x6, Bits: 32}
	EDI  = &Register{Name: "edi", Type: TypeGeneralPurpose, Reg: 0x7, Addr: 0x7, Bits: 32}
	R8D  = &Register{Name: "r8d", Type: TypeGeneralPurpose, Reg: 0x8, Addr: 0x8, Bits: 32, MinMode: 64}
	R9D  = &Register{Name: "r9d", Type: TypeGeneralPurpose, Reg: 0x9, Addr: 0x9, Bits: 32, MinMode: 64}
	R10D = &Register{Name: "r10d", Type: TypeGeneralPurpose, Reg: 0xa, Addr: 0xa, Bits: 32, MinMode: 64}
	R11D = &Register{Name: "r11d", Type: TypeGeneralPurpose, Reg: 0xb, Addr: 0xb, Bits: 32, MinMode: 64}
	R12D = &Register{Name: "r12d", Type: TypeGeneralPurpose, Reg: 0xc, Addr: 0xc, Bits: 32, MinMode: 64}
	R13D = &Register{Name: "r13d", Type: TypeGeneralPurpose, Reg: 0xd, Addr: 0xd, Bits: 32, MinMode: 64}
	R14D = &Register{Name: "r14d", Type: TypeGeneralPurpose, Reg: 0xe, Addr: 0xe, Bits: 32, MinMode: 64}
	R15D = &Register{Name: "r15d", Type: TypeGeneralPurpose, Reg: 0xf, Addr: 0xf, Bits: 32, MinMode: 64}

	// 64-bit registers.
	RAX = &Register{Name: "rax", Type: TypeGeneralPurpose, Reg: 0x0, Addr: 0x0, Bits: 64, MinMode: 64}
	RCX = &Register{Name: "rcx", Type: TypeGeneralPurpose, Reg: 0x1, Addr: 0x1, Bits: 64, MinMode: 64}
	RDX = &Register{Name: "rdx", Type: TypeGeneralPurpose, Reg: 0x2, Addr: 0x2, Bits: 64, MinMode: 64}
	RBX = &Register{Name: "rbx", Type: TypeGeneralPurpose, Reg: 0x3, Addr: 0x3, Bits: 64, MinMode: 64}
	RSP = &Register{Name: "rsp", Type: TypeGeneralPurpose, Reg: 0x4, Addr: 0x4, Bits: 64, MinMode: 64}
	RBP = &Register{Name: "rbp", Type: TypeGeneralPurpose, Reg: 0x5, Addr: 0x5, Bits: 64, MinMode: 64}
	RSI = &Register{Name: "rsi", Type: TypeGeneralPurpose, Reg: 0x6, Addr: 0x6, Bits: 64, MinMode: 64}
	RDI = &Register{Name: "rdi", Type: TypeGeneralPurpose, Reg: 0x7, Addr: 0x7, Bits: 64, MinMode: 64}
	R8  = &Register{Name: "r8", Type: TypeGeneralPurpose, Reg: 0x8, Addr: 0x8, Bits: 64, MinMode: 64}
	R9  = &Register{Name: "r9", Type: TypeGeneralPurpose, Reg: 0x9, Addr: 0x9, Bits: 64, MinMode: 64}
	R10 = &Register{Name: "r10", Type: TypeGeneralPurpose, Reg: 0xa, Addr: 0xa, Bits: 64, MinMode: 64}
	R11 = &Register{Name: "r11", Type: TypeGeneralPurpose, Reg: 0xb, Addr: 0xb, Bits: 64, MinMode: 64}
	R12 = &Register{Name: "r12", Type: TypeGeneralPurpose, Reg: 0xc, Addr: 0xc, Bits: 64, MinMode: 64}
	R13 = &Register{Name: "r13", Type: TypeGeneralPurpose, Reg: 0xd, Addr: 0xd, Bits: 64, MinMode: 64}
	R14 = &Register{Name: "r14", Type: TypeGeneralPurpose, Reg: 0xe, Addr: 0xe, Bits: 64, MinMode: 64}
	R15 = &Register{Name: "r15", Type: TypeGeneralPurpose, Reg: 0xf, Addr: 0xf, Bits: 64, MinMode: 64}

	// Legacy pseudo register pair.
	BX_SI = &Register{Name: "bx_si", Type: TypePair, Addr: 0x0, Bits: 16}
	BX_DI = &Register{Name: "bx_di", Type: TypePair, Addr: 0x1, Bits: 16}
	BP_SI = &Register{Name: "bp_si", Type: TypePair, Addr: 0x2, Bits: 16}
	BP_DI = &Register{Name: "bp_di", Type: TypePair, Addr: 0x3, Bits: 16}

	// Instruction pointer.
	IP  = &Register{Name: "ip", Type: TypeInstructionPointer, Bits: 16}
	EIP = &Register{Name: "eip", Type: TypeInstructionPointer, Bits: 32}
	RIP = &Register{Name: "rip", Type: TypeInstructionPointer, Addr: 0x5, Bits: 64, MinMode: 64}

	// Segment registers.
	ES = &Register{Name: "es", Type: TypeSegment, Reg: 0x0, Addr: 0x0, Bits: 16}
	CS = &Register{Name: "cs", Type: TypeSegment, Reg: 0x1, Addr: 0x1, Bits: 16}
	SS = &Register{Name: "ss", Type: TypeSegment, Reg: 0x2, Addr: 0x2, Bits: 16}
	DS = &Register{Name: "ds", Type: TypeSegment, Reg: 0x3, Addr: 0x3, Bits: 16}
	FS = &Register{Name: "fs", Type: TypeSegment, Reg: 0x4, Addr: 0x4, Bits: 16}
	GS = &Register{Name: "gs", Type: TypeSegment, Reg: 0x5, Addr: 0x5, Bits: 16}

	// x87 floating point stack positions.
	ST0 = &Register{Name: "st0", Type: TypeX87, Bits: 80, Reg: 0}
	ST1 = &Register{Name: "st1", Type: TypeX87, Bits: 80, Reg: 1}
	ST2 = &Register{Name: "st2", Type: TypeX87, Bits: 80, Reg: 2}
	ST3 = &Register{Name: "st3", Type: TypeX87, Bits: 80, Reg: 3}
	ST4 = &Register{Name: "st4", Type: TypeX87, Bits: 80, Reg: 4}
	ST5 = &Register{Name: "st5", Type: TypeX87, Bits: 80, Reg: 5}
	ST6 = &Register{Name: "st6", Type: TypeX87, Bits: 80, Reg: 6}
	ST7 = &Register{Name: "st7", Type: TypeX87, Bits: 80, Reg: 7}

	// Control registers.
	CR0  = &Register{Name: "cr0", Type: TypeControl, Reg: 0}
	CR1  = &Register{Name: "cr1", Type: TypeControl, Reg: 1}
	CR2  = &Register{Name: "cr2", Type: TypeControl, Reg: 2}
	CR3  = &Register{Name: "cr3", Type: TypeControl, Reg: 3}
	CR4  = &Register{Name: "cr4", Type: TypeControl, Reg: 4}
	CR5  = &Register{Name: "cr5", Type: TypeControl, Reg: 5}
	CR6  = &Register{Name: "cr6", Type: TypeControl, Reg: 6}
	CR7  = &Register{Name: "cr7", Type: TypeControl, Reg: 7}
	CR8  = &Register{Name: "cr8", Type: TypeControl, Reg: 8, MinMode: 64}
	CR9  = &Register{Name: "cr9", Type: TypeControl, Reg: 9, MinMode: 64}
	CR10 = &Register{Name: "cr10", Type: TypeControl, Reg: 10, MinMode: 64}
	CR11 = &Register{Name: "cr11", Type: TypeControl, Reg: 11, MinMode: 64}
	CR12 = &Register{Name: "cr12", Type: TypeControl, Reg: 12, MinMode: 64}
	CR13 = &Register{Name: "cr13", Type: TypeControl, Reg: 13, MinMode: 64}
	CR14 = &Register{Name: "cr14", Type: TypeControl, Reg: 14, MinMode: 64}
	CR15 = &Register{Name: "cr15", Type: TypeControl, Reg: 15, MinMode: 64}

	// Debug registers.
	DR0  = &Register{Name: "dr0", Type: TypeDebug, Reg: 0}
	DR1  = &Register{Name: "dr1", Type: TypeDebug, Reg: 1}
	DR2  = &Register{Name: "dr2", Type: TypeDebug, Reg: 2}
	DR3  = &Register{Name: "dr3", Type: TypeDebug, Reg: 3}
	DR4  = &Register{Name: "dr4", Type: TypeDebug, Reg: 4}
	DR5  = &Register{Name: "dr5", Type: TypeDebug, Reg: 5}
	DR6  = &Register{Name: "dr6", Type: TypeDebug, Reg: 6}
	DR7  = &Register{Name: "dr7", Type: TypeDebug, Reg: 7}
	DR8  = &Register{Name: "dr8", Type: TypeDebug, Reg: 8, MinMode: 64}
	DR9  = &Register{Name: "dr9", Type: TypeDebug, Reg: 9, MinMode: 64}
	DR10 = &Register{Name: "dr10", Type: TypeDebug, Reg: 10, MinMode: 64}
	DR11 = &Register{Name: "dr11", Type: TypeDebug, Reg: 11, MinMode: 64}
	DR12 = &Register{Name: "dr12", Type: TypeDebug, Reg: 12, MinMode: 64}
	DR13 = &Register{Name: "dr13", Type: TypeDebug, Reg: 13, MinMode: 64}
	DR14 = &Register{Name: "dr14", Type: TypeDebug, Reg: 14, MinMode: 64}
	DR15 = &Register{Name: "dr15", Type: TypeDebug, Reg: 15, MinMode: 64}

	// Opmask registers.
	K0 = &Register{Name: "k0", Type: TypeOpmask, Reg: 0}
	K1 = &Register{Name: "k1", Type: TypeOpmask, Reg: 1}
	K2 = &Register{Name: "k2", Type: TypeOpmask, Reg: 2}
	K3 = &Register{Name: "k3", Type: TypeOpmask, Reg: 3}
	K4 = &Register{Name: "k4", Type: TypeOpmask, Reg: 4}
	K5 = &Register{Name: "k5", Type: TypeOpmask, Reg: 5}
	K6 = &Register{Name: "k6", Type: TypeOpmask, Reg: 6}
	K7 = &Register{Name: "k7", Type: TypeOpmask, Reg: 7}

	// 387 floating-point registers.
	F0 = &Register{Name: "f0", Type: TypeFloat}
	F1 = &Register{Name: "f1", Type: TypeFloat}
	F2 = &Register{Name: "f2", Type: TypeFloat}
	F3 = &Register{Name: "f3", Type: TypeFloat}
	F4 = &Register{Name: "f4", Type: TypeFloat}
	F5 = &Register{Name: "f5", Type: TypeFloat}
	F6 = &Register{Name: "f6", Type: TypeFloat}
	F7 = &Register{Name: "f7", Type: TypeFloat}

	// Bounds registers.
	BND0 = &Register{Name: "bnd0", Type: TypeBounds, Reg: 0x0}
	BND1 = &Register{Name: "bnd1", Type: TypeBounds, Reg: 0x1}
	BND2 = &Register{Name: "bnd2", Type: TypeBounds, Reg: 0x2}

	// MMX registers.
	MMX0 = &Register{Name: "mmx0", Type: TypeMMX, Reg: 0x0, Addr: 0x0, Aliases: []string{"mm0"}}
	MMX1 = &Register{Name: "mmx1", Type: TypeMMX, Reg: 0x1, Addr: 0x1, Aliases: []string{"mm1"}}
	MMX2 = &Register{Name: "mmx2", Type: TypeMMX, Reg: 0x2, Addr: 0x2, Aliases: []string{"mm2"}}
	MMX3 = &Register{Name: "mmx3", Type: TypeMMX, Reg: 0x3, Addr: 0x3, Aliases: []string{"mm3"}}
	MMX4 = &Register{Name: "mmx4", Type: TypeMMX, Reg: 0x4, Addr: 0x4, Aliases: []string{"mm4"}}
	MMX5 = &Register{Name: "mmx5", Type: TypeMMX, Reg: 0x5, Addr: 0x5, Aliases: []string{"mm5"}}
	MMX6 = &Register{Name: "mmx6", Type: TypeMMX, Reg: 0x6, Addr: 0x6, Aliases: []string{"mm6"}}
	MMX7 = &Register{Name: "mmx7", Type: TypeMMX, Reg: 0x7, Addr: 0x7, Aliases: []string{"mm7"}}

	// TMM registers.
	TMM0 = &Register{Name: "tmm0", Type: TypeTMM, Reg: 0x0, Addr: 0x0}
	TMM1 = &Register{Name: "tmm1", Type: TypeTMM, Reg: 0x1, Addr: 0x1}
	TMM2 = &Register{Name: "tmm2", Type: TypeTMM, Reg: 0x2, Addr: 0x2}
	TMM3 = &Register{Name: "tmm3", Type: TypeTMM, Reg: 0x3, Addr: 0x3}
	TMM4 = &Register{Name: "tmm4", Type: TypeTMM, Reg: 0x4, Addr: 0x4}
	TMM5 = &Register{Name: "tmm5", Type: TypeTMM, Reg: 0x5, Addr: 0x5}
	TMM6 = &Register{Name: "tmm6", Type: TypeTMM, Reg: 0x6, Addr: 0x6}
	TMM7 = &Register{Name: "tmm7", Type: TypeTMM, Reg: 0x7, Addr: 0x7}

	// XMM registers.
	XMM0  = &Register{Name: "xmm0", Type: TypeXMM, Reg: 0x00, Addr: 0x00, Bits: 128}
	XMM1  = &Register{Name: "xmm1", Type: TypeXMM, Reg: 0x01, Addr: 0x01, Bits: 128}
	XMM2  = &Register{Name: "xmm2", Type: TypeXMM, Reg: 0x02, Addr: 0x02, Bits: 128}
	XMM3  = &Register{Name: "xmm3", Type: TypeXMM, Reg: 0x03, Addr: 0x03, Bits: 128}
	XMM4  = &Register{Name: "xmm4", Type: TypeXMM, Reg: 0x04, Addr: 0x04, Bits: 128}
	XMM5  = &Register{Name: "xmm5", Type: TypeXMM, Reg: 0x05, Addr: 0x05, Bits: 128}
	XMM6  = &Register{Name: "xmm6", Type: TypeXMM, Reg: 0x06, Addr: 0x06, Bits: 128}
	XMM7  = &Register{Name: "xmm7", Type: TypeXMM, Reg: 0x07, Addr: 0x07, Bits: 128}
	XMM8  = &Register{Name: "xmm8", Type: TypeXMM, Reg: 0x08, Addr: 0x08, Bits: 128, MinMode: 64}
	XMM9  = &Register{Name: "xmm9", Type: TypeXMM, Reg: 0x09, Addr: 0x09, Bits: 128, MinMode: 64}
	XMM10 = &Register{Name: "xmm10", Type: TypeXMM, Reg: 0x0a, Addr: 0x0a, Bits: 128, MinMode: 64}
	XMM11 = &Register{Name: "xmm11", Type: TypeXMM, Reg: 0x0b, Addr: 0x0b, Bits: 128, MinMode: 64}
	XMM12 = &Register{Name: "xmm12", Type: TypeXMM, Reg: 0x0c, Addr: 0x0c, Bits: 128, MinMode: 64}
	XMM13 = &Register{Name: "xmm13", Type: TypeXMM, Reg: 0x0d, Addr: 0x0d, Bits: 128, MinMode: 64}
	XMM14 = &Register{Name: "xmm14", Type: TypeXMM, Reg: 0x0e, Addr: 0x0e, Bits: 128, MinMode: 64}
	XMM15 = &Register{Name: "xmm15", Type: TypeXMM, Reg: 0x0f, Addr: 0x0f, Bits: 128, MinMode: 64}
	XMM16 = &Register{Name: "xmm16", Type: TypeXMM, Reg: 0x10, Addr: 0x10, Bits: 128, MinMode: 64, EVEX: true}
	XMM17 = &Register{Name: "xmm17", Type: TypeXMM, Reg: 0x11, Addr: 0x11, Bits: 128, MinMode: 64, EVEX: true}
	XMM18 = &Register{Name: "xmm18", Type: TypeXMM, Reg: 0x12, Addr: 0x12, Bits: 128, MinMode: 64, EVEX: true}
	XMM19 = &Register{Name: "xmm19", Type: TypeXMM, Reg: 0x13, Addr: 0x13, Bits: 128, MinMode: 64, EVEX: true}
	XMM20 = &Register{Name: "xmm20", Type: TypeXMM, Reg: 0x14, Addr: 0x14, Bits: 128, MinMode: 64, EVEX: true}
	XMM21 = &Register{Name: "xmm21", Type: TypeXMM, Reg: 0x15, Addr: 0x15, Bits: 128, MinMode: 64, EVEX: true}
	XMM22 = &Register{Name: "xmm22", Type: TypeXMM, Reg: 0x16, Addr: 0x16, Bits: 128, MinMode: 64, EVEX: true}
	XMM23 = &Register{Name: "xmm23", Type: TypeXMM, Reg: 0x17, Addr: 0x17, Bits: 128, MinMode: 64, EVEX: true}
	XMM24 = &Register{Name: "xmm24", Type: TypeXMM, Reg: 0x18, Addr: 0x18, Bits: 128, MinMode: 64, EVEX: true}
	XMM25 = &Register{Name: "xmm25", Type: TypeXMM, Reg: 0x19, Addr: 0x19, Bits: 128, MinMode: 64, EVEX: true}
	XMM26 = &Register{Name: "xmm26", Type: TypeXMM, Reg: 0x1a, Addr: 0x1a, Bits: 128, MinMode: 64, EVEX: true}
	XMM27 = &Register{Name: "xmm27", Type: TypeXMM, Reg: 0x1b, Addr: 0x1b, Bits: 128, MinMode: 64, EVEX: true}
	XMM28 = &Register{Name: "xmm28", Type: TypeXMM, Reg: 0x1c, Addr: 0x1c, Bits: 128, MinMode: 64, EVEX: true}
	XMM29 = &Register{Name: "xmm29", Type: TypeXMM, Reg: 0x1d, Addr: 0x1d, Bits: 128, MinMode: 64, EVEX: true}
	XMM30 = &Register{Name: "xmm30", Type: TypeXMM, Reg: 0x1e, Addr: 0x1e, Bits: 128, MinMode: 64, EVEX: true}
	XMM31 = &Register{Name: "xmm31", Type: TypeXMM, Reg: 0x1f, Addr: 0x1f, Bits: 128, MinMode: 64, EVEX: true}

	// YMM registers.
	YMM0  = &Register{Name: "ymm0", Type: TypeYMM, Reg: 0x00, Addr: 0x00, Bits: 256}
	YMM1  = &Register{Name: "ymm1", Type: TypeYMM, Reg: 0x01, Addr: 0x01, Bits: 256}
	YMM2  = &Register{Name: "ymm2", Type: TypeYMM, Reg: 0x02, Addr: 0x02, Bits: 256}
	YMM3  = &Register{Name: "ymm3", Type: TypeYMM, Reg: 0x03, Addr: 0x03, Bits: 256}
	YMM4  = &Register{Name: "ymm4", Type: TypeYMM, Reg: 0x04, Addr: 0x04, Bits: 256}
	YMM5  = &Register{Name: "ymm5", Type: TypeYMM, Reg: 0x05, Addr: 0x05, Bits: 256}
	YMM6  = &Register{Name: "ymm6", Type: TypeYMM, Reg: 0x06, Addr: 0x06, Bits: 256}
	YMM7  = &Register{Name: "ymm7", Type: TypeYMM, Reg: 0x07, Addr: 0x07, Bits: 256}
	YMM8  = &Register{Name: "ymm8", Type: TypeYMM, Reg: 0x08, Addr: 0x08, Bits: 256, MinMode: 64}
	YMM9  = &Register{Name: "ymm9", Type: TypeYMM, Reg: 0x09, Addr: 0x09, Bits: 256, MinMode: 64}
	YMM10 = &Register{Name: "ymm10", Type: TypeYMM, Reg: 0x0a, Addr: 0x0a, Bits: 256, MinMode: 64}
	YMM11 = &Register{Name: "ymm11", Type: TypeYMM, Reg: 0x0b, Addr: 0x0b, Bits: 256, MinMode: 64}
	YMM12 = &Register{Name: "ymm12", Type: TypeYMM, Reg: 0x0c, Addr: 0x0c, Bits: 256, MinMode: 64}
	YMM13 = &Register{Name: "ymm13", Type: TypeYMM, Reg: 0x0d, Addr: 0x0d, Bits: 256, MinMode: 64}
	YMM14 = &Register{Name: "ymm14", Type: TypeYMM, Reg: 0x0e, Addr: 0x0e, Bits: 256, MinMode: 64}
	YMM15 = &Register{Name: "ymm15", Type: TypeYMM, Reg: 0x0f, Addr: 0x0f, Bits: 256, MinMode: 64}
	YMM16 = &Register{Name: "ymm16", Type: TypeYMM, Reg: 0x10, Addr: 0x10, Bits: 256, MinMode: 64, EVEX: true}
	YMM17 = &Register{Name: "ymm17", Type: TypeYMM, Reg: 0x11, Addr: 0x11, Bits: 256, MinMode: 64, EVEX: true}
	YMM18 = &Register{Name: "ymm18", Type: TypeYMM, Reg: 0x12, Addr: 0x12, Bits: 256, MinMode: 64, EVEX: true}
	YMM19 = &Register{Name: "ymm19", Type: TypeYMM, Reg: 0x13, Addr: 0x13, Bits: 256, MinMode: 64, EVEX: true}
	YMM20 = &Register{Name: "ymm20", Type: TypeYMM, Reg: 0x14, Addr: 0x14, Bits: 256, MinMode: 64, EVEX: true}
	YMM21 = &Register{Name: "ymm21", Type: TypeYMM, Reg: 0x15, Addr: 0x15, Bits: 256, MinMode: 64, EVEX: true}
	YMM22 = &Register{Name: "ymm22", Type: TypeYMM, Reg: 0x16, Addr: 0x16, Bits: 256, MinMode: 64, EVEX: true}
	YMM23 = &Register{Name: "ymm23", Type: TypeYMM, Reg: 0x17, Addr: 0x17, Bits: 256, MinMode: 64, EVEX: true}
	YMM24 = &Register{Name: "ymm24", Type: TypeYMM, Reg: 0x18, Addr: 0x18, Bits: 256, MinMode: 64, EVEX: true}
	YMM25 = &Register{Name: "ymm25", Type: TypeYMM, Reg: 0x19, Addr: 0x19, Bits: 256, MinMode: 64, EVEX: true}
	YMM26 = &Register{Name: "ymm26", Type: TypeYMM, Reg: 0x1a, Addr: 0x1a, Bits: 256, MinMode: 64, EVEX: true}
	YMM27 = &Register{Name: "ymm27", Type: TypeYMM, Reg: 0x1b, Addr: 0x1b, Bits: 256, MinMode: 64, EVEX: true}
	YMM28 = &Register{Name: "ymm28", Type: TypeYMM, Reg: 0x1c, Addr: 0x1c, Bits: 256, MinMode: 64, EVEX: true}
	YMM29 = &Register{Name: "ymm29", Type: TypeYMM, Reg: 0x1d, Addr: 0x1d, Bits: 256, MinMode: 64, EVEX: true}
	YMM30 = &Register{Name: "ymm30", Type: TypeYMM, Reg: 0x1e, Addr: 0x1e, Bits: 256, MinMode: 64, EVEX: true}
	YMM31 = &Register{Name: "ymm31", Type: TypeYMM, Reg: 0x1f, Addr: 0x1f, Bits: 256, MinMode: 64, EVEX: true}

	// ZMM registers.
	ZMM0  = &Register{Name: "zmm0", Type: TypeZMM, Reg: 0x00, Addr: 0x00, Bits: 512, MinMode: 64, EVEX: true}
	ZMM1  = &Register{Name: "zmm1", Type: TypeZMM, Reg: 0x01, Addr: 0x01, Bits: 512, MinMode: 64, EVEX: true}
	ZMM2  = &Register{Name: "zmm2", Type: TypeZMM, Reg: 0x02, Addr: 0x02, Bits: 512, MinMode: 64, EVEX: true}
	ZMM3  = &Register{Name: "zmm3", Type: TypeZMM, Reg: 0x03, Addr: 0x03, Bits: 512, MinMode: 64, EVEX: true}
	ZMM4  = &Register{Name: "zmm4", Type: TypeZMM, Reg: 0x04, Addr: 0x04, Bits: 512, MinMode: 64, EVEX: true}
	ZMM5  = &Register{Name: "zmm5", Type: TypeZMM, Reg: 0x05, Addr: 0x05, Bits: 512, MinMode: 64, EVEX: true}
	ZMM6  = &Register{Name: "zmm6", Type: TypeZMM, Reg: 0x06, Addr: 0x06, Bits: 512, MinMode: 64, EVEX: true}
	ZMM7  = &Register{Name: "zmm7", Type: TypeZMM, Reg: 0x07, Addr: 0x07, Bits: 512, MinMode: 64, EVEX: true}
	ZMM8  = &Register{Name: "zmm8", Type: TypeZMM, Reg: 0x08, Addr: 0x08, Bits: 512, MinMode: 64, EVEX: true}
	ZMM9  = &Register{Name: "zmm9", Type: TypeZMM, Reg: 0x09, Addr: 0x09, Bits: 512, MinMode: 64, EVEX: true}
	ZMM10 = &Register{Name: "zmm10", Type: TypeZMM, Reg: 0x0a, Addr: 0x0a, Bits: 512, MinMode: 64, EVEX: true}
	ZMM11 = &Register{Name: "zmm11", Type: TypeZMM, Reg: 0x0b, Addr: 0x0b, Bits: 512, MinMode: 64, EVEX: true}
	ZMM12 = &Register{Name: "zmm12", Type: TypeZMM, Reg: 0x0c, Addr: 0x0c, Bits: 512, MinMode: 64, EVEX: true}
	ZMM13 = &Register{Name: "zmm13", Type: TypeZMM, Reg: 0x0d, Addr: 0x0d, Bits: 512, MinMode: 64, EVEX: true}
	ZMM14 = &Register{Name: "zmm14", Type: TypeZMM, Reg: 0x0e, Addr: 0x0e, Bits: 512, MinMode: 64, EVEX: true}
	ZMM15 = &Register{Name: "zmm15", Type: TypeZMM, Reg: 0x0f, Addr: 0x0f, Bits: 512, MinMode: 64, EVEX: true}
	ZMM16 = &Register{Name: "zmm16", Type: TypeZMM, Reg: 0x10, Addr: 0x10, Bits: 512, MinMode: 64, EVEX: true}
	ZMM17 = &Register{Name: "zmm17", Type: TypeZMM, Reg: 0x11, Addr: 0x11, Bits: 512, MinMode: 64, EVEX: true}
	ZMM18 = &Register{Name: "zmm18", Type: TypeZMM, Reg: 0x12, Addr: 0x12, Bits: 512, MinMode: 64, EVEX: true}
	ZMM19 = &Register{Name: "zmm19", Type: TypeZMM, Reg: 0x13, Addr: 0x13, Bits: 512, MinMode: 64, EVEX: true}
	ZMM20 = &Register{Name: "zmm20", Type: TypeZMM, Reg: 0x14, Addr: 0x14, Bits: 512, MinMode: 64, EVEX: true}
	ZMM21 = &Register{Name: "zmm21", Type: TypeZMM, Reg: 0x15, Addr: 0x15, Bits: 512, MinMode: 64, EVEX: true}
	ZMM22 = &Register{Name: "zmm22", Type: TypeZMM, Reg: 0x16, Addr: 0x16, Bits: 512, MinMode: 64, EVEX: true}
	ZMM23 = &Register{Name: "zmm23", Type: TypeZMM, Reg: 0x17, Addr: 0x17, Bits: 512, MinMode: 64, EVEX: true}
	ZMM24 = &Register{Name: "zmm24", Type: TypeZMM, Reg: 0x18, Addr: 0x18, Bits: 512, MinMode: 64, EVEX: true}
	ZMM25 = &Register{Name: "zmm25", Type: TypeZMM, Reg: 0x19, Addr: 0x19, Bits: 512, MinMode: 64, EVEX: true}
	ZMM26 = &Register{Name: "zmm26", Type: TypeZMM, Reg: 0x1a, Addr: 0x1a, Bits: 512, MinMode: 64, EVEX: true}
	ZMM27 = &Register{Name: "zmm27", Type: TypeZMM, Reg: 0x1b, Addr: 0x1b, Bits: 512, MinMode: 64, EVEX: true}
	ZMM28 = &Register{Name: "zmm28", Type: TypeZMM, Reg: 0x1c, Addr: 0x1c, Bits: 512, MinMode: 64, EVEX: true}
	ZMM29 = &Register{Name: "zmm29", Type: TypeZMM, Reg: 0x1d, Addr: 0x1d, Bits: 512, MinMode: 64, EVEX: true}
	ZMM30 = &Register{Name: "zmm30", Type: TypeZMM, Reg: 0x1e, Addr: 0x1e, Bits: 512, MinMode: 64, EVEX: true}
	ZMM31 = &Register{Name: "zmm31", Type: TypeZMM, Reg: 0x1f, Addr: 0x1f, Bits: 512, MinMode: 64, EVEX: true}
)

var Registers = []*Register{
	AL, CL, DL, BL, AH, CH, DH, BH, BPL, SPL, DIL, SIL, R8L, R9L, R10L, R11L, R12L, R13L, R14L, R15L,
	AX, CX, DX, BX, BP, SP, DI, SI, R8W, R9W, R10W, R11W, R12W, R13W, R14W, R15W,
	EAX, ECX, EDX, EBX, EBP, ESP, EDI, ESI, R8D, R9D, R10D, R11D, R12D, R13D, R14D, R15D,
	RAX, RCX, RDX, RBX, RBP, RSP, RDI, RSI, R8, R9, R10, R11, R12, R13, R14, R15,
	BX_SI, BX_DI, BP_SI, BP_DI,
	IP, EIP, RIP,
	ES, CS, SS, DS, FS, GS,
	ST0, ST1, ST2, ST3, ST4, ST5, ST6, ST7,
	CR0, CR1, CR2, CR3, CR4, CR5, CR6, CR7, CR8, CR9, CR10, CR11, CR12, CR13, CR14, CR15,
	DR0, DR1, DR2, DR3, DR4, DR5, DR6, DR7, DR8, DR9, DR10, DR11, DR12, DR13, DR14, DR15,
	K0, K1, K2, K3, K4, K5, K6, K7,
	F0, F1, F2, F3, F4, F5, F6, F7,
	BND0, BND1, BND2,
	MMX0, MMX1, MMX2, MMX3, MMX4, MMX5, MMX6, MMX7,
	TMM0, TMM1, TMM2, TMM3, TMM4, TMM5, TMM6, TMM7,
	XMM0, XMM1, XMM2, XMM3, XMM4, XMM5, XMM6, XMM7,
	XMM8, XMM9, XMM10, XMM11, XMM12, XMM13, XMM14, XMM15,
	XMM16, XMM17, XMM18, XMM19, XMM20, XMM21, XMM22, XMM23,
	XMM24, XMM25, XMM26, XMM27, XMM28, XMM29, XMM30, XMM31,
	YMM0, YMM1, YMM2, YMM3, YMM4, YMM5, YMM6, YMM7,
	YMM8, YMM9, YMM10, YMM11, YMM12, YMM13, YMM14, YMM15,
	YMM16, YMM17, YMM18, YMM19, YMM20, YMM21, YMM22, YMM23,
	YMM24, YMM25, YMM26, YMM27, YMM28, YMM29, YMM30, YMM31,
	ZMM0, ZMM1, ZMM2, ZMM3, ZMM4, ZMM5, ZMM6, ZMM7,
	ZMM8, ZMM9, ZMM10, ZMM11, ZMM12, ZMM13, ZMM14, ZMM15,
	ZMM16, ZMM17, ZMM18, ZMM19, ZMM20, ZMM21, ZMM22, ZMM23,
	ZMM24, ZMM25, ZMM26, ZMM27, ZMM28, ZMM29, ZMM30, ZMM31,
}

var (
	// Registers8bitGeneralPurpose contains
	// the 8-bit general purpose registers.
	Registers8bitGeneralPurpose = []*Register{
		AL, CL, DL, BL, AH, CH, DH, BH, BPL, SPL, DIL, SIL,
		R8L, R9L, R10L, R11L, R12L, R13L, R14L, R15L,
	}

	// Registers16bitGeneralPurpose contains
	// the 16-bit general purpose registers.
	Registers16bitGeneralPurpose = []*Register{
		AX, CX, DX, BX, BP, SP, DI, SI,
		R8W, R9W, R10W, R11W, R12W, R13W, R14W, R15W,
	}

	// Registers32bitGeneralPurpose contains
	// the 32-bit general purpose registers.
	Registers32bitGeneralPurpose = []*Register{
		EAX, ECX, EDX, EBX, EBP, ESP, EDI, ESI,
		R8D, R9D, R10D, R11D, R12D, R13D, R14D, R15D,
	}

	// Registers64bitGeneralPurpose contains
	// the 64-bit general purpose registers.
	Registers64bitGeneralPurpose = []*Register{
		RAX, RCX, RDX, RBX, RBP, RSP, RDI, RSI,
		R8, R9, R10, R11, R12, R13, R14, R15,
	}

	// RegistersAddress contains the registers
	// that can be used as base registers in a
	// memory operand.
	RegistersAddress = []*Register{
		BX, BP, DI, SI,
		EAX, ECX, EDX, EBX, EBP, ESP, EDI, ESI,
		R8D, R9D, R10D, R11D, R12D, R13D, R14D, R15D,
		RAX, RCX, RDX, RBX, RBP, RSP, RDI, RSI,
		R8, R9, R10, R11, R12, R13, R14, R15, RIP,
	}

	// RegistersIndex contains the registers
	// that can be used as index registers in
	// a SIB byte memory operand.
	//
	// This also includes the 16-bit SI and
	// DI registers to allow them to be used
	// in a pair with the BX and BP registers.
	// This form is not actually encoded in a
	// SIB byte.
	RegistersIndex = []*Register{
		SI, DI,
		EAX, ECX, EDX, EBX, EBP, ESI, EDI,
		RAX, RCX, RDX, RBX, RBP, RSI, RDI, RSI,
		R8, R9, R10, R11, R12, R13, R14, R15,
	}

	// Registers16bitSegment contains the
	// 16-bit segment registers.
	Registers16bitSegment = []*Register{
		ES, CS, SS, DS, FS, GS,
	}

	// RegistersStackIndices contains the
	// x87 FPU stack indices.
	RegistersStackIndices = []*Register{
		ST0, ST1, ST2, ST3, ST4, ST5, ST6, ST7,
	}

	// RegistersControl contains the
	// control registers.
	RegistersControl = []*Register{
		CR0, CR1, CR2, CR3, CR4, CR5, CR6, CR7,
		CR8, CR9, CR10, CR11, CR12, CR13, CR14, CR15,
	}

	// RegistersDebug contains the debug
	// registers.
	RegistersDebug = []*Register{
		DR0, DR1, DR2, DR3, DR4, DR5, DR6, DR7,
		DR8, DR9, DR10, DR11, DR12, DR13, DR14, DR15,
	}

	// RegistersOpmask contains the opmask
	// registers.
	RegistersOpmask = []*Register{K0, K1, K2, K3, K4, K5, K6, K7}

	// Registers64bitMMX contains the
	// 64-bit MMX registers.
	Registers64bitMMX = []*Register{
		MMX0, MMX1, MMX2, MMX3, MMX4, MMX5, MMX6, MMX7,
	}

	// RegistersTMM contains the
	// TMM tile registers.
	RegistersTMM = []*Register{
		TMM0, TMM1, TMM2, TMM3, TMM4, TMM5, TMM6, TMM7,
	}

	// Registers128bitXMM contains the
	// 128-bit XMM registers.
	Registers128bitXMM = []*Register{
		XMM0, XMM1, XMM2, XMM3, XMM4, XMM5, XMM6, XMM7,
		XMM8, XMM9, XMM10, XMM11, XMM12, XMM13, XMM14, XMM15,
		XMM16, XMM17, XMM18, XMM19, XMM20, XMM21, XMM22, XMM23,
		XMM24, XMM25, XMM26, XMM27, XMM28, XMM29, XMM30, XMM31,
	}

	// Registers256bitYMM contains the
	// 256-bit YMM registers.
	Registers256bitYMM = []*Register{
		YMM0, YMM1, YMM2, YMM3, YMM4, YMM5, YMM6, YMM7,
		YMM8, YMM9, YMM10, YMM11, YMM12, YMM13, YMM14, YMM15,
		YMM16, YMM17, YMM18, YMM19, YMM20, YMM21, YMM22, YMM23,
		YMM24, YMM25, YMM26, YMM27, YMM28, YMM29, YMM30, YMM31,
	}

	// Registers512bitZMM contains the
	// 512-bit ZMM registers.
	Registers512bitZMM = []*Register{
		ZMM0, ZMM1, ZMM2, ZMM3, ZMM4, ZMM5, ZMM6, ZMM7,
		ZMM8, ZMM9, ZMM10, ZMM11, ZMM12, ZMM13, ZMM14, ZMM15,
		ZMM16, ZMM17, ZMM18, ZMM19, ZMM20, ZMM21, ZMM22, ZMM23,
		ZMM24, ZMM25, ZMM26, ZMM27, ZMM28, ZMM29, ZMM30, ZMM31,
	}
)

// RegisterSizes maps the names of fixed-size
// register names (lower case) to their size
// in bits.
var RegisterSizes = make(map[string]int)

var RegistersByName = make(map[string]*Register)

func init() {
	for _, reg := range Registers {
		RegisterSizes[reg.Name] = reg.Bits
		RegistersByName[reg.Name] = reg
	}
}

// RegisterType categorises an x86
// register.
type RegisterType uint8

const (
	_ RegisterType = iota
	TypeGeneralPurpose
	TypePair // Pseudo-registers representing a register pair (like bx+si).
	TypeInstructionPointer
	TypeSegment
	TypeX87
	TypeControl
	TypeDebug
	TypeOpmask
	TypeFloat
	TypeBounds
	TypeMMX
	TypeTMM
	TypeXMM
	TypeYMM
	TypeZMM
)

func (t RegisterType) String() string {
	switch t {
	case TypeGeneralPurpose:
		return "general purpose register"
	case TypeInstructionPointer:
		return "instruction pointer register"
	case TypePair:
		return "register pair"
	case TypeSegment:
		return "segment register"
	case TypeX87:
		return "x87 register"
	case TypeControl:
		return "control register"
	case TypeDebug:
		return "debug register"
	case TypeOpmask:
		return "opmask register"
	case TypeFloat:
		return "float register"
	case TypeBounds:
		return "bounds register"
	case TypeMMX:
		return "MMX register"
	case TypeTMM:
		return "TMM register"
	case TypeXMM:
		return "XMM register"
	case TypeYMM:
		return "YMM register"
	case TypeZMM:
		return "ZMM register"
	default:
		return fmt.Sprintf("RegisterType(%d)", t)
	}
}

var RegisterTypes = map[string]RegisterType{
	"general purpose register":     TypeGeneralPurpose,
	"register pair":                TypePair,
	"instruction pointer register": TypeInstructionPointer,
	"segment register":             TypeSegment,
	"x87 register":                 TypeX87,
	"control register":             TypeControl,
	"debug register":               TypeDebug,
	"opmask register":              TypeOpmask,
	"float register":               TypeFloat,
	"bounds register":              TypeBounds,
	"MMX register":                 TypeMMX,
	"TMM register":                 TypeTMM,
	"XMM register":                 TypeXMM,
	"YMM register":                 TypeYMM,
	"ZMM register":                 TypeZMM,
}
