// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// This was inspired by golang.org/x/arch/x86/x86spec,
// but has been updated to support a more recent version
// of the “Intel® 64 and IA-32 Architectures Software Developer's Manual”
// and to emit more structured data.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"rsc.io/pdf"
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

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix(filepath.Base(os.Args[0]) + ": ")
}

func main() {
	var debugDescriptions bool
	var name, debugPages string
	flag.BoolVar(&debugDescriptions, "descriptions", false, "Include instruction descriptions when debugging.")
	flag.StringVar(&name, "f", "", "Path to the Intel manual PDF.")
	flag.StringVar(&debugPages, "debug", "", "List of comma-separated pages numbers to debug.")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n  %s OPTIONS\n\nOptions:\n", filepath.Base(os.Args[0]))
		flag.PrintDefaults()
	}

	flag.Parse()
	if name == "" {
		flag.Usage()
		os.Exit(2)
	}

	if debugDescriptions && len(debugPages) == 0 {
		log.Fatalf("cannot use -descriptions without -debug")
	}

	r, err := openPDF(name)
	if err != nil {
		log.Fatalf("failed to open %q: %v", name, err)
	}

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

			if len(specs) > 0 {
				fmt.Println("\nInstructions:")
				w := tabwriter.NewWriter(os.Stdout, 0, 8, 3, ' ', 0)
				columns[0] = "UID"
				columns[1] = "Syntax"
				columns[2] = "Encoding"
				columns[3] = "Operand size override"
				columns[4] = "Data size"
				n := 5
				fmt.Fprintln(w, strings.Join(columns[:n], "\t"))
				for _, spec := range specs {
					insts, err := spec.Instructions(nil)
					if err != nil {
						log.Fatal(err)
					}

					for _, inst := range insts {
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
				}
				w.Flush()
			}
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
	stats.Start()
	var instructions []*Instruction
	byUID := make(map[string]*Instruction)
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
							stats.ListingError()
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
							inst.Operands[i] = new(Operand)
							*inst.Operands[i] = *op
						}
					}
				}

				byUID[inst.UID] = inst
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
				stats.ExtraInstructionForm()
				instructions = append(instructions, inst)
			}
		}
	}

	sort.Slice(instructions, func(i, j int) bool { return instructions[i].UID < instructions[j].UID })

	print(stats.String())
	for inst := range expected {
		fmt.Printf("WARNING: Failed to find instruction %q.\n", inst)
	}

	fmt.Println()
	json.NewEncoder(os.Stdout).Encode(instructions[10])
}
