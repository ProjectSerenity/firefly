// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Sparse instruction specs into full data structures.

package main

import (
	"strings"

	"firefly-os.dev/tools/ruse/internal/x86"
)

func (s *Spec) Instructions(stats *Stats) ([]*Instruction, error) {
	// First, we work out whether the instruction
	// has any split operands, meaning that we
	// need to return multiple forms of the
	// instruction. We start by populating the
	// operands (just the first when we have an
	// unsplit operand) and then we loop through,
	// copying any unsplit operands if a split
	// operand has been found.
	mnemonic, rest, _ := strings.Cut(s.M.Instruction, " ")
	out := make([]*Instruction, 1, 3)
	out[0] = new(Instruction)

	encoding, err := x86.ParseEncoding(s.M.Opcode)
	if err != nil {
		return nil, Errorf(s.M.Page, "invalid encoding: %v", err)
	}

	tuple, ok := x86.TupleNone, true
	if s.E != nil {
		tuple, ok = x86.TupleTypes[s.E.TupleType]
	}

	if !ok {
		return nil, Errorf(s.M.Page, "invalid tuple type %q", s.E.TupleType)
	}

	var cpuid []string
	if s.M.CPUID != "" {
		cpuid = strings.Split(s.M.CPUID, " ")
	}

	args := strings.Split(rest, ",")
	for i := range args {
		args[i] = strings.TrimSpace(args[i])
	}

	for i, arg := range args {
		if arg == "" {
			if s.E != nil {
				switch s.E.Operands[i] {
				case "", "N/A":
				default:
					return nil, Errorf(s.M.Page, "operand %q has unexpected encoding %q", arg, s.E.Operands[i])
				}
			}

			continue
		}

		// Remove any EVEX suffixes.
		arg, suffixes, _ := strings.Cut(strings.TrimSpace(arg), " ")

		// Determine the encoding.
		out[0].Operands[i] = new(Operand)
		switch s.E.Operands[i] {
		case "None":
			out[0].Operands[i].Encoding = x86.EncodingNone
		case "Implicit":
			out[0].Operands[i].Encoding = x86.EncodingImplicit
		case "VEX.vvvv", "EVEX.vvvv":
			out[0].Operands[i].Encoding = x86.EncodingVEXvvvv
		case "Opcode":
			out[0].Operands[i].Encoding = x86.EncodingRegisterModifier
		case "ST(i)":
			out[0].Operands[i].Encoding = x86.EncodingStackIndex
		case "Offset":
			out[0].Operands[i].Encoding = x86.EncodingCodeOffset
		case "ModRM:reg":
			out[0].Operands[i].Encoding = x86.EncodingModRMreg
		case "ModRM:r/m":
			out[0].Operands[i].Encoding = x86.EncodingModRMrm
		case "SIB", "VSIB":
			out[0].Operands[i].Encoding = x86.EncodingSIB
		case "Displacement":
			out[0].Operands[i].Encoding = x86.EncodingDisplacement
		case "Immediate":
			out[0].Operands[i].Encoding = x86.EncodingImmediate
		case "VEX /is4":
			out[0].Operands[i].Encoding = x86.EncodingVEXis4
		case "", "N/A":
			switch arg {
			case "AL", "CL", "AX", "DX", "EAX", "ECX", "RAX", "CR8", "XMM0",
				"ES", "CS", "SS", "DS", "FS", "GS",
				"ST",
				"0", "1", "2", "3", "4", "5", "6", "7", "8", "9":
				stats.ListingError()
				out[0].Operands[i].Encoding = x86.EncodingNone
			case "<EAX>", "<ECX>", "<EDX>", "<XMM0>":
				stats.ListingError()
				out[0].Operands[i].Encoding = x86.EncodingImplicit
			case "imm8":
				stats.ListingError()
				out[0].Operands[i].Encoding = x86.EncodingImmediate
			default:
				return nil, Errorf(s.M.Page, "operand %q has invalid encoding %q", arg, s.E.Operands[i])
			}
		default:
			return nil, Errorf(s.M.Page, "operand %q has invalid encoding %q", arg, s.E.Operands[i])
		}

		parts := strings.Split(arg, "/")
		if len(parts) == 1 || unsplitOperands[arg] {
			out[0].Operands[i].Name = arg
			out[0].Operands[i].Syntax = arg
			if suffixes != "" {
				out[0].Operands[i].Syntax += " " + suffixes
			}

			continue
		}

		for j, part := range parts {
			if j > 0 {
				out = append(out, new(Instruction))
			}

			if out[j].Operands[i] == nil {
				// Copy the encoding details.
				out[j].Operands[i] = new(Operand)
				*out[j].Operands[i] = *out[0].Operands[i]
			}

			out[j].Operands[i].Name = part
			out[j].Operands[i].Syntax = part
			if suffixes != "" {
				out[j].Operands[i].Syntax += " " + suffixes
			}
		}
	}

	// If we've had a split operand, we
	// need to copy any unsplit operands
	// to the later instruction forms.
	for i, inst := range out {
		// Populate the basics.
		inst.Page = s.M.Page
		inst.Mnemonic = mnemonic
		inst.Syntax = s.M.Instruction
		inst.Encoding = encoding
		inst.TupleType = tuple
		inst.Mode64 = s.M.Mode64 == "Valid"
		inst.Mode32 = s.M.Mode32 == "Valid"
		inst.Mode16 = s.M.Mode16 == "Valid"
		inst.OperandSize = s.M.OperandSize
		inst.AddressSize = s.M.AddressSize
		inst.DataSize = s.M.DataSize
		inst.CPUID = cpuid

		// Copy the operands if
		// necessary.
		if i > 0 {
			for j := range out[0].Operands {
				if inst.Operands[j] == nil && out[0].Operands[j] != nil {
					inst.Operands[j] = new(Operand)
					*inst.Operands[j] = *out[0].Operands[j]
				}
			}
		}

		// Count the number of operands.
		for _, op := range inst.Operands {
			if op == nil {
				break
			}

			inst.MaxArgs++
			if op.Encoding != x86.EncodingImplicit {
				inst.MinArgs++
			}
		}

		// Check/finish the instruction.
		err = inst.fix(stats)
		if err != nil {
			return nil, err
		}
	}

	return out, nil
}
