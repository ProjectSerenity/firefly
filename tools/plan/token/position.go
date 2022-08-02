// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package token

import (
	"errors"
	"fmt"
	"strconv"
)

// Position describes an arbitrary source location
// within a Plan file.
type Position uint64

// FileStart records the first position in a file.
const FileStart = Position(1<<lineShift) | Position(1<<columnShift)

// Position is encoded as a 16-bit line number,
// a 16-bit column number, and a 32-bit offset
// into the file. A position is valid if it has
// a non-zero line number.
const (
	lineShift = 16 + 32
	lineMax   = 0xffff
	lineMask  = lineMax << lineShift

	columnShift = 32
	columnMax   = 0xffff
	columnMask  = columnMax << columnShift

	offsetShift = 0
	offsetMax   = 0xffff_ffff
	offsetMask  = offsetMax << offsetShift
)

// MaxOffset defines the largest offset into a file
// that can be represented in a Position.
const MaxOffset = offsetMax

var (
	invalidOffset = errors.New("invalid file offset")
	invalidLine   = errors.New("invalid line number")
	invalidColumn = errors.New("invalid column number")
)

// NewPosition returns a compact representation
// for the given position.
func NewPosition(offset, line, column int) (Position, error) {
	if offset < 0 || offsetMax < offset {
		return 0, invalidOffset
	}

	if line < 1 || lineMax < line {
		return 0, invalidLine
	}

	if column < 1 || columnMax < column {
		return 0, invalidColumn
	}

	p := Position(offset)<<offsetShift |
		Position(line)<<lineShift |
		Position(column)<<columnShift

	return p, nil
}

// IsValid returns whether p is a valid position.
func (p Position) IsValid() bool {
	// A position is valid if it has a non-zero
	// line number.
	return p&lineMask != 0
}

// Line returns the line number for this position.
//
// Line numbers start from 1.
func (p Position) Line() int {
	return int((p & lineMask) >> lineShift)
}

// Column returns the column number for this position.
//
// Column numbers start from 1.
func (p Position) Column() int {
	return int((p & columnMask) >> columnShift)
}

// Offset returns the file offset for this position.
//
// Offset numbers start from 0.
func (p Position) Offset() int {
	return int(p & offsetMask)
}

// Advance returns a new position that represents
// n byte further into the file on the same line
// as p.
func (p Position) Advance(n int) Position {
	offset := p.Offset() + n
	line := p.Line()
	column := p.Column() + n
	p, err := NewPosition(offset, line, column)
	if err != nil {
		panic(err)
	}

	return p
}

// File describes this position within the given
// file, with one of the following forms:
//
//	file:line:column  (Valid position within the file)
//	file:line         (Valid position with column 0)
//	line:column       (Valid position with filename "")
//	line              (Valid position with filename "" and column 0)
//	file              (Invalid position)
//	?                 (Invalid position with filename "")
func (p Position) File(filename string) string {
	line := p.Line()
	col := p.Column()

	// Handle invalid positions first.
	if line == 0 {
		if filename != "" {
			return filename
		}

		return "?"
	}

	// Handle an empty filename next.
	if filename == "" {
		if col == 0 {
			return strconv.Itoa(line)
		}

		return strconv.Itoa(line) + ":" + strconv.Itoa(col)
	}

	if col == 0 {
		return filename + ":" + strconv.Itoa(line)
	}

	return filename + ":" + strconv.Itoa(line) + ":" + strconv.Itoa(col)
}

// String describes this position with no filename,
// with one of the following forms:
//
//	line:column       (Valid position)
//	line              (Valid position with column 0)
//	?                 (Invalid position)
func (p Position) String() string {
	line := p.Line()
	col := p.Column()

	// Handle invalid positions first.
	if line == 0 {
		return "?"
	}

	// Handle an empty column next.
	if col == 0 {
		return strconv.Itoa(line)
	}

	return strconv.Itoa(line) + ":" + strconv.Itoa(col)
}

func (p Position) GoString() string {
	return fmt.Sprintf("token.Position{Offset: %d, Line: %d, Column: %d}", p.Offset(), p.Line(), p.Column())
}

func (p Position) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`{"offset":%d,"line":%d,"column":%d}`, p.Offset(), p.Line(), p.Column())), nil
}
