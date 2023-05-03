// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package types

import (
	"strconv"
)

// BasicKind describes the kind of basic type.
type BasicKind int

const (
	KindInvalid BasicKind = iota // type is invalid

	// predeclared types
	KindBool
	KindInt
	KindInt8
	KindInt16
	KindInt32
	KindInt64
	KindUint
	KindUint8
	KindUint16
	KindUint32
	KindUint64
	KindUintptr
	KindString

	// types for untyped values
	KindUntypedBool
	KindUntypedInt
	KindUntypedString

	// aliases
	KindByte = KindUint8
)

// BasicInfo is a set of flags describing properties of a basic type.
type BasicInfo int

// Properties of basic types.
const (
	IsBoolean BasicInfo = 1 << iota
	IsInteger
	IsUnsigned
	IsString
	IsUntyped

	IsOrdered   = IsInteger | IsString
	IsConstType = IsBoolean | IsInteger | IsString
)

// A Basic represents a basic type.
type Basic struct {
	kind BasicKind
	info BasicInfo
	name string
}

var _ Type = (*Basic)(nil)

// Kind returns the kind of basic type b.
func (b *Basic) Kind() BasicKind { return b.kind }

// Info returns information about properties of basic type b.
func (b *Basic) Info() BasicInfo { return b.info }

// Name returns the name of basic type b.
func (b *Basic) Name() string { return b.name }

func (b *Basic) Underlying() Type { return b }
func (b *Basic) String() string   { return b.name }

// Bounds checking.
func (b *Basic) boundsCheck(v string) (ok bool) {
	var bits int
	var signed bool
	switch b.kind {
	case KindInt, KindInt8, KindInt16, KindInt32, KindInt64:
		signed = true
	case KindUint, KindUint8, KindUint16, KindUint32, KindUint64, KindUintptr:
		signed = false
	default:
		return false
	}

	switch b.kind {
	case KindInt, KindUint, KindUintptr:
		bits = 64 // TODO: correctly handle register-width types' size.
	case KindInt8, KindUint8:
		bits = 8
	case KindInt16, KindUint16:
		bits = 16
	case KindInt32, KindUint32:
		bits = 32
	case KindInt64, KindUint64:
		bits = 64
	}

	var err error
	if signed {
		_, err = strconv.ParseInt(v, 0, bits)
	} else {
		_, err = strconv.ParseUint(v, 0, bits)
	}

	return err == nil
}
