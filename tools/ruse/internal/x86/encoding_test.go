// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package x86

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestMatchesMachineCode(t *testing.T) {
	tests := []struct {
		Name     string
		Encoding string
		Code     string
		Want     MachineCodeMatch
	}{
		{
			Name:     "opcode only",
			Encoding: "37",
			Code:     "37",
			Want:     Match,
		},
		{
			Name:     "REX-like opcode",
			Encoding: "48+rw",
			Code:     "49", // Looks like a REX prefix and isn't identical to 48.
			Want:     Match,
		},
		{
			Name:     "prefix opcode",
			Encoding: "9B 66 37",
			Code:     "9b 66 67 37 12",
			Want:     Match,
		},
		{
			Name:     "missing prefix",
			Encoding: "66 37",
			Code:     "37 12",
			Want:     MismatchMissingMandatoryPrefix,
		},
		{
			Name:     "reordered prefixes",
			Encoding: "66 67 37",
			Code:     "67 66 37 12",
			Want:     Match,
		},
		{
			Name:     "complex prefixes",
			Encoding: "NFx 66 0F AE /7",
			Code:     "66 0f ae 38",
			Want:     Match,
		},
		{
			Name:     "VEX",
			Encoding: "VEX.256.66.0F38.W0 13 /r",
			Code:     "c4 e2 7d 13 ea",
			Want:     Match,
		},
		{
			Name:     "fixed ModR/M.reg",
			Encoding: "80 /2 ib",
			Code:     "80 d1 80",
			Want:     Match,
		},
		{
			Name:     "VEX extended registers",
			Encoding: "VEX.NDS.256.66.0F.WIG 58 /r",
			Code:     "c5 65 58 f1", // (vaddpd ymm14 ymm3 ymm1)
			Want:     Match,
		},
		{
			Name:     "EVEX extended registers",
			Encoding: "EVEX.256.66.0F.W1 58 /r",
			Code:     "62 11 e5 28 58 f7", // (vaddpd ymm14 ymm3 ymm31)
			Want:     Match,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			encoding, err := ParseEncoding(test.Encoding)
			if err != nil {
				t.Fatalf("ParseEncoding(%q): got unexpected error: %v", test.Encoding, err)
			}

			codeS := strings.ReplaceAll(test.Code, " ", "")
			code, err := hex.DecodeString(codeS)
			if err != nil {
				t.Fatalf("bad code %q: %v", test.Code, err)
			}

			got := encoding.MatchesMachineCode(code)
			if got != test.Want {
				t.Fatalf("%q.MatchesMachineCode(% x): got %v, want %v", test.Encoding, code, got, test.Want)
			}
		})
	}
}

func TestParseEncoding(t *testing.T) {
	tests := []struct {
		Name     string
		Encoding string
		Want     *Encoding
	}{
		{
			Name:     "opcode only",
			Encoding: "37",
			Want: &Encoding{
				Syntax: "37",
				Opcode: []byte{0x37},
			},
		},
		{
			// Make sure this is recorded as
			// an opcode, not an opcode prefix.
			Name:     "fwait",
			Encoding: "9B",
			Want: &Encoding{
				Syntax: "9B",
				Opcode: []byte{0x9b},
			},
		},
		{
			Name:     "always REX",
			Encoding: "REX + 81 /0 id",
			Want: &Encoding{
				Syntax:   "REX + 81 /0 id",
				REX:      true,
				Opcode:   []byte{0x81},
				ModRM:    true,
				ModRMreg: 1,
			},
		},
		{
			Name:     "always REX.W",
			Encoding: "REX.W + 03 /r",
			Want: &Encoding{
				Syntax: "REX.W + 03 /r",
				REX:    true,
				REX_W:  true,
				Opcode: []byte{0x03},
				ModRM:  true,
			},
		},
		{
			Name:     "fixed ModRM mod",
			Encoding: "F3 0F 38 DD 11:rrr:bbb",
			Want: &Encoding{
				Syntax:            "F3 0F 38 DD 11:rrr:bbb",
				MandatoryPrefixes: []Prefix{0xf3},
				Opcode:            []byte{0x0f, 0x38, 0xdd},
				ModRM:             true,
				ModRMmod:          0b11 + 1,
			},
		},
		{
			Name:     "constrained ModRM mod",
			Encoding: "F3 0F 38 DD !(11):rrr:bbb",
			Want: &Encoding{
				Syntax:            "F3 0F 38 DD !(11):rrr:bbb",
				MandatoryPrefixes: []Prefix{0xf3},
				Opcode:            []byte{0x0f, 0x38, 0xdd},
				ModRM:             true,
				ModRMmod:          5,
			},
		},
		{
			Name:     "fixed ModRM reg",
			Encoding: "F3 0F 38 DD 11:101:bbb",
			Want: &Encoding{
				Syntax:            "F3 0F 38 DD 11:101:bbb",
				MandatoryPrefixes: []Prefix{0xf3},
				Opcode:            []byte{0x0f, 0x38, 0xdd},
				ModRM:             true,
				ModRMmod:          0b11 + 1,
				ModRMreg:          0b101 + 1,
			},
		},
		{
			Name:     "fixed ModRM rm",
			Encoding: "F3 0F 38 DD 11:rrr:101",
			Want: &Encoding{
				Syntax:            "F3 0F 38 DD 11:rrr:101",
				MandatoryPrefixes: []Prefix{0xf3},
				Opcode:            []byte{0x0f, 0x38, 0xdd},
				ModRM:             true,
				ModRMmod:          0b11 + 1,
				ModRMrm:           0b101 + 1,
			},
		},
		{
			Name:     "complex prefix",
			Encoding: "NFx 66 0F AE /7",
			Want: &Encoding{
				Syntax:            "NFx 66 0F AE /7",
				NoRepPrefixes:     true,
				MandatoryPrefixes: []Prefix{0x66},
				Opcode:            []byte{0x0f, 0xae},
				ModRM:             true,
				ModRMreg:          7 + 1,
			},
		},
		{
			Name:     "VEX",
			Encoding: "VEX.128.66.0F38.W0 13 /r",
			Want: &Encoding{
				Syntax:    "VEX.128.66.0F38.W0 13 /r",
				VEX:       true,
				VEX_L:     false,
				VEXpp:     0b01,
				VEXm_mmmm: 0b0_0010,
				VEX_W:     false,
				Opcode:    []byte{0x13},
				ModRM:     true,
			},
		},
		{
			Name:     "EVEX",
			Encoding: "EVEX.256.66.0F.W1 58 /r",
			Want: &Encoding{
				Syntax:    "EVEX.256.66.0F.W1 58 /r",
				EVEX:      true,
				VEX_L:     true,
				EVEX_Lp:   false,
				VEXpp:     0b01,
				VEXm_mmmm: 0b0_0001,
				VEX_W:     true,
				Opcode:    []byte{0x58},
				ModRM:     true,
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, err := ParseEncoding(test.Encoding)
			if err != nil {
				t.Fatalf("ParseEncoding(%q): got unexpected error: %v", test.Encoding, err)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Fatalf("ParseEncoding(%q): (-want, +got)\n%s", test.Encoding, diff)
			}
		})
	}
}
