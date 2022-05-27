// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package format

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"rsc.io/diff"

	"firefly-os.dev/tools/plan/parser"
	"firefly-os.dev/tools/plan/types"
)

func TestFormatFile(t *testing.T) {
	tests := []struct {
		Name   string
		Source string
		Want   string
	}{
		{
			Name:   "comment group",
			Source: `; foo`,
			Want:   `; foo`,
		},
		{
			Name:   "compressed run of empty comments",
			Source: ";foo\n;\n;\n; bar",
			Want:   "; foo\n;\n; bar",
		},
		{
			Name:   "preserved separate comment groups",
			Source: "; foo\n\n; bar",
			Want:   "; foo\n\n\n; bar",
		},
		{
			Name: "complex example",
			Source: `; Example
; 123.

(structure (field (docs "foo") (type byte)(name the first) ) (docs "quite a long string" "another string")(name asdf baz             example)  (field (docs "Extra padding.") (name spacer) (padding    3)))

(enumeration (type uint8) (value (name bob) (docs "bob")) (name names) (docs "set of names") (value (name dave) (docs "dave")))
(enumeration (name read error) (docs "Failure to read data") (type sint8)
	(value (name no error) (docs "All is well."))
	(value (name bad syscall) (docs "this syscall does not exist"))
	(value (name illegal parameter1) (docs "The police are getting involved."))
	(value (name illegal parameter2) (docs "The police are getting involved."))
	(value (name illegal parameter3) (docs "The police are getting involved."))
	(value (name illegal parameter4) (docs "The police are getting involved."))
	(value (name illegal parameter5) (docs "The police are getting involved."))
	(value (name illegal parameter6) (docs "The police are getting involved.")))

; Another comment.

(syscall (name read) (docs "read things")
(arg2 (name size) (docs "length") (type uint64))
(result2 (name error) (docs "what went wrong") (type read error))
(result1 (name name) (docs "who dun it") (type names))
(arg1 (name ptr) (type *mutable byte) (docs "address")))`,
			Want: `; Example
; 123.


(structure
	(name asdf baz example)
	(docs "quite a long string" "another string")
	(field
		(name the first)
		(docs "foo")
		(type byte))
	(field
		(name spacer)
		(docs "Extra padding.")
		(padding 3)))


(enumeration
	(name names)
	(docs "set of names")
	(type uint8)
	(value
		(name bob)
		(docs "bob"))
	(value
		(name dave)
		(docs "dave")))


(enumeration
	(name read error)
	(docs "Failure to read data")
	(type sint8)
	(value
		(name no error)
		(docs "All is well."))
	(value
		(name bad syscall)
		(docs "this syscall does not exist"))
	(value
		(name illegal parameter1)
		(docs "The police are getting involved."))
	(value
		(name illegal parameter2)
		(docs "The police are getting involved."))
	(value
		(name illegal parameter3)
		(docs "The police are getting involved."))
	(value
		(name illegal parameter4)
		(docs "The police are getting involved."))
	(value
		(name illegal parameter5)
		(docs "The police are getting involved."))
	(value
		(name illegal parameter6)
		(docs "The police are getting involved.")))


; Another comment.


(syscall
	(name read)
	(docs "read things")
	(arg1
		(name ptr)
		(docs "address")
		(type *mutable byte))
	(arg2
		(name size)
		(docs "length")
		(type uint64))
	(result1
		(name name)
		(docs "who dun it")
		(type names))
	(result2
		(name error)
		(docs "what went wrong")
		(type read error)))`,
		},
	}

	var buf bytes.Buffer
	var builder strings.Builder
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			buf.Reset()
			file, err := parser.ParseFile("test.plan", test.Source)
			if err != nil {
				t.Fatalf("ParseFile(): %v", err)
			}

			err = SortFields(file, types.X86_64)
			if err != nil {
				t.Fatalf("SortFields(): %v", err)
			}

			err = Fprint(&buf, file)
			if err != nil {
				t.Fatalf("Fprint(): %v", err)
			}

			got := buf.String()
			if got != test.Want+"\n" {
				t.Fatalf("Fprint():\n%s", diff.Format(got, test.Want+"\n"))
			}

			// Check that interpreting the original source and
			// the formatted source gives the same result, as
			// formatting should not result in semantic changes.

			origParsed, err := parser.ParseFile("test.plan", test.Source)
			if err != nil {
				t.Fatalf("ParseFile(test.Source): %v", err)
			}

			origInterpreted, err := types.Interpret("test.plan", origParsed, types.X86_64)
			if err != nil {
				t.Fatalf("Interpret(test.Source): %v", err)
			}

			formattedParsed, err := parser.ParseFile("test.plan", got)
			if err != nil {
				t.Fatalf("ParseFile(formatted): %v", err)
			}

			formattedInterpreted, err := types.Interpret("test.plan", formattedParsed, types.X86_64)
			if err != nil {
				t.Fatalf("Interpret(formatted): %v", err)
			}

			// Ignore the AST nodes, as their positions will change.
			origInterpreted.DropAST()
			formattedInterpreted.DropAST()

			if !reflect.DeepEqual(origInterpreted, formattedInterpreted) {
				// Encoding the values in JSON makes the error
				// message more useful and legible.
				gotJSON, err := json.MarshalIndent(origInterpreted, "", "\t")
				if err != nil {
					t.Fatal(err)
				}

				wantJSON, err := json.MarshalIndent(formattedInterpreted, "", "\t")
				if err != nil {
					t.Fatal(err)
				}

				t.Fatalf("Interpret():\n%s", diff.Format(string(gotJSON), string(wantJSON)))
			}

			// Check that formatting the formatted code results
			// in exactly the same sequence of bytes.

			format1 := got

			builder.Reset()
			err = Fprint(&builder, formattedParsed)
			if err != nil {
				t.Fatalf("Fprint(formatted): %v", err)
			}

			format2 := builder.String()
			if format2 != format1 {
				t.Fatalf("Fprint(formatted):\n%s", diff.Format(format2, format1))
			}
		})
	}
}
