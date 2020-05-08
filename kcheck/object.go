// Process C files, enforcing several properties:
//
// 	- C files' use of printk is checked for the
// 	  correct number of args, with correct verb use.
// 	- All files must be indented with tabs and have
// 	  no trailling whitespace.
//

package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"unicode"

	"github.com/ProjectSerenity/firefly/kcheck/cc"
)

var (
	objectFlags = flag.NewFlagSet("object", flag.ExitOnError)
	objectDesc  = "FILE"
)

func objectCommand(issues chan<- Issue, args []string) {
	if len(args) == 0 {
		log.Printf("No object specified\n\n")
		usage()
	}

	for _, arg := range args {
		processObject(issues, arg)
	}
}

func init() {
	registerCommand(objectFlags, objectDesc, objectCommand)
}

const (
	buildIgnore = "// +build ignore"
)

func processObject(issues chan<- Issue, name string) {
	f, err := os.Open(name)
	if err != nil {
		log.Fatalf("failed to open %q: %v", name, err)
	}

	defer f.Close()

	s := bufio.NewScanner(f)
	for line := 1; s.Scan(); line++ {
		text := s.Text()
		runes := []rune(text)

		span := func() cc.Span {
			return cc.Span{
				Start: cc.Pos{
					File: name,
					Line: line,
				},
				End: cc.Pos{
					File: name,
					Line: line,
				},
			}
		}

		// Check indentation.
		for _, r := range runes {
			if !unicode.IsSpace(r) {
				break
			}

			if r != '\t' {
				Errorf(issues, span(), "non-tab indentation (%s)", strconv.QuoteRune(r))
				break
			}
		}

		// Check for trailing spaces.
		for i := len(runes); i > 0; i-- {
			r := runes[i-1]
			if !unicode.IsSpace(r) {
				break
			}

			Errorf(issues, span(), "trailing whitespace")
			break
		}
	}

	if err := s.Err(); err != nil {
		log.Fatalf("failed to parse %s: %v", name, err)
	}

	_, err = f.Seek(0, 0)
	if err != nil {
		log.Fatalf("failed to reset %s: %v", name, err)
	}

	prog, err := cc.Read(name, f)
	if err != nil {
		log.Fatalf("failed to parse %s: %v", name, err)
	}

	// Validate str calls.
	cc.Preorder(prog, func(s cc.Syntax) {
		callExpr, ok := s.(*cc.Expr)
		if !ok {
			return
		}

		if callExpr.Op != cc.Call {
			return
		}

		if callExpr.Left == nil || callExpr.Left.Op != cc.Name || callExpr.Left.Text != "str" {
			return
		}

		if len(callExpr.List) == 0 {
			Errorf(issues, callExpr.Span, "str has no arguments")
			return
		}

		formatExpr := callExpr.List[0]
		if formatExpr.Op != cc.String {
			Errorf(issues, formatExpr.Span, "str argument is not a string literal")
			return
		}
	})

	// Validate printk calls.
	cc.Preorder(prog, func(s cc.Syntax) {
		callExpr, ok := s.(*cc.Expr)
		if !ok {
			return
		}

		if callExpr.Op != cc.Call {
			return
		}

		if callExpr.Left == nil || callExpr.Left.Op != cc.Name || callExpr.Left.Text != "std_Printk" {
			return
		}

		if len(callExpr.List) == 0 {
			Errorf(issues, callExpr.Span, "std_Printk has no arguments")
			return
		}

		formatExpr := callExpr.List[0]
		if formatExpr.Op != cc.String {
			Errorf(issues, formatExpr.Span, "std_Printk format string is not a string literal")
			return
		}

		var format string
		for _, literal := range formatExpr.Texts {
			unquoted, err := strconv.Unquote(literal)
			if err != nil {
				Errorf(issues, formatExpr.Span, "std_Printk format string could not be unquoted")
				return
			}

			format += unquoted
		}

		verbs, err := parsePrintkFormat(format)
		if err != nil {
			Errorf(issues, formatExpr.Span, "std_Printk format string invalid: %v", err)
			return
		}

		args := callExpr.List[1:]
		for i, verb := range verbs {
			if i >= len(args) {
				Errorf(issues, callExpr.Span, "std_Printk missing arg for verb %d (%q)", i+1, verb.Text)
				return
			}

			arg := args[i]
			argType := arg.XType
			for argType != nil && argType.Kind == cc.TypedefType {
				argType = argType.Base
			}

			if argType == nil {
				Errorf(issues, callExpr.Span, "std_Printk arg for verb %d (%q) has unknown type", i+1, verb.Text)
				return
			}

			switch {
			case verb.Integer:
				signed, argWidth, ok := numberType(argType.Kind)
				if !ok {
					Errorf(issues, callExpr.Span, "std_Printk arg for verb %d (%q) has non-integer type %s", i+1, verb.Text, argType.Kind)
					return
				}

				if verb.Unsigned && signed {
					Errorf(issues, callExpr.Span, "std_Printk arg for verb %d (%q) is not unsigned", i+1, verb.Text)
				} else if verb.Signed && !signed {
					Errorf(issues, callExpr.Span, "std_Printk arg for verb %d (%q) is unsigned", i+1, verb.Text)
				}

				if argWidth > verb.ArgWidth && argWidth > 32 {
					Errorf(issues, callExpr.Span, "std_Printk arg for verb %d (%q) is too large (%d bits)", i+1, verb.Text, argWidth)
					return
				}
			case verb.Character:
				signed, argWidth, ok := numberType(argType.Kind)
				if !ok {
					Errorf(issues, callExpr.Span, "std_Printk arg for verb %d (%q) has non-character type %s", i+1, verb.Text, argType.Kind)
					return
				}

				if !signed {
					Errorf(issues, callExpr.Span, "std_Printk arg for verb %d (%q) is unsigned", i+1, verb.Text)
				}

				if argWidth > 32 {
					Errorf(issues, callExpr.Span, "std_Printk arg for verb %d (%q) is too large (%d bits)", i+1, verb.Text, argWidth)
					return
				}
			case verb.String:
				if argType.Kind != cc.Ptr && argType.Kind != cc.Array {
					Errorf(issues, callExpr.Span, "std_Printk arg for verb %d (%q) is non-string type %s", i+1, verb.Text, argType.Kind)
					return
				}

				if argType.Base == nil || (!argType.Base.Is(cc.Uint8) && !argType.Base.Is(cc.Int8)) {
					Errorf(issues, callExpr.Span, "std_Printk arg for verb %d (%q) is non-string type *%s", i+1, verb.Text, argType.Base.Kind)
					return
				}
			case verb.Buffer, verb.Hexdump:
				if argType.Is(cc.Uintptr) {
					continue
				}

				if !argType.Is(cc.Ptr) && !argType.Is(cc.Array) {
					Errorf(issues, callExpr.Span, "std_Printk arg for verb %d (%q) is non-string type %s", i+1, verb.Text, argType.Kind)
					return
				}

				if argType.Base == nil || (!argType.Base.Is(cc.Uint8) && !argType.Base.Is(cc.Int8)) {
					Errorf(issues, callExpr.Span, "std_Printk arg for verb %d (%q) is non-buffer type *%s", i+1, verb.Text, argType.Base.Kind)
					return
				}
			case verb.Pointer:
				if !argType.Is(cc.Ptr) && !argType.Is(cc.Array) && !argType.Is(cc.Uintptr) {
					Errorf(issues, callExpr.Span, "std_Printk arg for verb %d (%q) is non-pointer type %s", i+1, verb.Text, argType.Kind)
					return
				}
			}
		}

		if len(args) > len(verbs) {
			Errorf(issues, callExpr.Span, "std_Printk has %d extra arguments not used by verbs", len(args)-len(verbs))
			return
		}
	})
}

func numberType(kind cc.TypeKind) (signed bool, size int, ok bool) {
	ok = true
	switch kind {
	case cc.Int8:
		signed = true
		fallthrough
	case cc.Uint8:
		size = 8
	case cc.Int16:
		signed = true
		fallthrough
	case cc.Uint16:
		size = 16
	case cc.Int32:
		signed = true
		fallthrough
	case cc.Uint32:
		size = 32
	case cc.Int64:
		signed = true
		fallthrough
	case cc.Uint64:
		size = 64
	case cc.Uintptr:
		size = 64
	default:
		ok = false
	}

	return signed, size, ok
}

type printkVerb struct {
	Text string

	Integer   bool
	Character bool
	String    bool
	Buffer    bool
	Hexdump   bool
	Pointer   bool

	Unsigned       bool
	Signed         bool
	Memory         bool
	SpacePrefixed  bool
	ZeroPrefixed   bool
	SpaceSeparated bool
	UpperCase      bool

	Base     int
	ArgWidth int
	MinWidth int
}

type printkFormatError struct {
	Start  int
	Length int
	Err    string
}

func (e printkFormatError) Error() string {
	return e.Err
}

func parsePrintkFormat(format string) (verbs []printkVerb, err error) {
	var (
		inVerb     bool
		verbStart  int
		isUnsigned bool
		isSigned   bool
		isMemory   bool
		isWidth    bool
		isZero     bool
		addSpace   bool
		size       int
		minWidth   int
	)

	reset := func() {
		inVerb = false
		verbStart = 0
		isUnsigned = false
		isSigned = false
		isMemory = false
		isWidth = false
		isZero = false
		addSpace = false
		size = 0
		minWidth = 0
	}

	errorf := func(end int, format string, v ...interface{}) ([]printkVerb, error) {
		err := fmt.Sprintf(format, v...)
		return nil, printkFormatError{verbStart, 1 + end - verbStart, err}
	}

	for i, r := range format {
		// Non-verb content.

		if !inVerb && r != '%' {
			continue
		}

		// Start of a verb.

		if !inVerb {
			inVerb = true
			verbStart = i
			continue
		}

		// Escaped percent.

		if r == '%' {
			if verbStart != i-1 {
				return errorf(i, "invalid escaped percent")
			}

			reset()
			continue
		}

		// Modifiers.

		switch r {
		case 'u':
			if isUnsigned || isSigned || isMemory || addSpace {
				return errorf(i, "conflicting methods")
			}

			isUnsigned = true
			continue
		case '+':
			if isUnsigned || isSigned || isMemory || addSpace {
				return errorf(i, "conflicting methods")
			}

			isSigned = true
			continue
		case 'm':
			if isUnsigned || isSigned || isMemory {
				return errorf(i, "conflicting methods")
			}

			isMemory = true
			continue
		case ' ':
			if isUnsigned || isSigned || addSpace {
				return errorf(i, "conflicting methods")
			}

			addSpace = true
			continue
		case 'w':
			if isZero {
				return errorf(i, "conflicting methods")
			}

			if isUnsigned || isSigned {
				return errorf(i, "width modifier specified after sign modifier")
			}

			isWidth = true
			continue
		case '0':
			if !isZero && !isWidth && size == 0 && minWidth == 0 {
				isZero = true
				continue
			}

			fallthrough
		case '1', '2', '3', '4', '5', '6', '7', '8', '9':
			if isUnsigned || isSigned || isMemory {
				size = 10*size + int(r-'0')
				continue
			}

			if isWidth || isZero {
				minWidth = 10*minWidth + int(r-'0')
				continue
			}

			return errorf(i, "unprefixed number modifier")
		}

		// Integers

		if r == 'b' || r == 'o' || r == 'd' || (r == 'x' && !isMemory) || (r == 'X' && !isMemory) {
			switch {
			case isMemory:
				return errorf(i, "memory modifier used with integer")
			case addSpace:
				return errorf(i, "space modifier used with integer")
			}

			switch size {
			case 8, 16, 32, 64:
			case 0:
				return errorf(i, "missing integer width")
			default:
				return errorf(i, "invalid integer width %d", size)
			}

			var base int
			switch r {
			case 'b':
				base = 2
			case 'o':
				base = 8
			case 'd':
				base = 10
			case 'x', 'X':
				base = 16
			}

			verb := printkVerb{
				Text:          format[verbStart : i+1],
				Integer:       true,
				Unsigned:      isUnsigned,
				Signed:        isSigned,
				SpacePrefixed: isWidth,
				ZeroPrefixed:  isZero,
				UpperCase:     r == 'X',
				Base:          base,
				ArgWidth:      size,
				MinWidth:      minWidth,
			}

			verbs = append(verbs, verb)

			reset()
			continue
		}

		// Characters

		if r == 'c' {
			switch {
			case isUnsigned:
				return errorf(i, "unsigned modifier used with character")
			case isSigned:
				return errorf(i, "signed modifier used with character")
			case isMemory:
				return errorf(i, "memory modifier used with character")
			case addSpace:
				return errorf(i, "space modifier used with character")
			case isWidth:
				return errorf(i, "width modifier used with character")
			case isZero:
				return errorf(i, "zero modifier used with character")
			}

			verb := printkVerb{
				Text:      format[verbStart : i+1],
				Character: true,
			}

			verbs = append(verbs, verb)

			reset()
			continue
		}

		// Strings

		if r == 's' {
			switch {
			case isUnsigned:
				return errorf(i, "unsigned modifier used with string")
			case isSigned:
				return errorf(i, "signed modifier used with string")
			case addSpace:
				return errorf(i, "space modifier used with string")
			case isZero:
				return errorf(i, "zero modifier used with string")
			}

			verb := printkVerb{
				Text:          format[verbStart : i+1],
				String:        true,
				Memory:        isMemory,
				SpacePrefixed: isWidth,
				ArgWidth:      size,
				MinWidth:      minWidth,
			}

			verbs = append(verbs, verb)

			reset()
			continue
		}

		// Buffers

		if r == 'x' || r == 'X' {
			switch {
			case isWidth:
				return errorf(i, "width modifier used with buffer")
			case isZero:
				return errorf(i, "zero modifier used with buffer")
			}

			verb := printkVerb{
				Text:           format[verbStart : i+1],
				Buffer:         true,
				Memory:         isMemory,
				SpaceSeparated: addSpace,
				UpperCase:      r == 'X',
				ArgWidth:       size,
			}

			verbs = append(verbs, verb)

			reset()
			continue
		}

		// Hexdump

		if r == 'h' {
			switch {
			case isUnsigned:
				return errorf(i, "unsigned modifier used with hexdump")
			case isSigned:
				return errorf(i, "signed modifier used with hexdump")
			case addSpace:
				return errorf(i, "space modifier used with hexdump")
			case isWidth:
				return errorf(i, "width modifier used with hexdump")
			case isZero:
				return errorf(i, "zero modifier used with hexdump")
			}

			verb := printkVerb{
				Text:     format[verbStart : i+1],
				Hexdump:  true,
				Memory:   isMemory,
				ArgWidth: size,
			}

			verbs = append(verbs, verb)

			reset()
			continue
		}

		// Pointer

		if r == 'p' {
			switch {
			case isUnsigned:
				return errorf(i, "unsigned modifier used with pointer")
			case isSigned:
				return errorf(i, "signed modifier used with pointer")
			case isMemory:
				return errorf(i, "memory modifier used with pointer")
			case addSpace:
				return errorf(i, "space modifier used with pointer")
			case isWidth:
				return errorf(i, "width modifier used with pointer")
			case isZero:
				return errorf(i, "zero modifier used with pointer")
			}

			verb := printkVerb{
				Text:    format[verbStart : i+1],
				Pointer: true,
			}

			verbs = append(verbs, verb)

			reset()
			continue
		}

		return errorf(i, "unrecognised verb %q", r)
	}

	if inVerb {
		return errorf(len(format)-1, "unterminated verb")
	}

	return verbs, nil
}
