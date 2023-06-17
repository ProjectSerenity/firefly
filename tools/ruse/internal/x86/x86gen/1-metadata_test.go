// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
	"rsc.io/pdf"
)

func TestParseMetadata(t *testing.T) {
	tests := []struct {
		Name string
		File string
		ID   string
		Date string
	}{
		{
			Name: "Vol2a",
			File: "Volume-2a-Instruction-Set-Reference-A-L",
			ID:   "253666-079US",
			Date: "March 2023",
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			name := filepath.Join("testdata", test.File+".pdf")
			r, err := openPDF(name)
			if err != nil {
				t.Fatalf("failed to open %q: %v", test.Name, err)
			}

			id, date, err := ParseMetadata(r)
			if err != nil {
				t.Fatalf("failed to parse %q: %v", test.Name, err)
			}

			if id != test.ID {
				t.Errorf("incorrect document ID: got %q, want %q", id, test.ID)
			}

			if date != test.Date {
				t.Errorf("incorrect document date: got %q, want %q", date, test.Date)
			}
		})
	}
}

func TestParseOutline(t *testing.T) {
	tests := []struct {
		Name    string
		Outline pdf.Outline
		Want    []string
	}{
		{
			Name: "Vol2a",
			Outline: pdf.Outline{
				Child: []pdf.Outline{
					{
						Title: "Chapter 1 About This Manual",
						Child: []pdf.Outline{
							{Title: "1.1 Intel® 64 and IA-32 Processors Covered in this Manual"},
							{Title: "1.2 Overview of Volume 2A, 2B, 2C, and 2D: Instruction Set Reference"},
							{
								Title: "1.3 Notational Conventions",
								Child: []pdf.Outline{
									{Title: "1.3.1 Bit and Byte Order"},
									{Title: "1.3.2 Reserved Bits and Software Compatibility"},
									{Title: "1.3.3 Instruction Operands"},
									{Title: "1.3.4 Hexadecimal and Binary Numbers"},
									{Title: "1.3.5 Segmented Addressing"},
								},
							},
							{Title: "1.4 Related Literature"},
						},
					},
					{
						Title: "Chapter 2 Instruction Format",
						Child: []pdf.Outline{
							{
								Title: "2.1 Instruction Format for Protected Mode, real-address Mode, and virtual-8086 mode",
								Child: []pdf.Outline{
									{Title: "2.1.1 Instruction Prefixes"},
									{Title: "2.1.2 Opcodes"},
									{Title: "2.1.3 ModR/M and SIB Bytes"},
									{Title: "2.1.4 Displacement and Immediate Bytes"},
									{Title: "2.1.5 Addressing-Mode Encoding of ModR/M and SIB Bytes"},
								},
							},
							{
								Title: "2.2 IA-32e Mode",
								Child: []pdf.Outline{
									{
										Title: "2.2.1 REX Prefixes",
										Child: []pdf.Outline{
											{Title: "2.2.1.1 Encoding"},
											{Title: "2.2.1.2 More on REX Prefix Fields"},
											{Title: "2.2.1.3 Displacement"},
											{Title: "2.2.1.4 Direct Memory-Offset MOVs"},
											{Title: "2.2.1.5 Immediates"},
										},
									},
									{Title: "2.2.2 Additional Encodings for Control and Debug Registers"},
								},
							},
							{
								Title: "2.3 Intel® Advanced Vector Extensions (Intel® AVX)",
								Child: []pdf.Outline{
									{Title: "2.3.1 Instruction Format"},
									{Title: "2.3.2 VEX and the LOCK prefix"},
									{Title: "2.3.3 VEX and the 66H, F2H, and F3H prefixes"},
									{Title: "2.3.4 VEX and the REX prefix"},
									{
										Title: "2.3.5 The VEX Prefix",
										Child: []pdf.Outline{
											{Title: "2.3.5.1 VEX Byte 0, bits[7:0]"},
											{Title: "2.3.5.2 VEX Byte 1, bit [7]-‘R’"},
											{Title: "2.3.5.3 3-byte VEX byte 1, bit[6]-‘X’"},
											{Title: "2.3.5.4 3-byte VEX byte 1, bit[5]-‘B’"},
											{Title: "2.3.5.5 3-byte VEX byte 2, bit[7]-‘W’"},
										},
									},
									{
										Title: "2.3.6 Instruction Operand Encoding and VEX.vvvv, ModR/M",
										Child: []pdf.Outline{
											{Title: "2.3.6.1 3-byte VEX byte 1, bits[4:0]-“m-mmmm”"},
											{Title: "2.3.6.2 2-byte VEX byte 1, bit[2], and 3-byte VEX byte 2, bit [2]-“L”"},
											{Title: "2.3.6.3 2-byte VEX byte 1, bits[1:0], and 3-byte VEX byte 2, bits [1:0]-“pp”"},
										},
									},
									{Title: "2.3.7 The Opcode Byte"},
									{Title: "2.3.8 The ModR/M, SIB, and Displacement Bytes"},
									{Title: "2.3.9 The Third Source Operand (Immediate Byte)"},
									{
										Title: "2.3.10 Intel® AVX Instructions and the Upper 128-bits of YMM registers",
										Child: []pdf.Outline{
											{Title: "2.3.10.1 Vector Length Transition and Programming Considerations"},
										},
									},
									{Title: "2.3.11 Intel® AVX Instruction Length"},
									{
										Title: "2.3.12 Vector SIB (VSIB) Memory Addressing",
										Child: []pdf.Outline{
											{Title: "2.3.12.1 64-bit Mode VSIB Memory Addressing"},
										},
									},
								},
							},
							{Title: "2.4 Intel® Advanced Matrix Extensions (Intel® AMX)"},
							{
								Title: "2.5 Intel® AVX and Intel® SSE Instruction Exception Specification",
								Child: []pdf.Outline{
									{Title: "2.5.1 Exceptions Type 1 (Aligned Memory Reference)"},
									{Title: "2.5.2 Exceptions Type 2 (>=16 Byte Memory Reference, Unaligned)"},
									{Title: "2.5.3 Exceptions Type 3 (<16 Byte Memory Argument)"},
									{Title: "2.5.4 Exceptions Type 4 (>=16 Byte Mem Arg, No Alignment, No Floating-point Exceptions)"},
									{Title: "2.5.5 Exceptions Type 5 (<16 Byte Mem Arg and No FP Exceptions)"},
								},
							},
							{
								Title: "2.6 VEX Encoding Support for GPR Instructions",
								Child: []pdf.Outline{
									{Title: "2.6.1 Exceptions Type 13 (VEX-Encoded GPR Instructions)"},
								},
							},
							{
								Title: "2.7 Intel® AVX-512 Encoding",
								Child: []pdf.Outline{
									{Title: "2.7.1 Instruction Format and EVEX"},
									{Title: "2.7.2 Register Specifier Encoding and EVEX"},
									{Title: "2.7.3 Opmask Register Encoding"},
									{Title: "2.7.4 Masking Support in EVEX"},
									{Title: "2.7.5 Compressed Displacement (disp8*N) Support in EVEX"},
									{Title: "2.7.6 EVEX Encoding of Broadcast/Rounding/SAE Support"},
									{Title: "2.7.7 Embedded Broadcast Support in EVEX"},
									{Title: "2.7.8 Static Rounding Support in EVEX"},
									{Title: "2.7.9 SAE Support in EVEX"},
									{Title: "2.7.10 Vector Length Orthogonality"},
									{
										Title: "2.7.11 #UD Equations for EVEX",
										Child: []pdf.Outline{
											{Title: "2.7.11.1 State Dependent #UD"},
											{Title: "2.7.11.2 Opcode Independent #UD"},
											{Title: "2.7.11.3 Opcode Dependent #UD"},
										},
									},
									{Title: "2.7.12 Device Not Available"},
									{Title: "2.7.13 Scalar Instructions"},
								},
							},
							{
								Title: "2.8 Exception Classifications of EVEX-Encoded instructions",
								Child: []pdf.Outline{
									{Title: "2.8.1 Exceptions Type E1 and E1NF of EVEX-Encoded Instructions"},
									{Title: "2.8.2 Exceptions Type E2 of EVEX-Encoded Instructions"},
									{Title: "2.8.3 Exceptions Type E3 and E3NF of EVEX-Encoded Instructions"},
									{Title: "2.8.4 Exceptions Type E4 and E4NF of EVEX-Encoded Instructions"},
									{Title: "2.8.5 Exceptions Type E5 and E5NF"},
								},
							},
							{Title: "2.9 Exception Classifications of Opmask instructions"},
							{Title: "2.10 Intel® AMX Instruction Exception Classes"},
						},
					},
					{
						Title: "Chapter 3 Instruction Set Reference, A-L",
						Child: []pdf.Outline{
							{
								Title: "3.1 Interpreting the Instruction Reference Pages",
								Child: []pdf.Outline{
									{
										Title: "3.1.1 Instruction Format",
										Child: []pdf.Outline{
											{Title: "3.1.1.1 Opcode Column in the Instruction Summary Table (Instructions without VEX Prefix)"},
											{Title: "3.1.1.2 Opcode Column in the Instruction Summary Table (Instructions with VEX prefix)"},
											{Title: "3.1.1.3 Instruction Column in the Opcode Summary Table"},
											{Title: "3.1.1.4 Operand Encoding Column in the Instruction Summary Table"},
											{Title: "3.1.1.5 64/32-bit Mode Column in the Instruction Summary Table"},
										},
									},
								},
							},
							{
								Title: "3.2 Intel® AMX Considerations",
								Child: []pdf.Outline{
									{Title: "3.2.1 Implementation Parameters"},
									{Title: "3.2.2 Helper Functions"},
								},
							},
							{
								Title: "3.3 Instructions (A-L)",
								Child: []pdf.Outline{
									{Title: "AAA-ASCII Adjust After Addition"},
									{Title: "AAD-ASCII Adjust AX Before Division"},
									{Title: "AAM-ASCII Adjust AX After Multiply"},
									{Title: "AAS-ASCII Adjust AL After Subtraction"},
									{Title: "ADC-Add With Carry"},
								},
							},
						},
					},
					{
						Title: "Chapter 4 Instruction Set Reference, M-U",
						Child: []pdf.Outline{
							{
								Title: "4.1 Imm8 Control Byte Operation for PCMPESTRI / PCMPESTRM / PCMPISTRI / PCMPISTRM",
								Child: []pdf.Outline{
									{Title: "4.1.1 General Description"},
									{Title: "4.1.2 Source Data Format"},
									{Title: "4.1.3 Aggregation Operation"},
									{Title: "4.1.4 Polarity"},
									{Title: "4.1.5 Output Selection"},
								},
							},
							{Title: "4.2 Common Transformation and Primitive Functions for SHA1XXX and SHA256XXX"},
							{
								Title: "4.3 Instructions (M-U)",
								Child: []pdf.Outline{
									{Title: "MASKMOVDQU-Store Selected Bytes of Double Quadword"},
									{Title: "MASKMOVQ-Store Selected Bytes of Quadword"},
									{Title: "MAXPD-Maximum of Packed Double Precision Floating-Point Values"},
									{Title: "MAXPS-Maximum of Packed Single Precision Floating-Point Values"},
									{Title: "MAXSD-Return Maximum Scalar Double Precision Floating-Point Value"},
								},
							},
						},
					},
					{
						Title: "Chapter 5 Instruction Set Reference, V",
						Child: []pdf.Outline{
							{Title: "5.1 Ternary Bit Vector Logic Table"},
							{
								Title: "5.2 Instructions (V)",
								Child: []pdf.Outline{
									{Title: "VADDPH-Add Packed FP16 Values"},
									{Title: "VADDSH-Add Scalar FP16 Values"},
									{Title: "VALIGND/VALIGNQ-Align Doubleword/Quadword Vectors"},
									{Title: "VBLENDMPD/VBLENDMPS-Blend Float64/Float32 Vectors Using an OpMask Control"},
									{Title: "VBROADCAST-Load with Broadcast Floating-Point Data"},
								},
							},
						},
					},
					{
						Title: "Chapter 6 Instruction Set Reference, W-Z",
						Child: []pdf.Outline{
							{
								Title: "6.1 Instructions (W-Z)",
								Child: []pdf.Outline{
									{Title: "WAIT/FWAIT-Wait"},
									{Title: "WBINVD-Write Back and Invalidate Cache"},
									{Title: "WBNOINVD-Write Back and Do Not Invalidate Cache"},
									{Title: "WRFSBASE/WRGSBASE-Write FS/GS Segment Base"},
									{Title: "WRMSR-Write to Model Specific Register"},
								},
							},
						},
					},
					{
						Title: "Chapter 7 Safer Mode Extensions Reference",
						Child: []pdf.Outline{
							{Title: "7.1 Overview"},
							{
								Title: "7.2 SMX Functionality",
								Child: []pdf.Outline{
									{Title: "7.2.1 Detecting and Enabling SMX"},
									{
										Title: "7.2.2 SMX Instruction Summary",
										Child: []pdf.Outline{
											{Title: "7.2.2.1 GETSEC[CAPABILITIES]"},
											{Title: "7.2.2.2 GETSEC[ENTERACCS]"},
											{Title: "7.2.2.3 GETSEC[EXITAC]"},
											{Title: "7.2.2.4 GETSEC[SENTER]"},
											{Title: "7.2.2.5 GETSEC[SEXIT]"},
										},
									},
									{Title: "7.2.3 Measured Environment and SMX"},
								},
							},
							{
								Title: "7.3 GETSEC Leaf Functions",
								Child: []pdf.Outline{
									{Title: "GETSEC[CAPABILITIES]-Report the SMX Capabilities"},
									{Title: "GETSEC[ENTERACCS]-Execute Authenticated Chipset Code"},
									{Title: "GETSEC[EXITAC]-Exit Authenticated Code Execution Mode"},
									{Title: "GETSEC[SENTER]-Enter a Measured Environment"},
									{Title: "GETSEC[SEXIT]-Exit Measured Environment"},
								},
							},
						},
					},
					{
						Title: "Chapter 8 Instruction Set Reference Unique to Intel® Xeon Phi™ Processors",
						Child: []pdf.Outline{
							{Title: "PREFETCHWT1-Prefetch Vector Data Into Caches With Intent to Write and T1 Hint"},
							{Title: "V4FMADDPS/V4FNMADDPS-Packed Single Precision Floating-Point Fused Multiply-Add (4-Iterations)"},
							{Title: "V4FMADDSS/V4FNMADDSS-Scalar Single Precision Floating-Point Fused Multiply-Add (4-Iterations)"},
							{Title: "VEXP2PD-Approximation to the Exponential 2^x of Packed Double Precision Floating-Point Values With Less Than 2^-23 Relative Error"},
							{Title: "VEXP2PS-Approximation to the Exponential 2^x of Packed Single Precision Floating-Point Values With Less Than 2^-23 Relative Error"},
						},
					},
					{
						Title: "Appendix A Opcode Map",
						Child: []pdf.Outline{
							{Title: "A.1 Using Opcode Tables"},
							{
								Title: "A.2 Key to Abbreviations",
								Child: []pdf.Outline{
									{Title: "A.2.1 Codes for Addressing Method"},
									{Title: "A.2.2 Codes for Operand Type"},
									{Title: "A.2.3 Register Codes"},
									{
										Title: "A.2.4 Opcode Look-up Examples for One, Two, and Three-Byte Opcodes",
										Child: []pdf.Outline{
											{Title: "A.2.4.1 One-Byte Opcode Instructions"},
											{Title: "A.2.4.2 Two-Byte Opcode Instructions"},
											{Title: "A.2.4.3 Three-Byte Opcode Instructions"},
											{Title: "A.2.4.4 VEX Prefix Instructions"},
										},
									},
									{Title: "A.2.5 Superscripts Utilized in Opcode Tables"},
								},
							},
							{Title: "A.3 One, Two, and THREE-Byte Opcode Maps"},
							{
								Title: "A.4 Opcode Extensions For One-Byte And Two-byte Opcodes",
								Child: []pdf.Outline{
									{Title: "A.4.1 Opcode Look-up Examples Using Opcode Extensions"},
									{Title: "A.4.2 Opcode Extension Tables"},
								},
							},
							{
								Title: "A.5 Escape Opcode Instructions",
								Child: []pdf.Outline{
									{Title: "A.5.1 Opcode Look-up Examples for Escape Instruction Opcodes"},
									{
										Title: "A.5.2 Escape Opcode Instruction Tables",
										Child: []pdf.Outline{
											{Title: "A.5.2.1 Escape Opcodes with D8 as First Byte"},
											{Title: "A.5.2.2 Escape Opcodes with D9 as First Byte"},
											{Title: "A.5.2.3 Escape Opcodes with DA as First Byte"},
											{Title: "A.5.2.4 Escape Opcodes with DB as First Byte"},
											{Title: "A.5.2.5 Escape Opcodes with DC as First Byte"},
										},
									},
								},
							},
						},
					},
					{
						Title: "Appendix B Instruction Formats and Encodings",
						Child: []pdf.Outline{
							{
								Title: "B.1 Machine Instruction Format",
								Child: []pdf.Outline{
									{Title: "B.1.1 Legacy Prefixes"},
									{Title: "B.1.2 REX Prefixes"},
									{Title: "B.1.3 Opcode Fields"},
									{
										Title: "B.1.4 Special Fields",
										Child: []pdf.Outline{
											{Title: "B.1.4.1 Reg Field (reg) for Non-64-Bit Modes"},
											{Title: "B.1.4.2 Reg Field (reg) for 64-Bit Mode"},
											{Title: "B.1.4.3 Encoding of Operand Size (w) Bit"},
											{Title: "B.1.4.4 Sign-Extend (s) Bit"},
											{Title: "B.1.4.5 Segment Register (sreg) Field"},
										},
									},
									{Title: "B.1.5 Other Notes"},
								},
							},
							{
								Title: "B.2 General-Purpose Instruction Formats and Encodings for Non-64-Bit Modes",
								Child: []pdf.Outline{
									{Title: "B.2.1 General Purpose Instruction Formats and Encodings for 64-Bit Mode"},
								},
							},
							{Title: "B.3 Pentium® Processor Family Instruction Formats and Encodings"},
							{Title: "B.4 64-bit Mode Instruction Encodings for SIMD Instruction Extensions"},
							{
								Title: "B.5 MMX Instruction Formats and Encodings",
								Child: []pdf.Outline{
									{Title: "B.5.1 Granularity Field (gg)"},
									{Title: "B.5.2 MMX Technology and General-Purpose Register Fields (mmxreg and reg)"},
									{Title: "B.5.3 MMX Instruction Formats and Encodings Table"},
								},
							},
							{Title: "B.6 Processor Extended State Instruction Formats and Encodings"},
							{Title: "B.7 P6 Family Instruction Formats and Encodings"},
							{Title: "B.8 SSE Instruction Formats and Encodings"},
							{
								Title: "B.9 SSE2 Instruction Formats and Encodings",
								Child: []pdf.Outline{
									{Title: "B.9.1 Granularity Field (gg)"},
								},
							},
							{Title: "B.10 SSE3 Formats and Encodings Table"},
							{Title: "B.11 SSsE3 Formats and Encoding Table"},
							{Title: "B.12 AESNI and PCLMULQDQ INstruction Formats and Encodings"},
							{Title: "B.13 Special Encodings for 64-Bit Mode"},
							{Title: "B.14 SSE4.1 Formats and Encoding Table"},
							{Title: "B.15 SSE4.2 Formats and Encoding Table"},
							{Title: "B.16 AVX Formats and Encoding Table"},
							{Title: "B.17 Floating-Point Instruction Formats and Encodings"},
							{Title: "B.18 VMX Instructions"},
							{Title: "B.19 SMX Instructions"},
						},
					},
					{
						Title: "Appendix C Intel® C/C++ Compiler Intrinsics and Functional Equivalents",
						Child: []pdf.Outline{
							{Title: "C.1 Simple Intrinsics"},
							{Title: "C.2 Composite Intrinsics"},
						},
					},
				},
			},
			Want: []string{
				"AAA-ASCII Adjust After Addition",
				"AAD-ASCII Adjust AX Before Division",
				"AAM-ASCII Adjust AX After Multiply",
				"AAS-ASCII Adjust AL After Subtraction",
				"ADC-Add With Carry",
				"MASKMOVDQU-Store Selected Bytes of Double Quadword",
				"MASKMOVQ-Store Selected Bytes of Quadword",
				"MAXPD-Maximum of Packed Double Precision Floating-Point Values",
				"MAXPS-Maximum of Packed Single Precision Floating-Point Values",
				"MAXSD-Return Maximum Scalar Double Precision Floating-Point Value",
				"VADDPH-Add Packed FP16 Values",
				"VADDSH-Add Scalar FP16 Values",
				"VALIGND/VALIGNQ-Align Doubleword/Quadword Vectors",
				"VBLENDMPD/VBLENDMPS-Blend Float64/Float32 Vectors Using an OpMask Control",
				"VBROADCAST-Load with Broadcast Floating-Point Data",
				"WAIT/FWAIT-Wait",
				"WBINVD-Write Back and Invalidate Cache",
				"WBNOINVD-Write Back and Do Not Invalidate Cache",
				"WRFSBASE/WRGSBASE-Write FS/GS Segment Base",
				"WRMSR-Write to Model Specific Register",
				"PREFETCHWT1-Prefetch Vector Data Into Caches With Intent to Write and T1 Hint",
				"V4FMADDPS/V4FNMADDPS-Packed Single Precision Floating-Point Fused Multiply-Add (4-Iterations)",
				"V4FMADDSS/V4FNMADDSS-Scalar Single Precision Floating-Point Fused Multiply-Add (4-Iterations)",
				"VEXP2PD-Approximation to the Exponential 2^x of Packed Double Precision Floating-Point Values With Less Than 2^-23 Relative Error",
				"VEXP2PS-Approximation to the Exponential 2^x of Packed Single Precision Floating-Point Values With Less Than 2^-23 Relative Error",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			got := ParseOutline(test.Outline)
			if diff := cmp.Diff(test.Want, got); diff != "" {
				t.Fatalf("ParseOutline(%q): (-want, +got)\n%s", test.Name, diff)
			}
		})
	}
}
