package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ProjectSerenity/firefly/kcheck/cc"
)

func TestCommands(t *testing.T) {
	span := func(name string, line int) cc.Span {
		return cc.Span{
			Start: cc.Pos{
				File: filepath.Join("testdata", name),
				Line: line,
			},
			End: cc.Pos{
				File: filepath.Join("testdata", name),
				Line: line,
			},
		}
	}

	tests := []struct {
		Name string
		Func func(issues chan<- Issue, name string)
		Want []Issue
	}{
		{
			Name: "good.h",
			Func: processHeader,
			Want: nil,
		},
		{
			Name: "no_pragma_once.h",
			Func: processHeader,
			Want: []Issue{
				{
					Span:  span("no_pragma_once.h", 1),
					Error: fmt.Errorf("missing %q", "#pragma once"),
				},
			},
		},
		{
			Name: "no_ifndef.h",
			Func: processHeader,
			Want: []Issue{
				{
					Span:  span("no_ifndef.h", 2),
					Error: fmt.Errorf("missing %q", "#ifndef NO_IFNDEF_H"),
				},
			},
		},
		{
			Name: "no_define.h",
			Func: processHeader,
			Want: []Issue{
				{
					Span:  span("no_define.h", 3),
					Error: fmt.Errorf("missing %q", "#define NO_DEFINE_H"),
				},
			},
		},
		{
			Name: "no_endif.h",
			Func: processHeader,
			Want: []Issue{
				{
					Span:  span("no_endif.h", 5),
					Error: fmt.Errorf("missing %q", "#endif // NO_ENDIF_H"),
				},
			},
		},
		{
			Name: "nested_include.h",
			Func: processHeader,
			Want: []Issue{
				{
					Span:  span("nested_include.h", 5),
					Error: fmt.Errorf("nested #include"),
				},
			},
		},
		{
			Name: "bad_indentation.h",
			Func: processHeader,
			Want: []Issue{
				{
					Span:  span("bad_indentation.h", 5),
					Error: fmt.Errorf("non-tab indentation (' ')"),
				},
			},
		},
		{
			Name: "trailing_whitespace.h",
			Func: processHeader,
			Want: []Issue{
				{
					Span:  span("trailing_whitespace.h", 5),
					Error: fmt.Errorf("trailing whitespace"),
				},
			},
		},
		{
			Name: "bad_indentation.c",
			Func: processObject,
			Want: []Issue{
				{
					Span:  span("bad_indentation.c", 2),
					Error: fmt.Errorf("non-tab indentation (' ')"),
				},
			},
		},
		{
			Name: "trailing_whitespace.c",
			Func: processObject,
			Want: []Issue{
				{
					Span:  span("trailing_whitespace.c", 2),
					Error: fmt.Errorf("trailing whitespace"),
				},
			},
		},
		{
			Name: "bad_str_variable.c",
			Func: processObject,
			Want: []Issue{
				{
					Span:  span("bad_str_variable.c", 5),
					Error: fmt.Errorf("str argument is not a string literal"),
				},
			},
		},
		{
			Name: "bad_printk_variable_format_string.c",
			Func: processObject,
			Want: []Issue{
				{
					Span:  span("bad_printk_variable_format_string.c", 5),
					Error: fmt.Errorf("std_Printk format string is not a string literal"),
				},
			},
		},
		{
			Name: "bad_printk_verb_missing_arg.c",
			Func: processObject,
			Want: []Issue{
				{
					Span:  span("bad_printk_verb_missing_arg.c", 4),
					Error: fmt.Errorf("std_Printk missing arg for verb %d (%q)", 1, "%m3s"),
				},
			},
		},
		{
			Name: "bad_printk_verb_mismatch_int_string.c",
			Func: processObject,
			Want: []Issue{
				{
					Span:  span("bad_printk_verb_mismatch_int_string.c", 5),
					Error: fmt.Errorf("std_Printk arg for verb %d (%q) has non-integer type %s", 1, "%u8d", "pointer"),
				},
			},
		},
		{
			Name: "bad_printk_verb_mismatch_int_uint.c",
			Func: processObject,
			Want: []Issue{
				{
					Span:  span("bad_printk_verb_mismatch_int_uint.c", 5),
					Error: fmt.Errorf("std_Printk arg for verb %d (%q) is not unsigned", 1, "%u8d"),
				},
			},
		},
		{
			Name: "bad_printk_verb_mismatch_uint_int.c",
			Func: processObject,
			Want: []Issue{
				{
					Span:  span("bad_printk_verb_mismatch_uint_int.c", 5),
					Error: fmt.Errorf("std_Printk arg for verb %d (%q) is unsigned", 1, "%+8d"),
				},
			},
		},
		{
			Name: "bad_printk_verb_mismatch_uint8_uint64_literal.c",
			Func: processObject,
			Want: []Issue{
				{
					Span:  span("bad_printk_verb_mismatch_uint8_uint64_literal.c", 4),
					Error: fmt.Errorf("std_Printk arg for verb %d (%q) is too large (%d bits)", 1, "%u8d", 64),
				},
			},
		},
		{
			Name: "bad_printk_verb_mismatch_uint8_uint64_var.c",
			Func: processObject,
			Want: []Issue{
				{
					Span:  span("bad_printk_verb_mismatch_uint8_uint64_var.c", 5),
					Error: fmt.Errorf("std_Printk arg for verb %d (%q) is too large (%d bits)", 1, "%u8d", 64),
				},
			},
		},
		{
			Name: "bad_printk_verb_mismatch_char_string.c",
			Func: processObject,
			Want: []Issue{
				{
					Span:  span("bad_printk_verb_mismatch_char_string.c", 5),
					Error: fmt.Errorf("std_Printk arg for verb %d (%q) has non-character type %s", 1, "%c", "pointer"),
				},
			},
		},
		{
			Name: "bad_printk_verb_mismatch_char_uint.c",
			Func: processObject,
			Want: []Issue{
				{
					Span:  span("bad_printk_verb_mismatch_char_uint.c", 5),
					Error: fmt.Errorf("std_Printk arg for verb %d (%q) is unsigned", 1, "%c"),
				},
			},
		},
		{
			Name: "bad_printk_verb_mismatch_char_int64_literal.c",
			Func: processObject,
			Want: []Issue{
				{
					Span:  span("bad_printk_verb_mismatch_char_int64_literal.c", 4),
					Error: fmt.Errorf("std_Printk arg for verb %d (%q) is too large (%d bits)", 1, "%c", 64),
				},
			},
		},
		{
			Name: "bad_printk_verb_mismatch_string_int.c",
			Func: processObject,
			Want: []Issue{
				{
					Span:  span("bad_printk_verb_mismatch_string_int.c", 4),
					Error: fmt.Errorf("std_Printk arg for verb %d (%q) is non-string type %s", 1, "%m1s", "int32"),
				},
			},
		},
		{
			Name: "bad_printk_verb_mismatch_string_pointer.c",
			Func: processObject,
			Want: []Issue{
				{
					Span:  span("bad_printk_verb_mismatch_string_pointer.c", 5),
					Error: fmt.Errorf("std_Printk arg for verb %d (%q) is non-string type *%s", 1, "%m1s", "int32"),
				},
			},
		},
		{
			Name: "bad_printk_verb_mismatch_buffer_int.c",
			Func: processObject,
			Want: []Issue{
				{
					Span:  span("bad_printk_verb_mismatch_buffer_int.c", 4),
					Error: fmt.Errorf("std_Printk arg for verb %d (%q) is non-string type %s", 1, "%m1x", "int32"),
				},
			},
		},
		{
			Name: "bad_printk_verb_mismatch_buffer_pointer.c",
			Func: processObject,
			Want: []Issue{
				{
					Span:  span("bad_printk_verb_mismatch_buffer_pointer.c", 5),
					Error: fmt.Errorf("std_Printk arg for verb %d (%q) is non-buffer type *%s", 1, "%m1x", "int32"),
				},
			},
		},
		{
			Name: "bad_printk_verb_mismatch_pointer_int.c",
			Func: processObject,
			Want: []Issue{
				{
					Span:  span("bad_printk_verb_mismatch_pointer_int.c", 4),
					Error: fmt.Errorf("std_Printk arg for verb %d (%q) is non-pointer type %s", 1, "%p", "int32"),
				},
			},
		},
		{
			Name: "bad_printk_extra_values.c",
			Func: processObject,
			Want: []Issue{
				{
					Span:  span("bad_printk_extra_values.c", 4),
					Error: fmt.Errorf("std_Printk has %d extra arguments not used by verbs", 1),
				},
			},
		},
		{
			Name: "valid_printk.c",
			Func: processObject,
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			full := filepath.Join("testdata", test.Name)

			var got []Issue
			done := make(chan struct{})
			issues := make(chan Issue)
			go func() {
				for issue := range issues {
					if len(got) < 10 {
						got = append(got, issue)
					}
				}

				close(done)
			}()

			test.Func(issues, full)
			close(issues)
			<-done

			if test.Want == nil {
				if got != nil {
					t.Errorf("got unexpected issues: %v", got)
				}

				return
			}

			for i := range got {
				got[i].Span.Start.Byte = 0
				got[i].Span.End.Byte = 0
			}

			if !reflect.DeepEqual(got, test.Want) {
				t.Errorf("Unexpected issues:\nGot:  %v\nWant: %v", got, test.Want)
			}
		})
	}
}

func TestParsePrintkFormat(t *testing.T) {
	tests := []struct {
		// Input.
		Format string

		// Error fields.
		Start  int
		Length int
		Err    string

		// Success fields.
		Want []printkVerb
	}{
		// No verbs, but valid.
		{
			Format: "",
		},
		{
			Format: "%%",
		},
		{
			Format: "asdf",
		},
		// Invalid verbs.
		{
			Format: "%m%",
			Length: 3,
			Err:    "invalid escaped percent",
		},
		{
			Format: "%u32u32d",
			Length: 5,
			Err:    "conflicting methods",
		},
		{
			Format: "%u32+32d",
			Length: 5,
			Err:    "conflicting methods",
		},
		{
			Format: "%u32 x",
			Length: 5,
			Err:    "conflicting methods",
		},
		{
			Format: "%m3m32x",
			Length: 4,
			Err:    "conflicting methods",
		},
		{
			Format: "%u32 x",
			Length: 5,
			Err:    "conflicting methods",
		},
		{
			Format: "%05w3d",
			Length: 4,
			Err:    "conflicting methods",
		},
		{
			Format: "%u8w3d",
			Length: 4,
			Err:    "width modifier specified after sign modifier",
		},
		{
			Format: "%8d",
			Length: 2,
			Err:    "unprefixed number modifier",
		},
		{
			Format: "%m3d",
			Length: 4,
			Err:    "memory modifier used with integer",
		},
		{
			Format: "% d",
			Length: 3,
			Err:    "space modifier used with integer",
		},
		{
			Format: "%d",
			Length: 2,
			Err:    "missing integer width",
		},
		{
			Format: "%u2d",
			Length: 4,
			Err:    "invalid integer width 2",
		},
		{
			Format: "%u8c",
			Length: 4,
			Err:    "unsigned modifier used with character",
		},
		{
			Format: "%+8c",
			Length: 4,
			Err:    "signed modifier used with character",
		},
		{
			Format: "%m2c",
			Length: 4,
			Err:    "memory modifier used with character",
		},
		{
			Format: "% c",
			Length: 3,
			Err:    "space modifier used with character",
		},
		{
			Format: "%w2c",
			Length: 4,
			Err:    "width modifier used with character",
		},
		{
			Format: "%02c",
			Length: 4,
			Err:    "zero modifier used with character",
		},
		{
			Format: "%u8s",
			Length: 4,
			Err:    "unsigned modifier used with string",
		},
		{
			Format: "%+8s",
			Length: 4,
			Err:    "signed modifier used with string",
		},
		{
			Format: "% s",
			Length: 3,
			Err:    "space modifier used with string",
		},
		{
			Format: "%02s",
			Length: 4,
			Err:    "zero modifier used with string",
		},
		{
			Format: "%w2m2x",
			Length: 6,
			Err:    "width modifier used with buffer",
		},
		{
			Format: "%02m2x",
			Length: 6,
			Err:    "zero modifier used with buffer",
		},
		{
			Format: "%u8h",
			Length: 4,
			Err:    "unsigned modifier used with hexdump",
		},
		{
			Format: "%+8h",
			Length: 4,
			Err:    "signed modifier used with hexdump",
		},
		{
			Format: "% h",
			Length: 3,
			Err:    "space modifier used with hexdump",
		},
		{
			Format: "%w8h",
			Length: 4,
			Err:    "width modifier used with hexdump",
		},
		{
			Format: "%08h",
			Length: 4,
			Err:    "zero modifier used with hexdump",
		},
		{
			Format: "%u8p",
			Length: 4,
			Err:    "unsigned modifier used with pointer",
		},
		{
			Format: "%+8p",
			Length: 4,
			Err:    "signed modifier used with pointer",
		},
		{
			Format: "%m2p",
			Length: 4,
			Err:    "memory modifier used with pointer",
		},
		{
			Format: "% p",
			Length: 3,
			Err:    "space modifier used with pointer",
		},
		{
			Format: "%w8p",
			Length: 4,
			Err:    "width modifier used with pointer",
		},
		{
			Format: "%08p",
			Length: 4,
			Err:    "zero modifier used with pointer",
		},
		{
			Format: "%~",
			Length: 2,
			Err:    "unrecognised verb '~'",
		},
		{
			Format: "%",
			Length: 1,
			Err:    "unterminated verb",
		},
		{
			Format: "%u8d %08p",
			Start:  5,
			Length: 4,
			Err:    "zero modifier used with pointer",
		},
		// Valid verbs.
		{
			Format: "%u8b",
			Want: []printkVerb{
				{
					Text:     "%u8b",
					Integer:  true,
					Unsigned: true,
					Base:     2,
					ArgWidth: 8,
				},
			},
		},
		{
			Format: "%u8d",
			Want: []printkVerb{
				{
					Text:     "%u8d",
					Integer:  true,
					Unsigned: true,
					Base:     10,
					ArgWidth: 8,
				},
			},
		},
		{
			Format: "%010+16o",
			Want: []printkVerb{
				{
					Text:         "%010+16o",
					Integer:      true,
					Signed:       true,
					ZeroPrefixed: true,
					Base:         8,
					ArgWidth:     16,
					MinWidth:     10,
				},
			},
		},
		{
			Format: "%w16u32x",
			Want: []printkVerb{
				{
					Text:          "%w16u32x",
					Integer:       true,
					Unsigned:      true,
					SpacePrefixed: true,
					Base:          16,
					ArgWidth:      32,
					MinWidth:      16,
				},
			},
		},
		{
			Format: "%u64X",
			Want: []printkVerb{
				{
					Text:      "%u64X",
					Integer:   true,
					Unsigned:  true,
					Base:      16,
					ArgWidth:  64,
					UpperCase: true,
				},
			},
		},
		{
			Format: "%c",
			Want: []printkVerb{
				{
					Text:      "%c",
					Character: true,
				},
			},
		},
		{
			Format: "%s",
			Want: []printkVerb{
				{
					Text:   "%s",
					String: true,
				},
			},
		},
		{
			Format: "%m4s",
			Want: []printkVerb{
				{
					Text:     "%m4s",
					String:   true,
					Memory:   true,
					ArgWidth: 4,
				},
			},
		},
		{
			Format: "%w8m4s",
			Want: []printkVerb{
				{
					Text:          "%w8m4s",
					String:        true,
					Memory:        true,
					SpacePrefixed: true,
					ArgWidth:      4,
					MinWidth:      8,
				},
			},
		},
		{
			Format: "%m4x",
			Want: []printkVerb{
				{
					Text:     "%m4x",
					Buffer:   true,
					Memory:   true,
					ArgWidth: 4,
				},
			},
		},
		{
			Format: "%m4X",
			Want: []printkVerb{
				{
					Text:      "%m4X",
					Buffer:    true,
					Memory:    true,
					ArgWidth:  4,
					UpperCase: true,
				},
			},
		},
		{
			Format: "%m4 x",
			Want: []printkVerb{
				{
					Text:           "%m4 x",
					Buffer:         true,
					Memory:         true,
					SpaceSeparated: true,
					ArgWidth:       4,
				},
			},
		},
		{
			Format: "%m4h",
			Want: []printkVerb{
				{
					Text:     "%m4h",
					Hexdump:  true,
					Memory:   true,
					ArgWidth: 4,
				},
			},
		},
		{
			Format: "%p",
			Want: []printkVerb{
				{
					Text:    "%p",
					Pointer: true,
				},
			},
		},
		{
			Format: "%u8d %p",
			Want: []printkVerb{
				{
					Text:     "%u8d",
					Integer:  true,
					Unsigned: true,
					Base:     10,
					ArgWidth: 8,
				},
				{
					Text:    "%p",
					Pointer: true,
				},
			},
		},
	}

	for _, test := range tests {
		got, err := parsePrintkFormat(test.Format)
		if test.Err == "" {
			if err != nil {
				t.Errorf("printk(%q): unexpected error %v", test.Format, err)
				continue
			}

			if !reflect.DeepEqual(got, test.Want) {
				gotList := make([]string, len(got))
				for i, got := range got {
					gotList[i] = fmt.Sprintf("%#v", got)
				}

				wantList := make([]string, len(test.Want))
				for i, want := range test.Want {
					wantList[i] = fmt.Sprintf("%#v", want)
				}

				gotS := strings.Join(gotList, "\n\t\t")
				wantS := strings.Join(wantList, "\n\t\t")

				t.Errorf("printk(%q):\n\tGot:\n\t\t%s\n\n\tWant:\n\t\t%s", test.Format, gotS, wantS)
				continue
			}

			continue
		}

		if err == nil {
			gotList := make([]string, len(got))
			for i, got := range got {
				gotList[i] = fmt.Sprintf("%#v", got)
			}

			gotS := strings.Join(gotList, "\n\t\t")

			t.Errorf("printk(%q):\n\tGot:\n\t\t%s\n\n\tWant error %s", test.Format, gotS, test.Err)
			continue
		}

		pfe, ok := err.(printkFormatError)
		if !ok {
			t.Errorf("printk(%q): got unexpected error type %#v", test.Format, err)
			continue
		}

		if pfe.Start != test.Start || pfe.Length != test.Length || pfe.Err != test.Err {
			t.Errorf("printk(%q):\nGot:  %d-%d %q\nWant: %d-%d %q", test.Format, pfe.Start, pfe.Length, pfe.Err, test.Start, test.Length, test.Err)
			continue
		}
	}
}

func TestNumericalTypes(t *testing.T) {
	// Check that cc agrees on the size of
	// our sized types.
	name := filepath.Join("testdata", "std.h")
	f, err := os.Open(name)
	if err != nil {
		t.Fatalf("failed to open %s: %v", name, err)
	}

	defer f.Close()

	prog, err := cc.Read(name, f)
	if err != nil {
		t.Fatalf("failed to parse %s: %v", name, err)
	}

	types := []string{
		"int8",
		"int16",
		"int32",
		"int64",
		"uint8",
		"uint16",
		"uint32",
		"uint64",
		"float32",
		"float64",
	}

	for _, decl := range prog.Decls {
		match := false
		for _, want := range types {
			if decl.Name == want {
				match = true
				break
			}
		}

		if !match {
			continue
		}

		if decl.Name != decl.Type.Kind.String() {
			t.Errorf("cc sees %s as %s", decl.Name, decl.Type.Kind)
		}
	}
}
