// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package x86 prints debugging information about the Ruse
// toolchain's understanding of the x86 instructino set.
package x86

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"firefly-os.dev/tools/ruse/internal/x86"
	"firefly-os.dev/tools/ruse/ssafir"
)

var program = filepath.Base(os.Args[0])

// Main prints information about a given instruction
// mnemonic.
func Main(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("x86", flag.ExitOnError)

	var help bool
	flags.BoolVar(&help, "h", false, "Show this message and exit.")

	flags.Usage = func() {
		log.Printf("Usage:\n  %s %s [OPTIONS] MNEMONIC...\n\n", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(2)
	}

	err := flags.Parse(args)
	if err != nil || help {
		flags.Usage()
	}

	mnemonics := flags.Args()

	var buf bytes.Buffer
	for i, mnemonic := range mnemonics {
		if i > 0 {
			// Add a spacer.
			fmt.Fprintln(w)
		}

		// See whether it's a register first.
		if reg := x86.RegistersByName[mnemonic]; reg != nil {
			fmt.Fprintf(&buf, "%s: &Register{\n", mnemonic)
			fmt.Fprintf(&buf, "	Name: %q,\n", reg.Name)
			fmt.Fprintf(&buf, "	Type: %q,\n", reg.Type)
			if reg.Bits != 0 {
				fmt.Fprintf(&buf, "	Bits: %d,\n", reg.Bits)
			}
			if reg.Reg != 0 {
				fmt.Fprintf(&buf, "	Reg:  %#04b,\n", reg.Reg)
			}
			if reg.Addr != 0 {
				fmt.Fprintf(&buf, "	Addr: %#04b,\n", reg.Addr)
			}
			if reg.MinMode != 0 {
				fmt.Fprintf(&buf, "	Mode: %d,\n", reg.MinMode)
			}
			if reg.EVEX {
				fmt.Fprintf(&buf, "	EVEX: %v,\n", reg.EVEX)
			}
			if len(reg.Aliases) > 0 {
				fmt.Fprintf(&buf, "	Aliases: [\n")
				for _, alias := range reg.Aliases {
					fmt.Fprintf(&buf, "		%q,\n", alias)
				}
				fmt.Fprintf(&buf, "	],\n")
			}
			fmt.Fprintf(&buf, "}\n")
			continue
		}

		candidates, ok := x86MnemonicToOps[mnemonic]
		if !ok {
			fmt.Fprintf(w, "%s: no instruction data found\n", mnemonic)
			continue
		}

		fmt.Fprintf(&buf, "%s: []*Instruction{\n", mnemonic)
		for _, op := range candidates {
			inst := x86OpToInstruction(op)
			if inst == nil {
				return fmt.Errorf("internal error: found no instruction data for op %s", op)
			}

			fmt.Fprintf(&buf, "	{\n")
			fmt.Fprintf(&buf, "		Page:      %d,\n", inst.Page)
			fmt.Fprintf(&buf, "		Mnemonic:  %q,\n", inst.Mnemonic)
			fmt.Fprintf(&buf, "		UID:       %q,\n", inst.UID)
			fmt.Fprintf(&buf, "		Syntax:    %q,\n", inst.Syntax)

			enc := inst.Encoding
			fmt.Fprintf(&buf, "		Encoding: {\n")
			fmt.Fprintf(&buf, "			Syntax:        %q,\n", enc.Syntax)
			if len(enc.PrefixOpcodes) > 0 {
				fmt.Fprintf(&buf, "			PrefixOpcodes: [\n")
				for _, op := range enc.PrefixOpcodes {
					fmt.Fprintf(&buf, "				%#02x,\n", op)
				}
				fmt.Fprintf(&buf, "			],\n")
			}
			if enc.NoVEXPrefixes {
				fmt.Fprintf(&buf, "			NoVEX:         %v,\n", enc.NoVEXPrefixes)
			}
			if enc.NoRepPrefixes {
				fmt.Fprintf(&buf, "			NoRep:         %v,\n", enc.NoRepPrefixes)
			}
			if len(enc.MandatoryPrefixes) > 0 {
				fmt.Fprintf(&buf, "			Prefixes: [\n")
				for _, prefix := range enc.MandatoryPrefixes {
					fmt.Fprintf(&buf, "				%q,\n", prefix)
				}
				fmt.Fprintf(&buf, "			],\n")
			}
			switch {
			case enc.REX_R:
				fmt.Fprintf(&buf, "			REX.R:         %v,\n", enc.REX_R)
			case enc.REX_W:
				fmt.Fprintf(&buf, "			REX.W:         %v,\n", enc.REX_W)
			case enc.REX:
				fmt.Fprintf(&buf, "			REX:           %v,\n", enc.REX)
			}
			if enc.VEX {
				fmt.Fprintf(&buf, "			VEX:           %v,\n", enc.VEX)
				fmt.Fprintf(&buf, "			VEX.L:         %b,\n", b2i(enc.VEX_L))
				fmt.Fprintf(&buf, "			VEX.pp:        %02b,\n", enc.VEXpp)
				fmt.Fprintf(&buf, "			VEX.m_mmmm:    %05b,\n", enc.VEXm_mmmm)
				fmt.Fprintf(&buf, "			VEX.W:         %b,\n", b2i(enc.VEX_W))
				if enc.VEX_WIG {
					fmt.Fprintf(&buf, "			VEX.WIG:       %v,\n", enc.VEX_WIG)
				}
				if enc.VEXis4 {
					fmt.Fprintf(&buf, "			VEX/is4:       %v,\n", enc.VEXis4)
				}
			}
			if enc.EVEX {
				fmt.Fprintf(&buf, "			EVEX:          %v,\n", enc.EVEX)
				fmt.Fprintf(&buf, "			EVEX.L:        %b,\n", b2i(enc.EVEX_Lp))
				if enc.Mask {
					fmt.Fprintf(&buf, "			EVEX.opmask:   %v,\n", enc.Mask)
				}
				if enc.Zero {
					fmt.Fprintf(&buf, "			EVEX.zero:     %v,\n", enc.Zero)
				}
				if enc.Rounding {
					fmt.Fprintf(&buf, "			EVEX.round:    %v,\n", enc.Rounding)
				}
				if enc.Suppress {
					fmt.Fprintf(&buf, "			EVEX.suppress: %v,\n", enc.Suppress)
				}
			}
			fmt.Fprintf(&buf, "			Opcode:        [")
			for i, op := range enc.Opcode {
				if i > 0 {
					fmt.Fprint(&buf, ", ")
				}
				fmt.Fprintf(&buf, "%#02x", op)
			}
			fmt.Fprintf(&buf, "],\n")
			if enc.RegisterModifier != 0 {
				fmt.Fprintf(&buf, "			RegModifier:   %d,\n", enc.RegisterModifier-1)
			}
			if enc.StackIndex != 0 {
				fmt.Fprintf(&buf, "			StackIndex:    %d,\n", enc.StackIndex-1)
			}
			if enc.CodeOffset {
				fmt.Fprintf(&buf, "			CodeOffset:    %v,\n", enc.CodeOffset)
			}
			if enc.ModRM {
				fmt.Fprintf(&buf, "			ModR/M:        %v,\n", enc.ModRM)
			}
			if enc.ModRMmod == 5 {
				fmt.Fprintf(&buf, "			ModR/M.mod:    !0b11,\n")
			} else if enc.ModRMmod != 0 {
				fmt.Fprintf(&buf, "			ModR/M.mod:    %02b,\n", enc.ModRMmod-1)
			}
			if enc.ModRMreg != 0 {
				fmt.Fprintf(&buf, "			ModR/M.reg:    %03b,\n", enc.ModRMreg-1)
			}
			if enc.ModRMrm != 0 {
				fmt.Fprintf(&buf, "			ModR/M.r/m:    %03b,\n", enc.ModRMrm-1)
			}
			if enc.SIB {
				fmt.Fprintf(&buf, "			SIB: vvv       %v,\n", enc.SIB)
			}
			if len(enc.ImpliedImmediate) > 0 {
				fmt.Fprintf(&buf, "			Immediate:    %#x,\n", enc.ImpliedImmediate)
			}
			fmt.Fprintf(&buf, "		},\n")

			if inst.TupleType != 0 {
				fmt.Fprintf(&buf, "		TupleType: %q,\n", inst.TupleType)
			}
			if inst.MaxArgs == 0 {
				fmt.Fprintf(&buf, "		MaxArgs:   %d,\n", inst.MaxArgs)
			} else {
				fmt.Fprintf(&buf, "		MinArgs:   %d,\n", inst.MinArgs)
				fmt.Fprintf(&buf, "		MaxArgs:   %d,\n", inst.MaxArgs)
				fmt.Fprintf(&buf, "		Opoerands: [\n")
				for _, op := range inst.Operands {
					if op == nil {
						break
					}

					fmt.Fprintf(&buf, "			{\n")
					fmt.Fprintf(&buf, "				Name:     %q,\n", op.Name)
					fmt.Fprintf(&buf, "				Syntax:   %q,\n", op.Syntax)
					fmt.Fprintf(&buf, "				UID:      %q,\n", op.UID)
					fmt.Fprintf(&buf, "				Type:     %q,\n", op.Type)
					fmt.Fprintf(&buf, "				Encoding: %q,\n", op.Encoding)
					if op.Bits != 0 {
						fmt.Fprintf(&buf, "				Bits:     %d,\n", op.Bits)
					}
					fmt.Fprintf(&buf, "			},\n")
				}
				fmt.Fprintf(&buf, "		],\n")
			}
			if inst.Mode16 {
				fmt.Fprintf(&buf, "		Mode16:    %v,\n", inst.Mode16)
			}
			if inst.Mode32 {
				fmt.Fprintf(&buf, "		Mode32:    %v,\n", inst.Mode32)
			}
			if inst.Mode64 {
				fmt.Fprintf(&buf, "		Mode64:    %v,\n", inst.Mode64)
			}
			if len(inst.CPUID) > 0 {
				fmt.Fprintf(&buf, "	CPUID: [\n")
				for _, flag := range inst.CPUID {
					fmt.Fprintf(&buf, "		%q,\n", flag)
				}
				fmt.Fprintf(&buf, "	],\n")
			}
			if inst.OperandSize {
				fmt.Fprintf(&buf, "		Operand:   %v,\n", inst.OperandSize)
			}
			if inst.AddressSize {
				fmt.Fprintf(&buf, "		Address:   %v,\n", inst.AddressSize)
			}
			if inst.DataSize != 0 {
				fmt.Fprintf(&buf, "		Data:      %d,\n", inst.DataSize)
			}
			fmt.Fprintf(&buf, "	},\n")
		}
		fmt.Fprintf(&buf, "}\n")
	}

	_, err = w.Write(buf.Bytes())
	return err
}

// x86OpToInstruction maps a `ssafir.Op` to
// an `*x86.Instruction`. If the op is not
// an x86 instruction, `nil` is returned.
func x86OpToInstruction(op ssafir.Op) *x86.Instruction {
	i := int(op - firstX86Op)
	if 0 <= i && i < len(x86.Instructions) {
		return x86.Instructions[i]
	}

	return nil
}

func b2i(b bool) int {
	if b {
		return 1
	}

	return 0
}
