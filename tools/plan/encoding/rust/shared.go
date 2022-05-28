// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package rust

import (
	"fmt"
	"strings"

	"firefly-os.dev/tools/plan/types"
)

const (
	enumerationTemplate = "enumeration_rs.txt"
	bitfieldTemplate    = "bitfield_rs.txt"
	structureTemplate   = "structure_rs.txt"
)

func sharedAddOne(i int) int {
	return i + 1
}

func sharedErrorEnumeration(s *types.Syscall) *types.Enumeration {
	return s.Results[len(s.Results)-1].Type.(*types.Reference).Underlying.(*types.Enumeration)
}

func sharedFromU64(t types.Type) string {
	t = types.Underlying(t)
	if integer, ok := t.(types.Integer); ok && integer == types.Uint64 {
		return ""
	}

	return fmt.Sprintf(" as %s", sharedToString(t))
}

func sharedIsEnumeration(typ types.Type) bool {
	_, ok := types.Underlying(typ).(*types.Enumeration)
	return ok
}

func sharedIsInteger(typ types.Type) bool {
	_, ok := types.Underlying(typ).(types.Integer)
	return ok
}

func sharedIsPadding(typ types.Type) bool {
	_, ok := typ.(types.Padding)
	return ok
}

func sharedIsPointer(typ types.Type) bool {
	_, ok := typ.(*types.Pointer)
	return ok
}

func sharedIsSigned(typ types.Type) bool {
	if integer, ok := types.Underlying(typ).(types.Integer); ok {
		switch integer {
		case types.Sint8, types.Sint16, types.Sint32, types.Sint64:
			return true
		}
	}

	return false
}

func sharedIsSint64(typ types.Type) bool {
	if integer, ok := types.Underlying(typ).(types.Integer); ok && integer == types.Sint64 {
		return true
	}

	return false
}

func sharedIsUint64(typ types.Type) bool {
	if integer, ok := types.Underlying(typ).(types.Integer); ok && integer == types.Uint64 {
		return true
	}

	return false
}

func sharedOneResult(s *types.Syscall) bool {
	return len(s.Results) == 1
}

func sharedParamNamesAndTypes(p types.Parameters) string {
	if len(p) == 0 {
		return ""
	}

	names := make([]string, len(p))
	for i, param := range p {
		names[i] = param.Name.SnakeCase() + ": " + sharedToString(p[i].Type)
	}

	return strings.Join(names, ", ")
}

func sharedParamTypes(p types.Parameters) string {
	if len(p) == 0 {
		return ""
	}

	names := make([]string, len(p))
	for i, param := range p {
		names[i] = sharedToString(param.Type)
	}

	return strings.Join(names, ", ")
}

func sharedToString(t types.Type) string {
	switch t := types.Underlying(t).(type) {
	case types.Integer:
		ss := map[types.Integer]string{
			types.Byte:   "u8",
			types.Uint8:  "u8",
			types.Uint16: "u16",
			types.Uint32: "u32",
			types.Uint64: "u64",
			types.Sint8:  "i8",
			types.Sint16: "i16",
			types.Sint32: "i32",
			types.Sint64: "i64",
		}

		s, ok := ss[t]
		if !ok {
			panic(fmt.Sprintf("unrecognised integer type %d", t))
		}

		return s
	case *types.Pointer:
		if t.Mutable {
			return "*mut " + sharedToString(t.Underlying)
		} else {
			return "*const " + sharedToString(t.Underlying)
		}
	case types.Padding:
		return fmt.Sprintf("[u8; %d]", t)
	case *types.Enumeration:
		return t.Name.PascalCase()
	case *types.Bitfield:
		return t.Name.PascalCase()
	case *types.Structure:
		return t.Name.PascalCase()
	case *types.SyscallReference:
		return t.Name.SnakeCase()
	default:
		panic(fmt.Sprintf("sharedToString(%T): unexpected type", t))
	}
}

func sharedToU64(t types.Type) string {
	if integer, ok := types.Underlying(t).(types.Integer); ok && integer == types.Uint64 {
		return ""
	}

	return " as u64"
}
