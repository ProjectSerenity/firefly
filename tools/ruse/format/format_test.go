// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package format

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"rsc.io/diff"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/parser"
	"firefly-os.dev/tools/ruse/token"
)

func TestFormatFile(t *testing.T) {
	tests := []struct {
		Name   string
		Source string
		Want   string
	}{
		{
			Name:   "package group",
			Source: "(package foo) ; foo",
			Want:   "(package foo) ; foo",
		},
		{
			Name:   "comment group",
			Source: "(package foo)\n\n; foo",
			Want:   "(package foo)\n\n; foo",
		},
		{
			Name:   "compressed run of empty comments",
			Source: "(package foo)\n\n;foo\n;\n;\n; bar",
			Want:   "(package foo)\n\n; foo\n;\n; bar",
		},
		{
			Name:   "preserved separate comment groups",
			Source: "(package foo)\n\n; foo\n\n\n; bar",
			Want:   "(package foo)\n\n; foo\n\n; bar",
		},
		{
			Name: "complex example",
			Source: `(package foo)
; Example
; 123.

(func (example (docs string) (type byte)(name string) int64  ) (+ 1	 2      3)(int->int64 (+ (len docs) ( len name ) (* (byte->int type) 2))))

;A function with the annotations already in
;the right order.
'(arch x86-64)
'(param (x int) rax)
'(param (a int32) rbx)
'(result int rax) '(strikes rax    rcx)
(asm-func
syscall1
(movq rax rcx) (xorq rdx rdx) (syscall))



; A function with annotations in the wrong order.
; The function is short so would appear all on
; one line if we didn't special-case that.
'(strikes     rcx) '(result int32 rax)
'(arch x86-64)
'(param (x int32) rax)
(asm-func double (addq rax rax))

; A complex one-line function.
(func (binary-function (x int64) (y string))
	(let _ (+ x (int->int64 (len y)))))`,
			// Breaker comment.
			Want: `(package foo)

; Example
; 123.

(func (example (docs string) (type byte) (name string) int64)
	(+ 1 2 3)
	(int->int64 (+ (len docs) (len name) (* (byte->int type) 2))))

; A function with the annotations already in
; the right order.
'(arch x86-64)
'(param (x int) rax)
'(param (a int32) rbx)
'(result int rax)
'(strikes rax rcx)
(asm-func syscall1
	(movq rax rcx)
	(xorq rdx rdx)
	(syscall))

; A function with annotations in the wrong order.
; The function is short so would appear all on
; one line if we didn't special-case that.
'(arch x86-64)
'(param (x int32) rax)
'(result int32 rax)
'(strikes rcx)
(asm-func double
	(addq rax rax))

; A complex one-line function.
(func (binary-function (x int64) (y string))
	(let _ (+ x (int->int64 (len y)))))`,
		},
	}

	var buf bytes.Buffer
	var builder strings.Builder
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			buf.Reset()
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "input.ruse", test.Source, parser.ParseComments)
			if err != nil {
				t.Fatalf("ParseFile(): %v", err)
			}

			SortAnnotations(file)

			err = Fprint(&buf, fset, file)
			if err != nil {
				t.Fatalf("Fprint(): %v", err)
			}

			got := buf.String()
			if got != test.Want+"\n" {
				t.Fatalf("Fprint(): (+got, -want)\n%s", diff.Format(test.Want+"\n", got))
			}

			// Check that interpreting the original source and
			// the formatted source gives the same result, as
			// formatting should not result in semantic changes.

			origParsed, err := parser.ParseFile(fset, "reparsed.ruse", test.Source, parser.ParseComments)
			if err != nil {
				t.Fatalf("ParseFile(test.Source): %v", err)
			}

			formattedParsed, err := parser.ParseFile(fset, "formatted.ruse", got, parser.ParseComments)
			if err != nil {
				t.Fatalf("ParseFile(formatted): %v", err)
			}

			// Repeat sorting so we're consistent.
			SortAnnotations(origParsed)

			// Ignore positions and comments, as these are changed.
			if diff := cmp.Diff(origParsed, formattedParsed, cmpopts.IgnoreTypes(token.Pos(0), new(ast.Comment))); diff != "" {
				t.Fatalf("Fprintf(): (+got, -want)\n%s", diff)
			}

			// Check that formatting the formatted code results
			// in exactly the same sequence of bytes.

			format1 := got

			builder.Reset()
			err = Fprint(&builder, fset, formattedParsed)
			if err != nil {
				t.Fatalf("Fprint(formatted): %v", err)
			}

			format2 := builder.String()
			if format2 != format1 {
				t.Fatalf("Fprint(formatted): (+got, -want)\n%s", diff.Format(format1, format2))
			}
		})
	}
}
