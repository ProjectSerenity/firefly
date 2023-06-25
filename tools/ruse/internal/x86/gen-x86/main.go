// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// This was inspired by golang.org/x/arch/x86/x86spec,
// but has been updated to support a more recent version
// of the “Intel® 64 and IA-32 Architectures Software Developer's Manual”
// and to emit more structured data.

// Command gen-x86 parses the Intel x86 manual to generate structured
// data on the x86 instruction set.
package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"text/template"

	"rsc.io/pdf"

	"firefly-os.dev/tools/ruse/internal/x86"
)

func openPDF(name string) (*pdf.Reader, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}

	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	return pdf.NewReader(newCachedReaderAt(f), fi.Size())
}

var (
	program   = filepath.Base(os.Args[0])
	data_go   = filepath.Join("tools", "ruse", "internal", "x86", "data.go")
	x86_json  = filepath.Join("tools", "ruse", "internal", "x86", "x86.json")
	x86manual = filepath.Join("bazel-firefly-os.dev", "external", "x86manual", "file", "downloaded")
)

func init() {
	log.SetFlags(log.Llongfile)
	log.SetOutput(os.Stderr)
	log.SetPrefix(program + ": ")
}

//go:embed templates/*.tmpl
var templatesFS embed.FS

var templates = template.Must(template.New("").Funcs(map[string]any{
	"registers": func(operand *x86.Operand) string {
		if len(operand.Registers) == 0 {
			return ""
		}

		return registersNameByOperandUID[operand.UID]
	},
}).ParseFS(templatesFS, "templates/*.tmpl"))

func main() {
	var debugDescriptions, verbose bool
	var name, debugPages, findMnemonics string
	flag.BoolVar(&debugDescriptions, "descriptions", false, "Include instruction descriptions when debugging.")
	flag.StringVar(&name, "f", "", "Path to the Intel manual PDF.")
	flag.StringVar(&debugPages, "debug", "", "List of comma-separated pages numbers to debug.")
	flag.StringVar(&findMnemonics, "find", "", "List of comma-separated mnemonics to debug.")
	flag.BoolVar(&verbose, "v", false, "Print each error individually.")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n  %s OPTIONS\n\nOptions:\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}

	flag.Parse()

	// If no file has been specified, we
	// default to finding manual in Bazel.
	if name == "" {
		name = x86manual
		if workspace := os.Getenv("BUILD_WORKSPACE_DIRECTORY"); workspace != "" {
			name = filepath.Join(workspace, name)
		}
	}

	// Firstly, change to the workspace root
	// directory so we can write the
	// data.go and x86.json data files into
	// their destination.
	if workspace := os.Getenv("BUILD_WORKSPACE_DIRECTORY"); workspace != "" {
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatalf("failed to determine current directory: %v", err)
		}

		defer os.Chdir(cwd) // Move back to where we started once we're finished.

		err = os.Chdir(workspace)
		if err != nil {
			log.Fatalf("failed to change directory to %q: %v", workspace, err)
		}
	}

	if debugDescriptions && len(debugPages) == 0 {
		log.Fatalf("cannot use -descriptions without -debug")
	}

	r, err := openPDF(name)
	if err != nil {
		log.Fatalf("failed to open %q: %v", name, err)
	}

	var instructions []*x86.Instruction
	if debugPages != "" {
		pages := strings.Split(debugPages, ",")
		numbers := make([]int, len(pages))
		columns := make([]string, 7)
		for i, page := range pages {
			page = strings.TrimSpace(page)
			num, err := strconv.Atoi(page)
			if err != nil {
				log.Fatalf("invalid debug pages %q: invalid page number %q: %v", debugPages, page, err)
			}

			numbers[i] = num
		}

		for i, page := range numbers {
			if i > 0 {
				fmt.Println()
			}

			listing, _, err := ParsePage(r, page, nil, true)
			if err != nil {
				log.Fatal(err)
			}

			if listing == nil {
				fmt.Printf("No listing found on page %d.\n", page)
				continue
			}

			// Debug the listing.
			fmt.Printf("Instruction: %s\n", listing.Name)
			if len(listing.MnemonicTable) == 0 {
				fmt.Println("No mnemonic table found.")
			} else {
				fmt.Println("Mnemonics:")
				w := tabwriter.NewWriter(os.Stdout, 0, 8, 3, ' ', 0)
				columns[0] = "Opcode"
				columns[1] = "Instruction"
				columns[2] = "Operand Encoding"
				columns[3] = "64-Bit Mode"
				columns[4] = "32-Bit Mode"
				columns[5] = "CPUID"
				columns[6] = "Description"
				n := 7
				if !debugDescriptions {
					n--
				}

				fmt.Fprintln(w, strings.Join(columns[:n], "\t"))
				for _, m := range listing.MnemonicTable {
					columns[0] = m.Opcode
					columns[1] = m.Instruction
					columns[2] = m.OperandEncoding
					columns[3] = m.Mode64
					columns[4] = m.Mode32
					columns[5] = m.CPUID
					columns[6] = m.Description
					fmt.Fprintln(w, strings.Join(columns[:n], "\t"))
				}
				w.Flush()
			}

			if len(listing.OperandEncodingTable) == 0 {
				fmt.Println("\nNo operand encoding table found.")
			} else {
				fmt.Println("\nOperand Encoding:")
				w := tabwriter.NewWriter(os.Stdout, 0, 8, 3, ' ', 0)
				columns[0] = "Op/En"
				columns[1] = "Tuple Type"
				columns[2] = "Operand 1"
				columns[3] = "Operand 2"
				columns[4] = "Operand 3"
				columns[5] = "Operand 4"
				n := 6
				fmt.Fprintln(w, strings.Join(columns[:n], "\t"))
				for _, e := range listing.OperandEncodingTable {
					columns[0] = e.Encoding
					columns[1] = e.TupleType
					columns[2] = e.Operands[0]
					columns[3] = e.Operands[1]
					columns[4] = e.Operands[2]
					columns[5] = e.Operands[3]
					fmt.Fprintln(w, strings.Join(columns[:n], "\t"))
				}
				w.Flush()
			}

			// Check the specs.
			specs, err := listing.Specs(nil)
			if err != nil {
				log.Fatal(err)
			}

			if len(specs) == 0 {
				continue
			}

			instructions = instructions[:0]

			// Start with the extra instructions.
			for _, extra := range Extras {
				specs, err := extra.Specs(nil)
				if err != nil {
					log.Fatal(err)
				}

				for _, spec := range specs {
					insts, err := spec.Instructions(nil)
					if err != nil {
						log.Fatal(err)
					}

					for _, inst := range insts {
						instructions = append(instructions, inst)
					}
				}
			}

			for _, spec := range specs {
				insts, err := spec.Instructions(nil)
				if err != nil {
					log.Fatal(err)
				}

				instructions = append(instructions, insts...)
			}

			autodetectVariableOperandSizes(instructions)

			fmt.Println("\nInstructions:")
			w := tabwriter.NewWriter(os.Stdout, 0, 8, 3, ' ', 0)
			columns[0] = "UID"
			columns[1] = "Syntax"
			columns[2] = "Encoding"
			columns[3] = "Operand size override"
			columns[4] = "Data size"
			n := 5
			fmt.Fprintln(w, strings.Join(columns[:n], "\t"))
			for _, inst := range instructions {
				if inst.Page == 0 {
					continue
				}

				columns[0] = inst.UID
				columns[1] = inst.Syntax
				columns[2] = inst.Encoding.Syntax
				columns[3] = ""
				columns[4] = ""
				if inst.OperandSize {
					columns[3] = "true"
				}
				if inst.DataSize != 0 {
					columns[4] = strconv.Itoa(inst.DataSize)
				}

				fmt.Fprintln(w, strings.Join(columns[:n], "\t"))
			}

			w.Flush()
		}

		return
	}

	id, date, err := ParseMetadata(r)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Intel x86 manual %s (%s).\n", id, date)

	outline := ParseOutline(r.Outline())
	fmt.Printf("Found %s instruction listings in the outline.\n", humaniseNumber(len(outline)))
	expected := make(map[string]bool)
	for _, inst := range outline {
		expected[respace(inst)] = true
	}

	var stats Stats
	stats.PrintErrors = verbose
	stats.Start()
	byUID := make(map[string]*x86.Instruction)
	byMnemonic := make(map[string][]*x86.Instruction)
	numPages := r.NumPage()
	for pageNum := 1; pageNum <= numPages; {
		var listing *Listing
		listing, pageNum, err = ParsePage(r, pageNum, &stats, false)
		if err == ErrAllIgnored {
			delete(expected, listing.Name)
			continue
		}

		if err != nil {
			println(stats.String())
			log.Fatal(err)
		}

		if listing == nil {
			continue
		}

		// Check that we're expecting this
		// instruction from the outline.
		if !expected[listing.Name] {
			fmt.Printf("WARNING: Found unexpected instruction %q.\n", listing.Name)
			continue
		}

		delete(expected, listing.Name)

		specs, err := listing.Specs(&stats)
		if err != nil {
			println(stats.String())
			log.Fatal(err)
		}

		for _, spec := range specs {
			insts, err := spec.Instructions(&stats)
			if err != nil {
				println(stats.String())
				log.Fatal(err)
			}

			for _, inst := range insts {
				if other := byUID[inst.UID]; other != nil {
					switch inst.UID {
					case "LEAVE":
						if !inst.Mode64 || !inst.Mode32 {
							// There are three versions
							// of LEAVE but their syntax
							// and encoding is identical,
							// so we just keep the version
							// with full compatibility.
							continue
						}
					case "JZ_Rel16", "JZ_Rel32":
						if inst.Page != other.Page {
							// This instruction is repeated in
							// the manual. We just ignore the
							// second version.
							stats.ListingError("p.%d: Instruction %q is repeated on p.%d", other.Page, other.Syntax, inst.Page)
							continue
						}
					case "VMOVQ_XMM1_M64_VEX", "VMOVQ_XMM1_M64_EVEX128",
						"VMOVQ_M64_XMM1_VEX", "VMOVQ_M64_XMM1_EVEX128":
						// There are two versions of these
						// instructions but each pair has
						// the same meaning.
						//
						// We keep whichever version is
						// supported in compatibility mode.
						if !other.Mode32 {
							// Replace the old version.
							*other = *inst
							continue
						}

						if !inst.Mode32 {
							// Keep the old verison.
							continue
						}
					case "SMSW_M16":
						// This instruction has two forms,
						// but their behaviour is the same,
						// so we just keep the first one
						// we encounter.
						continue
					}

					println(stats.String())
					log.Fatal(Errorf(inst.Page, "found two instructions with the UID %q:\n%#v\n%#v", inst.UID, other, inst))
				}

				// Add any additional operands.
				extras, ok := ExtraOperands[inst.UID]
				if ok {
					for i, op := range extras {
						if op != nil {
							if i < inst.MinArgs {
								log.Fatalf("ExtraOperands[%q]: installing operand %d (< inst.Mandatory %d)", inst.UID, i, inst.MinArgs)
							} else if i < inst.MaxArgs {
								log.Fatalf("ExtraOperands[%q]: installing operand %d (< inst.Implied %d)", inst.UID, i, inst.MaxArgs)
							}

							inst.MaxArgs = i + 1
							inst.Operands[i] = new(x86.Operand)
							*inst.Operands[i] = *op
						}
					}
				}

				byUID[inst.UID] = inst
				byMnemonic[inst.Mnemonic] = append(byMnemonic[inst.Mnemonic], inst)
				stats.InstructionForm()
				instructions = append(instructions, inst)
			}
		}
	}

	// Add the extra instructions.
	for _, extra := range Extras {
		specs, err := extra.Specs(&stats)
		if err != nil {
			println(stats.String())
			log.Fatal(err)
		}

		for _, spec := range specs {
			stats.ExtraInstruction()
			insts, err := spec.Instructions(&stats)
			if err != nil {
				println(stats.String())
				log.Fatal(err)
			}

			for _, inst := range insts {
				if other := byUID[inst.UID]; other != nil {
					println(stats.String())
					log.Fatal(Errorf(inst.Page, "found two instructions with the UID %q:\n%#v\n%#v", inst.UID, other, inst))
				}

				byUID[inst.UID] = inst
				byMnemonic[inst.Mnemonic] = append(byMnemonic[inst.Mnemonic], inst)
				stats.ExtraInstructionForm()
				instructions = append(instructions, inst)
			}
		}
	}

	sort.Slice(instructions, func(i, j int) bool { return instructions[i].UID < instructions[j].UID })

	autodetectVariableOperandSizes(instructions)

	print(stats.String())
	for inst := range expected {
		fmt.Printf("WARNING: Failed to find instruction %q.\n", inst)
	}

	if findMnemonics != "" {
		mnemonics := strings.Split(findMnemonics, ",")
		for i := range mnemonics {
			mnemonics[i] = strings.TrimSpace(mnemonics[i])
		}

		sort.Strings(mnemonics)

		for _, mnemonic := range mnemonics {
			fmt.Println()
			insts, ok := byMnemonic[mnemonic]
			if !ok {
				fmt.Fprintf(os.Stderr, "found no instructions with mnemonic %q\n", mnemonic)
				continue
			}

			fmt.Printf("Mnemonic: %s\n", mnemonic)
			columns := make([]string, 11)
			w := tabwriter.NewWriter(os.Stdout, 0, 8, 3, ' ', 0)
			headers := [][11]string{
				{"", "", "", "Opcode", "64-Bit", "32-Bit", "16-Bit", "CPUID", "Operand", "Address", "Data"},
				{"Page", "UID", "Syntax", "Encoding", "Mode", "Mode", "Mode", "Flags", "Size", "Size", "Size"},
				{"----", "---", "------", "--------", "------", "------", "------", "-----", "-------", "-------", "----"},
			}

			for _, header := range headers {
				copy(columns, header[:])
				fmt.Fprintln(w, strings.Join(columns, "\t"))
			}

			for _, inst := range insts {
				// Debug the instruction.
				columns[0] = ""
				if inst.Page != 0 {
					columns[0] = strconv.Itoa(inst.Page)
				}
				columns[1] = inst.UID
				columns[2] = inst.Syntax
				columns[3] = inst.Encoding.Syntax
				columns[4] = strconv.FormatBool(inst.Mode64)
				columns[5] = strconv.FormatBool(inst.Mode32)
				columns[6] = strconv.FormatBool(inst.Mode16)
				columns[7] = strings.Join(inst.CPUID, ", ")
				columns[8] = ""
				if inst.OperandSize {
					columns[8] = "true"
				}
				columns[9] = ""
				if inst.AddressSize {
					columns[9] = "true"
				}
				columns[10] = ""
				if inst.DataSize != 0 {
					columns[10] = strconv.Itoa(inst.DataSize)
				}
				fmt.Fprintln(w, strings.Join(columns, "\t"))
			}
			w.Flush()
		}

		return
	}

	// Prepare the generated Go code
	// and write it out to the desired
	// path.

	var data struct {
		Command      string
		Instructions []*x86.Instruction
	}

	data.Command = "//tools/ruse/internal/x86/x86gen"
	data.Instructions = instructions

	var b bytes.Buffer
	err = templates.ExecuteTemplate(&b, "data.go.tmpl", data)
	if err != nil {
		log.Fatalf("failed to write instructions to %q: %v", data_go, err)
	}

	fail := false
	formatted, err := format.Source(b.Bytes())
	if err != nil {
		fail = true
		log.Println(err)
		formatted = b.Bytes()
	}

	// Write to data.go in the repository requested.
	err = os.WriteFile(data_go, formatted, 0644)
	if err != nil {
		log.Fatalf("failed to write %s: %v", data_go, err)
	}

	// Write out the JSON data.
	f, err := os.Create(x86_json)
	if err != nil {
		log.Fatalf("failed to create %q: %v", x86_json, err)
	}

	defer f.Close()

	jw := json.NewEncoder(f)
	jw.SetEscapeHTML(false)
	for i, inst := range instructions {
		// Encode each instruction on a
		// separate line.
		err = jw.Encode(inst)
		if err != nil {
			log.Fatalf("failed to write instruction %d to %q: %v", i, x86_json, err)
		}
	}

	if fail {
		os.Exit(1)
	}
}

// autodetectVariableOperandSizes takes a
// set of instructions and identifies any
// that should have a variable operand
// size. This is typically where multiple
// instructions have the same encoding.
// The exception is the conditional jump
// instructions, which alias one another.
func autodetectVariableOperandSizes(insts []*x86.Instruction) {
	// Group instructions by encoding.
	opcodes := make(map[string][]*x86.Instruction)
	for _, inst := range insts {
		// Some instructions should be
		// excluded as they don't have
		// variable operand sizes, but
		// appear that they might.
		switch inst.Mnemonic {
		// Don't include `MOVSD xmm1, xmm2/m64`
		// or `MOVSS xmm1, xmm2/m32`, which are
		// split in two for some reason.
		case "MOVSS", "MOVSD":
			// Don't affect the MOVS variant.
			if inst.MinArgs > 0 {
				continue
			}
		// These vendor-specific instructions
		// clash with one another.
		case "SENDUIPI", "VMXON":
			continue
		}

		switch inst.UID {
		// The operand size override
		// prefix is unhelpful here,
		// as the 16-bit and 32-bit
		// versions have the same
		// effect. It is allowed to
		// use the operand size
		// override prefix in 32-bit
		// mode for the register
		// forms, but we drop it
		// in 16-bit mode.
		case "MOV_M16_Sreg", "MOV_Sreg_M16",
			"MOV_Rmr32_Sreg", "MOV_Sreg_Rmr32":
			continue
		// These clash with other
		// instructions.
		case "POP_DS", "POP_ES", "POP_SS",
			"PUSH_CS", "PUSH_DS", "PUSH_ES", "PUSH_SS":
			inst.DataSize = 0
			continue
		// These can vary in operand
		// size, but not based on
		// the operand size.
		case "POP16_FS", "POP32_FS", "POP64_FS",
			"POP16_GS", "POP32_GS", "POP64_GS",
			"PUSH_FS", "PUSH_GS":
			inst.DataSize = 0
			continue
		// The memory form is the
		// same in any mode.
		case "SLDT_M16", "SMSW_M16", "STR_M16":
			inst.OperandSize = false
			continue
		// This is variable in size,
		// but we shouldn't use REX.W
		// for the 64-bit form, as the
		// 32-bit form isn't supported
		// in 64-bit mode.
		case "POP_Rmr64", "POP_M64", "POP_R64op",
			"PUSH_Rmr64", "PUSH_M64", "PUSH_R64op":
			inst.OperandSize = false
			continue
		}

		opcode := inst.Encoding.Syntax
		if strings.Contains(opcode, "VEX") {
			continue
		}

		// We also ignore any code offset
		// suffix, since this is influenced
		// by the operand size override
		// prefix.
		opcode = strings.TrimSuffix(opcode, " cw")
		opcode = strings.TrimSuffix(opcode, " cd")

		// Likewise with immediate sizes.
		opcode = strings.TrimSuffix(opcode, " iw")
		opcode = strings.TrimSuffix(opcode, " id")

		// Likewise with opcode registers.
		opcode = strings.TrimSuffix(opcode, "+rw")
		opcode = strings.TrimSuffix(opcode, "+rd")
		opcodes[opcode] = append(opcodes[opcode], inst)
	}

	// Now find the groups and check
	// their mode compatibilities
	// overlap.
	for _, group := range opcodes {
		if len(group) < 2 {
			continue
		}

		// Turn the mode compatibilities
		// into a bitmap.
		const (
			mode64 = 1 << iota
			mode32
			mode16
		)

		modes := make([]uint8, len(group))
		for i, inst := range group {
			var mode uint8
			if inst.Mode64 {
				mode |= mode64
			}
			if inst.Mode32 {
				mode |= mode32
			}
			if inst.Mode16 {
				mode |= mode16
			}

			modes[i] = mode
		}

		for i := range modes {
			for j := i + 1; j < len(modes); j++ {
				// Don't include different forms
				// of the same instruction.
				if group[i].Syntax == group[j].Syntax {
					continue
				}

				// Don't include aliased jump
				// instructions like JZ and JE.
				if group[i].Mnemonic != group[j].Mnemonic && group[i].Mnemonic[0] == 'J' {
					continue
				}

				if modes[i]&modes[j] != 0 {
					group[i].OperandSize = true
					group[j].OperandSize = true

					// Also bind their pages
					// together, if we've added
					// an extra instruction.
					if group[i].Page == 0 {
						group[i].Page = group[j].Page
					}
					if group[j].Page == 0 {
						group[j].Page = group[i].Page
					}
				}
			}
		}
	}

	for _, inst := range insts {
		// See whether we can use the
		// operand data to determine
		// the instruction data size.
		if inst.DataSize == 0 && (inst.OperandSize || inst.Encoding.REX) && inst.Operands[0] != nil {
			inst.DataSize = inst.Operands[0].Bits
		}

		// Remove unwanted data sizes if
		// necessary.
		switch inst.Mnemonic {
		case "INCSSPQ":
			inst.DataSize = 0
		}
	}
}
