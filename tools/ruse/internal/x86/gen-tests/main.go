// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command gen-tests generates test vectors
// for x86 instructions, including the Intel
// and Ruse assembly code and the expected
// machine code, plus other metadata.
//
// Uses our x86.json data (see ../x86json)
// to generate varied test cases for each
// instruction, then uses clang and objdump
// to determine the expected machine code
// bytes.
//
// This is performed using the following
// process:
//
// 1. We use x86.json to deterministically generate a large variety of assembly.
// 2. We prune some examples that an assembler would never generate in that form. For example, `adc ecx, 1` should always be assembled as `ADC r/m32, imm8`, not `ADC r/m32, imm32` so should not be selected for the latter.
// 3. We skip some examples by calculating their expected machine code directly. This is useful for instructions that are hard to generate otherwise but are simple to encode, such as odd-sized `pop` instructions.
// 4. The generated instructions are batched up and assembled using Clang. If an error is encountered, the batch is enumerated to isolate the error.
// 5. The assembled code is disassembled using objdump. If an error is encountered, the batch is enumerated to isolate the error.
// 6. The disassembly is parsed, using heuristics to pair up with disassembly with the original assembly. The printed machine code alongside the disassembly is used to populate each test case.
// 7. We perform various checks on each test case to gain confidence that an error has not produced an incorrect test case.
// 8. The test cases are written out to the requested file path.
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"encoding/csv"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"firefly-os.dev/tools/ruse/internal/x86"
)

var program = filepath.Base(os.Args[0])

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix(program + ": ")
}

func main() {
	var help bool
	var workers int
	flag.BoolVar(&help, "h", false, "Show this message and exit.")
	flag.IntVar(&workers, "workers", runtime.NumCPU(), "How many concurrent workers should build tests.")

	flag.Usage = func() {
		log.Printf("Usage:\n  %s [OPTIONS]\n\n", program)
		flag.PrintDefaults()
		os.Exit(2)
	}

	flag.Parse()
	if help || workers < 1 {
		flag.Usage()
	}

	err := genTests(workers)
	if err != nil {
		log.Fatal(err)
	}
}

const TestBatchSize = 2048

type TestEntry struct {
	Inst  *x86.Instruction // Selected instruction form.
	Mode  x86.Mode         // CPU mode.
	Ruse  string           // Ruse assembly.
	Intel string           // Intel assembly.
	Code  string           // Hex-encoded machine code.
}

func (e *TestEntry) IntelArgs() []string {
	i := strings.IndexByte(e.Intel, ' ')
	if i < 0 {
		return nil
	}

	args := strings.Split(e.Intel[i+1:], ",")
	for i := range args {
		args[i] = strings.TrimSpace(args[i])
	}

	return args
}

func genTests(workers int) error {
	start := time.Now()

	// Start by changing to the workspace
	// root directory so we can find the
	// x86.csv data file and write the
	// tests into their destination.
	if workspace := os.Getenv("BUILD_WORKSPACE_DIRECTORY"); workspace != "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to determine current directory: %v", err)
		}

		defer os.Chdir(cwd) // Move back to where we started once we're finished.

		err = os.Chdir(workspace)
		if err != nil {
			return fmt.Errorf("failed to change directory to %q: %v", workspace, err)
		}
	}

	insts := x86.Instructions
	entries, err := GenerateTestEntries(insts)
	if err != nil {
		return fmt.Errorf("failed to generate test entries: %v", err)
	}

	type TestBatch struct {
		Mode     x86.Mode
		Entries  []*TestEntry
		BatchNum int
	}

	// Determine the expected machine code for
	// each entry, using clang and objdump.
	doBatch := func(batch TestBatch) error {
		var b bytes.Buffer
		err = runTests(&b, batch.Mode, batch.Entries)
		if err != nil {
			// If we've had a specific instruction
			// mismatch, we return that immediately.
			var mis *InstructionMismatchError
			if errors.As(err, &mis) {
				return fmt.Errorf("failed to run %d-bit test batch %d: %v", batch.Mode.Int, batch.BatchNum, mis)
			}

			// Otherwise, we run each entry in the
			// batch separately to help narrow down
			// which one caused the error.
			println("Failed during batch", batch.BatchNum, "of", batch.Mode.String+"-bit test generation. Finding individual error...")
			single := make([]*TestEntry, 1)
			for _, entry := range batch.Entries {
				single[0] = entry
				e := runTests(&b, batch.Mode, single)
				if e != nil {
					return fmt.Errorf("failed to run %d-bit test batch %d: %v", batch.Mode.Int, batch.BatchNum, e)
				}
			}

			return fmt.Errorf("internal error while running %d-bit test batch %d: no error while running individually, original error: %v", batch.Mode.Int, batch.BatchNum, err)
		}

		return nil
	}

	var wg sync.WaitGroup
	results := make(chan error, workers)
	batches := make(chan TestBatch, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(wg *sync.WaitGroup, results chan<- error, batches <-chan TestBatch) {
			defer wg.Done()
			for batch := range batches {
				err := doBatch(batch)
				if err != nil {
					results <- err
					return
				}
			}

			results <- nil
		}(&wg, results, batches)
	}

	go func() {
		defer func() {
			close(batches)
			wg.Wait()
			close(results)
		}()

		for _, mode := range x86.Modes {
			batch := TestBatch{
				Mode:     mode,
				Entries:  make([]*TestEntry, 0, TestBatchSize),
				BatchNum: 1,
			}

			for _, entry := range entries {
				if entry.Mode.Int != mode.Int {
					continue
				}

				if entry.Code != "" {
					// Wherever possible, we want to insert the code
					// into the binary so that we can use objdump to
					// check our code. In some cases, where objdump
					// doesn't behave, we skip.
					switch entry.Inst.Syntax {
					case "FWAIT", // Objdump assumes this is a prefix for the next instruction.
						// Objdump always prints the mnemonic without the suffix.
						"POP ES", "POPD ES",
						"POP CS", "POPD CS",
						"POP SS", "POPD SS",
						"POP DS", "POPD DS",
						"POP FS", "POPD FS", "POPQ FS",
						"POP GS", "POPD GS", "POPQ GS",
						"POPA", "POPAD",
						"POPF", "POPFD", "POPFQ",
						"PUSH ES", "PUSHD ES",
						"PUSH CS", "PUSHD CS",
						"PUSH SS", "PUSHD SS",
						"PUSH DS", "PUSHD DS",
						"PUSH FS", "PUSHD FS", "PUSHQ FS",
						"PUSH GS", "PUSHD GS", "PUSHQ GS",
						"PUSHA", "PUSHAD",
						"PUSHF", "PUSHFD", "PUSHFQ":
						x86TestsDone.Add(1)
						continue
					}
				}

				if len(batch.Entries) == cap(batch.Entries) {
					batches <- batch
					batch.BatchNum++
					batch.Entries = make([]*TestEntry, 0, TestBatchSize)
				}

				batch.Entries = append(batch.Entries, entry)
			}

			if len(batch.Entries) > 0 {
				batches <- batch
			}
		}
	}()

	// Wait for everything to finish.
	for err := range results {
		if err != nil {
			return err
		}
	}

	// We should now have machine code
	// for every entry.
	for _, entry := range entries {
		if entry.Code == "" {
			return fmt.Errorf("internal error: failed to generate machine code for:\n\t%s|%s|%s|%s|%s|%s", entry.Inst.Mnemonic, entry.Mode.String, entry.Ruse, entry.Intel, entry.Inst.Syntax, entry.Inst.Encoding.Syntax)
		}
	}

	// We do some post-processing for cases
	// where two instruction forms overlap
	// and become ambiguous. For example,
	// "adc al, cl" matches both "ADC r/m8, r8"
	// and "ADC r8, r/m8". Clang will always
	// select one or the other, so we find
	// cases where the wrong form has been
	// chosen and correct the code.
	//
	// This has to happen before we sort
	// the entries, as we'll change the code,
	// which is one of the search indices.
	for _, entry := range entries {
		FixupEntry(entry)

		// Next, we must check that the machine
		// code matches the instruction encoding.
		unhexed, err := hex.DecodeString(entry.Code)
		if err != nil {
			return fmt.Errorf("got invalid machine code %q: %v", entry.Code, err)
		}

		if result := entry.Inst.Encoding.MatchesMachineCode(unhexed); result != x86.Match {
			err := &InstructionMismatchError{
				Got:      fmt.Sprintf("% x (%s)", unhexed, result),
				Want:     entry.Intel,
				Mnemonic: entry.Inst.Mnemonic,
				UID:      entry.Inst.UID,
				Syntax:   entry.Inst.Syntax,
				Encoding: entry.Inst.Encoding.Syntax,
			}

			return fmt.Errorf("failed to match assembly with disassembly for %d-bit instruction: %w", entry.Mode.Int, err)
		}
	}

	// Sort the entries.
	sort.Slice(entries, func(i, j int) bool {
		ei := entries[i]
		ej := entries[j]
		if ei.Inst.Mnemonic != ej.Inst.Mnemonic {
			return ei.Inst.Mnemonic < ej.Inst.Mnemonic
		}

		if ei.Mode.Int != ej.Mode.Int {
			return ei.Mode.Int < ej.Mode.Int
		}

		return ei.Code < ej.Code
	})

	// Check that there are no cases where
	// different instructions produced the
	// same machine code sequence, as that
	// suggests that at least one test has
	// the wrong code.
	var b bytes.Buffer
	numDuplicates := 0
	for _, mode := range x86.Modes {
		codeToEntry := make(map[string]*TestEntry)
		duplicates := make(map[string][]*TestEntry)
		for _, entry := range entries {
			if entry.Mode.Int != mode.Int {
				continue
			}

			first, ok := codeToEntry[entry.Code]
			if !ok {
				codeToEntry[entry.Code] = entry
				continue
			}

			if AreEquivalent(first, entry) {
				continue
			}

			dups, ok := duplicates[entry.Code]
			if !ok {
				dups = make([]*TestEntry, 1, 2)
				dups[0] = first
			}

			duplicates[entry.Code] = append(dups, entry)
		}

		if len(duplicates) == 0 {
			continue
		}

		codes := make([]string, 0, len(duplicates))
		for code := range duplicates {
			codes = append(codes, code)
		}

		sort.Strings(codes)

		for _, code := range codes {
			numDuplicates++
			if numDuplicates > 10 {
				continue
			}

			entries := duplicates[code]
			if b.Len() > 0 {
				b.WriteByte('\n')
			}

			fmt.Fprintf(&b, "%d-bit mode: %s:\n", mode.Int, code)
			for i, entry := range entries {
				fmt.Fprintf(&b, "  %d: Intel:    %s\n", i+1, entry.Intel)
			}
			for i, entry := range entries {
				fmt.Fprintf(&b, "  %d: syntax:   %s\n", i+1, entry.Inst.Syntax)
			}
			for i, entry := range entries {
				fmt.Fprintf(&b, "  %d: encoding: %s\n", i+1, entry.Inst.Encoding.Syntax)
			}
			for i, entry := range entries {
				fmt.Fprintf(&b, "  %d: mnemonic: %s\n", i+1, entry.Inst.Mnemonic)
			}
		}
	}

	if numDuplicates > 0 {
		return fmt.Errorf("%s ambiguous machine code sequences:\n%s", humaniseNumber(numDuplicates), b.String())
	}

	name := filepath.Join("tools", "ruse", "compiler", "testdata", "x86-tests.csv.gz")
	f, err := os.Create(name)
	if err != nil {
		return fmt.Errorf("failed to create %q: %v", name, err)
	}

	defer f.Close()
	w := gzip.NewWriter(f)

	b.Reset()
	fmt.Fprintf(&b, "# Code generated by %s. DO NOT EDIT.\n", program)
	b.WriteString("#\n")
	b.WriteString("# Copyright 2023 The Firefly Authors.\n")
	b.WriteString("#\n")
	b.WriteString("# Use of this source code is governed by a BSD 3-clause\n")
	b.WriteString("# license that can be found in the LICENSE file.\n")
	_, err = w.Write(b.Bytes())
	if err != nil {
		return fmt.Errorf("failed to write comment to %q: %v", name, err)
	}

	cw := csv.NewWriter(w)
	err = cw.Write([]string{"uid", "mode", "code", "ruse", "intel"})
	if err != nil {
		return fmt.Errorf("failed to write header to %q: %v", name, err)
	}

	for _, entry := range entries {
		err = cw.Write([]string{entry.Inst.UID, entry.Mode.String, entry.Code, entry.Ruse, entry.Intel})
		if err != nil {
			return fmt.Errorf("failed to write instruction %q to %q: %v", entry.Inst.Syntax, name, err)
		}
	}

	cw.Flush()
	err = cw.Error()
	if err != nil {
		return fmt.Errorf("failed to flush %q: %v", name, err)
	}

	err = w.Flush()
	if err != nil {
		return fmt.Errorf("failed to flush %q: gzip error %v", name, err)
	}

	err = w.Close()
	if err != nil {
		return fmt.Errorf("failed to close %q: gzip error %v", name, err)
	}

	err = f.Close()
	if err != nil {
		return fmt.Errorf("failed to close %q: %v", name, err)
	}

	numDone := humaniseNumber(len(entries))
	println("Generated", numDone, "tests for x86 in", time.Since(start).Round(time.Second).String()+".")

	return nil
}

func humaniseNumber(v int) string {
	prefix, suffix := strconv.Itoa(v), ""
	for len(prefix) > 3 {
		suffix = "," + prefix[len(prefix)-3:] + suffix
		prefix = prefix[:len(prefix)-3]
	}

	return prefix + suffix
}

type InstructionMismatchError struct {
	Done     int64
	File     string
	Line     string
	Got      string
	Want     string
	Mnemonic string
	UID      string
	Syntax   string
	Encoding string
}

func (e *InstructionMismatchError) Error() string {
	parts := make([]string, 0, 8)
	if e.Done != 0 {
		parts = append(parts, fmt.Sprintf("Done:      %d", e.Done))
	}
	if e.File != "" {
		parts = append(parts, fmt.Sprintf("File:      %s", e.File))
	}
	if e.Line != "" {
		parts = append(parts, fmt.Sprintf("Line:      %s", e.Line))
	}
	parts = append(parts, fmt.Sprintf("Got:       %s", e.Got))
	parts = append(parts, fmt.Sprintf("Want:      %s", e.Want))
	parts = append(parts, fmt.Sprintf("Mnemonic:  %s", e.Mnemonic))
	parts = append(parts, fmt.Sprintf("UID:       %s", e.UID))
	parts = append(parts, fmt.Sprintf("Syntax:    %s", e.Syntax))
	parts = append(parts, fmt.Sprintf("Encoding:  %s", e.Encoding))

	return fmt.Sprintf("instruction mismatch:\n\t%s", strings.Join(parts, "\n\t"))
}

var x86TestsDone atomic.Int64

func runTests(b *bytes.Buffer, mode x86.Mode, entries []*TestEntry) error {
	b.Reset()
	b.WriteString(".global _start\n")
	//b.WriteString(".intel_mnemonic\n")
	//b.WriteString(".intel_syntax noprefix\n")
	b.WriteString(".text\n")
	b.WriteByte('\n')
	b.WriteString("_start:\n")
	for _, entry := range entries {
		b.WriteByte('\t')
		asm := entry.Intel
		if strings.HasPrefix(asm, "bound ") {
			// Clang expects the size hint to match
			// the register size, not the size of
			// two registers, whereas objdump gives
			// the other. To account for that, we
			// swap down here but still expect the
			// double value later.
			asm = strings.Replace(asm, "dword", "word", 1)
			asm = strings.Replace(asm, "qword", "dword", 1)
		}

		if entry.Code == "" {
			b.WriteString(asm)
		} else {
			for i := 0; i < len(entry.Code); i += 2 {
				if i == 0 {
					b.WriteByte(' ')
				}

				fmt.Fprintf(b, ".byte 0x%s;", entry.Code[i:i+2])
			}
		}

		b.WriteByte('\n')
	}

	intermediate, err := os.CreateTemp("", "x86.*.s")
	if err != nil {
		return fmt.Errorf("failed to create temporary assembly file: %v", err)
	}

	tmpName := intermediate.Name()
	_, err = intermediate.Write(b.Bytes())
	if err != nil {
		intermediate.Close()
		return fmt.Errorf("failed to write temporary assembly file: %v", err)
	}

	err = intermediate.Close()
	if err != nil {
		return fmt.Errorf("failed to close temporary assembly file: %v", err)
	}

	// Assemble the test cases.
	b.Reset()
	cmd := exec.Command("clang", "-masm=intel", "-m"+mode.String, "-nostdlib", "-nodefaultlibs", "-static", "-o", tmpName+".exe", tmpName)
	cmd.Stderr = b
	err = cmd.Run()
	if err != nil {
		if len(entries) == 1 {
			e := &InstructionMismatchError{
				Done:     x86TestsDone.Load(),
				File:     tmpName,
				Got:      b.String(),
				Want:     entries[0].Intel,
				Mnemonic: entries[0].Inst.Mnemonic,
				UID:      entries[0].Inst.UID,
				Syntax:   entries[0].Inst.Syntax,
				Encoding: entries[0].Inst.Encoding.Syntax,
			}

			return fmt.Errorf("failed to assemble %d-bit test cases in %s: %v\n%v", mode.Int, tmpName, err, e)
		}

		return fmt.Errorf("failed to assemble %d-bit test cases in %s: %v\n%s", mode.Int, tmpName, err, b.String())
	}

	// Parse the disassembly.
	b.Reset()
	cmd = exec.Command("objdump", "-Mintel,addr"+mode.String+",data"+mode.String, "--wide", "-d", tmpName+".exe")
	cmd.Stdout = b
	cmd.Stderr = b
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to disassemble %d-bit test cases: %v\n%s", mode.Int, err, b.String())
	}

	// Write the disassembly to disk
	// for debugging purposes.
	err = os.WriteFile(tmpName+".disasm", b.Bytes(), 0644)
	if err != nil {
		return fmt.Errorf("failed to write disassembly for %d-bit test cases: %v", mode.Int, err)
	}

	s := bufio.NewScanner(b)
	for s.Scan() {
		// Skip the prelude.
		line := strings.TrimSpace(s.Text())
		if !strings.HasSuffix(line, "<_start>:") {
			continue
		}

		for i, entry := range entries {
			if !s.Scan() {
				return fmt.Errorf("ran out of disassembly while looking for %s (err: %v)", entry.Intel, s.Err())
			}

			line := strings.TrimSpace(s.Text())

			// Lines look like this:
			//
			// 	  2011a8:       48 09 c8                or     rax,rcx
			// 	  402ffd:       77 80                	ja     402f7f <_start+0x1e93>

			// Remove the address.
			addr, rest, ok := strings.Cut(line, ":")
			if !ok {
				return fmt.Errorf("failed to remove address from line:\n\t%s\n\t%s", line, tmpName+".disasm")
			}

			rest = strings.TrimSpace(rest)
			code := spacedHexRegex.FindString(rest)
			if code == "" {
				return fmt.Errorf("failed to find machine code in line:\n\t%s\n\t%s", line, tmpName+".disasm")
			}

			disasm := strings.TrimSpace(strings.TrimPrefix(rest, code))

			// Remove the whitespace from
			// the hex, swap to lower case.
			code = strings.Map(alphanumRunesOnlyAndLower, code)

			// Ensure there's at least one
			// space after every comma.
			disasm = strings.ReplaceAll(disasm, ",", ", ")
			disasm = strings.ToLower(disasm)

			// Remove any size prefixes, as
			// we won't have included them.
			disasm = strings.TrimPrefix(disasm, "addr16 ")
			disasm = strings.TrimPrefix(disasm, "addr32 ")
			disasm = strings.TrimPrefix(disasm, "data16 ")
			disasm = strings.TrimPrefix(disasm, "data32 ")

			// Remove any address names
			// objdump has derived.
			disasm, _, _ = strings.Cut(disasm, " <_start")

			// Instructions taking a relative address
			// need some special parsing, as objdump
			// prints their destination address (as
			// seen above), rather than the offset.
			if len(entry.Inst.Parameters) == 1 && entry.Inst.Parameters[0].Type == x86.TypeRelativeAddress {
				// Parse the addresses.
				fields := strings.Fields(disasm)
				base, err1 := strconv.ParseUint(addr, 16, 64)
				dest, err2 := strconv.ParseUint(fields[len(fields)-1], 16, 64)
				if err1 != nil || err2 != nil {
					err := &InstructionMismatchError{
						Done:     x86TestsDone.Load() + int64(i),
						File:     tmpName + ".disasm",
						Line:     line,
						Got:      fmt.Sprintf("bad addresses %q and %q: %v, %v", addr, fields[len(fields)-1], err1, err2),
						Want:     entry.Intel,
						Mnemonic: entry.Inst.Mnemonic,
						UID:      entry.Inst.UID,
						Syntax:   entry.Inst.Syntax,
						Encoding: entry.Inst.Encoding.Syntax,
					}

					return fmt.Errorf("failed to parse disassembly for %d-bit instruction: %w", mode.Int, err)
				}

				// Calculate the difference and
				// use it to replace the target
				// in the disassembly.adecimal
				// text).
				diff := int64(dest) - int64(base)
				if diff < -0x200000 {
					// For some reason, objdump
					// sometimes gets relative
					// addresses wrong by 0x400000.
					diff += 0x400000
				}

				fields[len(fields)-1] = fmt.Sprintf("%#x", diff)
				disasm = strings.Join(fields, " ")
			}

			if !IsDisassemblyMatch(entry, disasm, code) {
				err := &InstructionMismatchError{
					Done:     x86TestsDone.Load() + int64(i),
					File:     tmpName + ".disasm",
					Line:     line,
					Got:      disasm,
					Want:     entry.Intel,
					Mnemonic: entry.Inst.Mnemonic,
					UID:      entry.Inst.UID,
					Syntax:   entry.Inst.Syntax,
					Encoding: entry.Inst.Encoding.Syntax,
				}

				return fmt.Errorf("failed to match assembly with disassembly for %d-bit instruction: %w", mode.Int, err)
			}

			if entry.Code == "" {
				// We're all good!
				entry.Code = code
			} else if entry.Code != code {
				err := &InstructionMismatchError{
					Done:     x86TestsDone.Load() + int64(i),
					File:     tmpName + ".disasm",
					Line:     line,
					Got:      disasm,
					Want:     entry.Code,
					Mnemonic: entry.Inst.Mnemonic,
					UID:      entry.Inst.UID,
					Syntax:   entry.Inst.Syntax,
					Encoding: entry.Inst.Encoding.Syntax,
				}

				return fmt.Errorf("failed to match machine code with disassembly for %d-bit instruction: %w", mode.Int, err)
			}
		}

		// All done.
		x86TestsDone.Add(int64(len(entries)))
		b.Reset()
		return nil
	}

	if err := s.Err(); err != nil {
		return fmt.Errorf("failed to read disassembly: %v\n\t%s", err, tmpName+".disasm")
	}

	return fmt.Errorf("returned unexpectedly without reaching disassembly")
}

var spacedHexRegex = regexp.MustCompile(`([a-f0-9][a-f0-9] )+`)

func alphanumRunesOnlyAndLower(r rune) rune {
	switch {
	case 'a' <= r && r <= 'z':
	case 'A' <= r && r <= 'Z':
		return 'a' + (r - 'A') // To lower case.
	case '0' <= r && r <= '9':
	default:
		return -1
	}

	return r
}

func noComma(s string) string {
	return strings.TrimSuffix(s, ",")
}
