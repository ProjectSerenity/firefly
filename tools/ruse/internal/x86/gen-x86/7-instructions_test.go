// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"firefly-os.dev/tools/ruse/internal/x86"
)

func TestSpec_Instructions(t *testing.T) {
	tests := []struct {
		Name string
		Spec *Spec
		Want []*x86.Instruction
	}{
		{
			Name: "ADC",
			Spec: &Spec{
				M: &Mnemonic{
					Page:            137,
					Opcode:          "80 /2 ib",
					Instruction:     "ADC r8/m8, imm8",
					OperandEncoding: "MI",
					Mode64:          "Valid",
					Mode32:          "Valid",
					Mode16:          "Valid",
				},
				E: &OperandEncoding{
					Page:     137,
					Encoding: "MI",
					Operands: [4]string{"ModRM:r/m", "Immediate", "N/A", "N/A"},
				},
			},
			Want: []*x86.Instruction{
				{
					Page:     137,
					Mnemonic: "ADC",
					UID:      "ADC_Rmr8_Imm8",
					Syntax:   "ADC r8/m8, imm8",
					Encoding: &x86.Encoding{
						Syntax:   "80 /2 ib",
						Opcode:   []byte{0x80},
						ModRM:    true,
						ModRMreg: 3,
					},
					TupleType: x86.TupleNone,
					DataSize:  8,
					MinArgs:   2,
					MaxArgs:   2,
					Operands: [4]*x86.Operand{
						{
							Name:      "r8",
							Syntax:    "r8",
							UID:       "Rmr8",
							Type:      x86.TypeRegister,
							Encoding:  x86.EncodingModRMrm,
							Bits:      8,
							Registers: x86.Registers8bitGeneralPurpose,
						},
						{
							Name:     "imm8",
							Syntax:   "imm8",
							UID:      "Imm8",
							Type:     x86.TypeSignedImmediate,
							Encoding: x86.EncodingImmediate,
							Bits:     8,
						},
					},
					Mode16: true,
					Mode32: true,
					Mode64: true,
				},
				{
					Page:     137,
					Mnemonic: "ADC",
					UID:      "ADC_M8_Imm8",
					Syntax:   "ADC r8/m8, imm8",
					Encoding: &x86.Encoding{
						Syntax:   "80 /2 ib",
						Opcode:   []byte{0x80},
						ModRM:    true,
						ModRMreg: 3,
					},
					TupleType: x86.TupleNone,
					DataSize:  8,
					MinArgs:   2,
					MaxArgs:   2,
					Operands: [4]*x86.Operand{
						{
							Name:     "m8",
							Syntax:   "m8",
							UID:      "M8",
							Type:     x86.TypeMemory,
							Encoding: x86.EncodingModRMrm,
							Bits:     8,
						},
						{
							Name:     "imm8",
							Syntax:   "imm8",
							UID:      "Imm8",
							Type:     x86.TypeSignedImmediate,
							Encoding: x86.EncodingImmediate,
							Bits:     8,
						},
					},
					Mode16: true,
					Mode32: true,
					Mode64: true,
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got, err := test.Spec.Instructions(nil)
			if err != nil {
				t.Fatalf("spec.Instructions(): got %v", err)
			}

			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Fatalf("spec.Instructions(): (-want, +got)\n%s", diff)
			}
		})
	}
}
