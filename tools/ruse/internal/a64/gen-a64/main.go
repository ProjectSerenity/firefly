// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command gen-a64 parses the Arm A64 Instruction
// Set manual to generate structured data on the
// A64 instruction set.
package main

import (
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

var (
	program  = filepath.Base(os.Args[0])
	data_go  = filepath.Join("tools", "ruse", "internal", "a64", "data.go")
	a64_json = filepath.Join("tools", "ruse", "internal", "a64", "a64.json")
)

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix(program + ": ")
}

var failed int

func fail(format string, v ...any) {
	failed++
	if failed <= 10 {
		log.Printf(format, v...)
	}
}

func main() {
	var debug string
	flag.StringVar(&debug, "debug", "", "Print debug information about the instructions in the given file.")
	flag.Parse()

	if debug != "" {
		f, err := os.Open(debug)
		if err != nil {
			log.Fatalf("failed to open %s: %v", debug, err)
		}

		defer f.Close()

		group, err := ParseInstruction(f, true)
		if err != nil {
			log.Fatalf("failed to parse instruction in %s: %v", debug, err)
		}

		_ = group
		return
	}

	// First, we iterate through the A64 instruction
	// set architecture files, building up a set of
	// instruction specs.
	numFailed := 0
	var insts []*Instruction
	dir := filepath.Join("external", "a64manual")
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(path, ".xml") {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			log.Fatalf("failed to open %s: %v", path, err)
		}

		group, err := ParseInstruction(f, false)
		if err != nil {
			f.Close()
			//log.Fatalf("failed to parse instruction in %s: %v", path, err)
			numFailed++
			return nil
		}

		err = f.Close()
		if err != nil {
			log.Fatalf("failed to close %s: %v", path, err)
		}

		insts = append(insts, group...)

		return nil
	})

	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Found %d instructions.", len(insts))
	if numFailed > 0 {
		log.Fatalf("Failed to parse %d instructions.", numFailed)
	}
}

type XMLInstruction struct {
	XMLName xml.Name               `xml:"instructionsection"`
	ID      string                 `xml:"id,attr"`
	Type    string                 `xml:"type,attr"`
	Classes []*XMLInstructionClass `xml:"classes>iclass"`
}

type XMLInstructionClass struct {
	XMLName  xml.Name     `xml:"iclass"`
	Name     string       `xml:"name,attr"`
	Encoding *XMLEncoding `xml:"regdiagram"`
	Forms    []*XMLForm   `xml:"encoding"`
}

type XMLEncoding struct {
	Boxes []*XMLBox `xml:"box"`
}

type XMLBox struct {
	XMLName  xml.Name `xml:"box"`
	Bit      uint8    `xml:"hibit,attr"`
	Name     string   `xml:"name,attr"`
	Width    uint8    `xml:"width,attr"`
	Settings uint8    `xml:"settings,attr"`
	Bits     []string `xml:"c"`
}

type XMLForm struct {
	Name     string         `xml:"name,attr"`
	BitDiffs string         `xml:"bitdiffs,attr"`
	Boxes    []*XMLBox      `xml:"box"`
	Assembly XMLAsmTemplate `xml:"asmtemplate"`
}

type XMLAsmTemplate struct {
	Text string `xml:",innerxml"`
}

type Instruction struct {
	Family   string
	Mnemonic string
	Form     string
	Encoding *Encoding
}

func ParseInstruction(r io.Reader, debug bool) ([]*Instruction, error) {
	debugf := func(format string, v ...any) {
		if debug {
			fmt.Fprintf(os.Stderr, format, v...)
		}
	}

	dec := xml.NewDecoder(r)
	var xInst XMLInstruction
	err := dec.Decode(&xInst)
	if err != nil {
		var xmlErr xml.UnmarshalError
		if errors.As(err, &xmlErr) && strings.HasPrefix(string(xmlErr), "expected element type <instructionsection> but have <") {
			return nil, nil
		}

		return nil, err
	}

	if xInst.Type != "instruction" {
		return nil, nil
	}

	var insts []*Instruction
	debugf("\nID: %q\n", xInst.ID)
	for i, class := range xInst.Classes {
		debugf("Class %d:\n", i+1)
		debugf("  Name: %q\n", class.Name)
		debugf("  Encoding:\n")
		for _, b := range class.Encoding.Boxes {
			if b.Name != "" && (len(b.Bits) == 0 || b.Bits[0] == "") {
				if b.Width <= 1 {
					debugf("    %2d:    %s\n", b.Bit, b.Name)
				} else {
					debugf("    %2d-%2d: %s (%d)\n", b.Bit, b.Bit-b.Width+1, b.Name, b.Width)
				}
			} else if b.Name != "" {
				if b.Width <= 1 {
					debugf("    %2d:    %s %s\n", b.Bit, b.Name, b.Bits[0])
				} else {
					debugf("    %2d-%2d: %s %s (%d)\n", b.Bit, b.Bit-b.Width+1, b.Name, strings.Join(b.Bits, " "), b.Width)
				}
			} else {
				if b.Width <= 1 {
					debugf("    %2d:    %s\n", b.Bit, b.Bits[0])
				} else {
					debugf("    %2d-%2d: %s (%d)\n", b.Bit, b.Bit-b.Width+1, strings.Join(b.Bits, " "), b.Width)
				}
			}
		}

		enc, err := parseEncoding(class.Encoding)
		if err != nil {
			return nil, fmt.Errorf("failed to parse class %d (%s): %v", i, class.Name, err)
		}

		for j, form := range class.Forms {
			debugf("  Form %d:\n", j+1)
			debugf("    Name: %q\n", form.Name)
			debugf("    Bit diffs: %q\n", form.BitDiffs)
			for _, b := range form.Boxes {
				if b.Name != "" && (len(b.Bits) == 0 || b.Bits[0] == "") {
					if b.Width <= 1 {
						debugf("    %2d:    %s\n", b.Bit, b.Name)
					} else {
						debugf("    %2d-%2d: %s (%d)\n", b.Bit, b.Bit-b.Width+1, b.Name, b.Width)
					}
				} else if b.Name != "" {
					if b.Width <= 1 {
						debugf("    %2d:    %s %s\n", b.Bit, b.Name, b.Bits[0])
					} else {
						debugf("    %2d-%2d: %s %s (%d)\n", b.Bit, b.Bit-b.Width+1, b.Name, strings.Join(b.Bits, " "), b.Width)
					}
				} else {
					if b.Width <= 1 {
						debugf("    %2d:    %s\n", b.Bit, b.Bits[0])
					} else {
						debugf("    %2d-%2d: %s (%d)\n", b.Bit, b.Bit-b.Width+1, strings.Join(b.Bits, " "), b.Width)
					}
				}
			}
			debugf("    Asm template: %s\n", form.Assembly.Text)

			mnemonic, args, err := parseTemplate(form.Assembly.Text)
			if err != nil {
				return nil, fmt.Errorf("failed to parse assembly template in class %d (%s) form %d (%s): %v", i, class.Name, j, form.Name, err)
			}

			encoding, err := enc.ResolveForm(form.BitDiffs, args)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve class %d (%s) form %d (%s): %v", i, class.Name, j, form.Name, err)
			}

			inst := &Instruction{
				Family:   xInst.ID,
				Mnemonic: strings.ToLower(mnemonic),
				Form:     form.Name,
				Encoding: encoding,
			}

			insts = append(insts, inst)
		}
	}

	return insts, nil
}

type Encoding struct {
	Bits      BitPattern
	Variables []Variable // Used while parsing XML data. Not emitted in the final data.
	Args      []Arg      // Arguments to the instruction.
}

func (e *Encoding) ResolveForm(diffs string, args []Arg) (*Encoding, error) {
	// Start by parsing the bit diffs.
	valsByName := make(map[string]string)
	parts := strings.Split(diffs, "&&")
	for i, part := range parts {
		part = strings.TrimSpace(part)
		halves := strings.Split(part, "==")
		if len(halves) != 2 {
			fail("malformed variable name %q: got %d parts", part, len(halves))
			return nil, fmt.Errorf("invalid bit diffs %q: variable %d (%q) is malformed", diffs, i, part)
		}

		name := strings.TrimSpace(halves[0])
		value := strings.TrimSpace(halves[1])
		if _, ok := valsByName[name]; ok {
			return nil, fmt.Errorf("invalid bit diffs %q: multiple variables with name %q", diffs, name)
		}

		// Check that there is a
		// corresponding variable.
		ok := false
		for _, v := range e.Variables {
			if v.Name == name {
				ok = true
				break
			}
		}

		if !ok {
			return nil, fmt.Errorf("invalid bit diffs %q: unrecognised variable %q", diffs, name)
		}

		valsByName[name] = value
	}

	out := &Encoding{
		Bits: e.Bits,
		Args: append([]Arg(nil), args...),
	}

	// Then prepare the args.
	argValsByName := make(map[string][]*Variable)
	for _, arg := range out.Args {
		if arg.Size != nil {
			argValsByName[arg.Size.Name] = append(argValsByName[arg.Size.Name], arg.Size)
		}
		if arg.Index != nil {
			argValsByName[arg.Index.Name] = append(argValsByName[arg.Index.Name], arg.Index)
		}
	}

	for _, v := range e.Variables {
		if val, ok := valsByName[v.Name]; ok {
			if int(v.Width) != len(val) {
				return nil, fmt.Errorf("invalid bit diffs %q: variable %s has width %d but diff %q has width %d", diffs, v.Name, v.Width, val, len(val))
			}

			bits, err := strconv.ParseUint(val, 2, 32)
			if err != nil {
				return nil, fmt.Errorf("invalid bit diffs %q: variable %s has malformed diff %q: %v", diffs, v.Name, val, err)
			}

			out.Bits |= BitPattern(bits) << v.Shift
			continue
		}

		if vals, ok := argValsByName[v.Name]; ok {
			for _, val := range vals {
				if val.Width != 0 {
					return nil, fmt.Errorf("invalid arg %q: arg specified multiple times", v.Name)
				}

				val.Width = v.Width
				val.Shift = v.Shift
			}

			continue
		}

		return nil, fmt.Errorf("variable %q is not resolved by the form bit diffs or operands", v.Name)
	}

	// Check that all args have been resolved.
	for _, arg := range out.Args {
		if arg.Size != nil && arg.Size.Width == 0 {
			return nil, fmt.Errorf("arg %q's size does not match any variables in the instruction", arg.Name)
		}
		if arg.Index != nil && arg.Index.Width == 0 {
			return nil, fmt.Errorf("arg %q's index does not match any variables in the instruction", arg.Name)
		}
	}

	return out, nil
}

type BitPattern uint32

func (p BitPattern) String() string {
	parts := []string{
		fmt.Sprintf("%04b", (p>>28)&0b1111),
		fmt.Sprintf("%04b", (p>>24)&0b1111),
		fmt.Sprintf("%04b", (p>>20)&0b1111),
		fmt.Sprintf("%04b", (p>>16)&0b1111),
		fmt.Sprintf("%04b", (p>>12)&0b1111),
		fmt.Sprintf("%04b", (p>>8)&0b1111),
		fmt.Sprintf("%04b", (p>>4)&0b1111),
		fmt.Sprintf("%04b", (p>>0)&0b1111),
	}

	return strings.Join(parts, "_")
}

type Variable struct {
	Name  string
	Width uint8
	Shift uint8
}

func parseEncoding(enc *XMLEncoding) (*Encoding, error) {
	var e Encoding
	var done [32]bool
	for i, b := range enc.Boxes {
		if b.Name == "" {
			// This should just be a bit pattern.
			if b.Settings == 0 || int(b.Settings) != len(b.Bits) {
				return nil, fmt.Errorf("box %d: found box with no name and settings %d (bits %d)", i, b.Settings, len(b.Bits))
			}

			for j, bit := range b.Bits {
				shift := int(b.Bit) - j
				var val BitPattern
				switch bit {
				case "0":
					val = 0
				case "1":
					val = 1
				default:
					return nil, fmt.Errorf("box %d: found box with no name and bad bit %d: %q", i, shift, bit)
				}

				if done[shift] {
					return nil, fmt.Errorf("box %d: found box with no name and multiple values for bit %d", i, shift)
				}

				done[shift] = true

				e.Bits |= val << shift
			}

			continue
		}

		// This is a variable, which is either
		// a named bitpattern or a variable
		// that will be populated later.
		complete := true
		for _, bit := range b.Bits {
			if bit != "0" && bit != "1" {
				complete = false
			}
		}

		if b.Settings != 0 && int(b.Settings) == len(b.Bits) && complete {
			// A named bitpattern.
			for j, bit := range b.Bits {
				shift := int(b.Bit) - j
				var val BitPattern
				switch bit {
				case "0":
					val = 0
				case "1":
					val = 1
				default:
					return nil, fmt.Errorf("box %d: found box with name %q and bad bit %d: %q", i, b.Name, shift, bit)
				}

				if done[shift] {
					return nil, fmt.Errorf("box %d: found box with name %q and multiple values for bit %d", i, b.Name, shift)
				}

				done[shift] = true

				e.Bits |= val << shift
			}

			continue
		}

		// A named variable.
		width := b.Width
		if width == 0 {
			width = 1
		}

		for shift := int(b.Bit); shift > int(b.Bit)-int(width); shift-- {
			if done[shift] {
				return nil, fmt.Errorf("box %d: found box with name %q and multiple values for bit %d", i, b.Name, shift)
			}

			done[shift] = true
		}

		e.Variables = append(e.Variables, Variable{
			Name:  b.Name,
			Width: width,
			Shift: b.Bit - (width - 1),
		})
	}

	for i, b := range done {
		if !b {
			return nil, fmt.Errorf("bit %d was not set by any box", i)
		}
	}

	return &e, nil
}

type Arg struct {
	Name     string
	Syntax   string
	Size     *Variable
	Index    *Variable
	Optional bool
}

var (
	fieldRegex    = regexp.MustCompile(`\(field "?([a-z0-9A-Z]+)"?\)`)
	registerRegex = regexp.MustCompile(`^(W|X)[a-z]`)
)

func parseTemplate(t string) (mnemonic string, args []Arg, err error) {
	type (
		textNode struct {
			XMLName xml.Name `xml:"text"`
			Text    string   `xml:",chardata"`
		}

		linkNode struct {
			XMLName xml.Name `xml:"a"`
			Hover   string   `xml:"hover,attr"`
			Text    string   `xml:",chardata"`
		}
	)

	parseLink := func(link *linkNode) (fragment, register, field string, err error) {
		ref := html.UnescapeString(link.Hover)
		matches := fieldRegex.FindStringSubmatch(ref)
		if len(matches) != 2 {
			fail("bad hover text %q", ref)
			return "", "", "", fmt.Errorf("failed to decode arg %d: bad hover text %q", len(args), ref)
		}

		field = matches[1]

		syntax := html.UnescapeString(link.Text)
		syntax = strings.TrimPrefix(syntax, "<")
		syntax = strings.TrimSuffix(syntax, ">")
		fragment = syntax
		syntax = strings.ReplaceAll(syntax, "|", "_")
		register = registerRegex.ReplaceAllString(syntax, "$1")

		return fragment, register, field, nil
	}

	dec := xml.NewDecoder(strings.NewReader(t))

	// Decode the mnemonic.
	var text textNode
	err = dec.Decode(&text)
	if err != nil {
		return "", nil, fmt.Errorf("failed to decode mnemonic: %v", err)
	}

	mnemonic = strings.TrimSpace(text.Text)

	// Decode any args.
	optional := 0
	for {
		// We start by parsing the beginning of
		// the arg. In cases where the arg is a
		// single field in the instruction, this
		// will do the bulk of the work.
		//
		// Where a single argument contains more
		// than one field, more will happen in
		// the components loop.
		var link linkNode
		err = dec.Decode(&link)
		if err == io.EOF {
			break
		}

		if err != nil {
			fail("not hover text: %v", err)
			return "", nil, fmt.Errorf("failed to decode arg %d: %v", len(args), err)
		}

		fragment, register, field, err := parseLink(&link)
		if err != nil {
			return "", nil, err
		}

		arg := Arg{
			Name:     fragment,
			Syntax:   register,
			Optional: optional > 0,
		}

		// If we have only one component, it
		// is generally a register index.
		arg.Index = &Variable{
			Name: field,
		}

		// Loop through the subsequent elements
		// until we reach the end, or a text
		// element containing a joining ", ".
	args:
		for {
			// We parse just the token to start,
			// so that we can branch on "a" (link)
			// and "text" (text) elements.
			next, err := dec.Token()
			if err == io.EOF {
				break
			}

			if err != nil {
				fail("not next text: %v", err)
				return "", nil, fmt.Errorf("failed to decode arg %d: %v", len(args), err)
			}

			startElt, ok := next.(xml.StartElement)
			if !ok {
				fail("invalid next element: %#v", next)
				return "", nil, fmt.Errorf("failed to decode arg %d: invalid next element %#v", len(args), next)
			}

			switch startElt.Name.Local {
			// Another field.
			case "a":
				var link linkNode
				err = dec.DecodeElement(&link, &startElt)
				if err != nil {
					fail("bad link text: %v", err)
					return "", nil, fmt.Errorf("failed to decode arg %d: bad link text: %v", len(args), err)
				}

				fragment, register, field, err := parseLink(&link)
				if err != nil {
					return "", nil, err
				}

				arg.Name += fragment
				if arg.Size != nil {
					fail("unexpected third variable %q in arg %q", field, arg.Name)
					return "", nil, fmt.Errorf("failed to decode arg %d: unexpected third variable %q", len(args), field)
				}

				if arg.Index != nil {
					arg.Size = arg.Index
				}

				arg.Index = &Variable{
					Name: field,
				}

				_ = register // Not sure what to do with this yet.

			// Joining text.
			case "text":
				var text textNode
				err = dec.DecodeElement(&text, &startElt)
				if err != nil {
					fail("bad joining text: %v", err)
					return "", nil, fmt.Errorf("failed to decode arg %d: bad joining text: %v", len(args), err)
				}

				switch text.Text {
				case ", ":
					// Move onto the next arg.
					break args
				case "{":
					optional++
				case "}":
					optional--
				case "#", " ":
					// Ignore this and keep going.
				case ".":
					// We add this to the fragment
					// but otherwise keep going.
					arg.Name += text.Text
				default:
					fail("bad joining text: %q", text.Text)
					return "", nil, fmt.Errorf("failed to decode arg %d: bad joining text: %q", len(args), text.Text)
				}

			default:
				fail("wrong next element: %s", startElt.Name)
				return "", nil, fmt.Errorf("failed to decode arg %d: wrong next element %s", len(args), startElt.Name)
			}
		}

		args = append(args, arg)
	}

	if optional != 0 {
		if optional > 0 {
			return "", nil, fmt.Errorf("failed to parse template: imbalanced braces (%d more open than close braces)", optional)
		}

		return "", nil, fmt.Errorf("failed to parse template: imbalanced braces (%d more close than open braces)", -optional)
	}

	return mnemonic, args, nil
}
