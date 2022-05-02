// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package token

import (
	"encoding/json"
	"testing"
)

func TestPosition(t *testing.T) {
	tests := []struct {
		name   string
		offset int
		line   int
		column int
		error  error
	}{
		{
			name:   "minimal",
			offset: 0,
			line:   1,
			column: 1,
		},
		{
			name:   "simple",
			offset: 1,
			line:   2,
			column: 3,
		},
		{
			name:   "max",
			offset: 0xffff_ffff,
			line:   0xffff,
			column: 0xffff,
		},
		{
			name:   "small offset",
			offset: -1,
			line:   0xffff,
			column: 0xffff,
			error:  invalidOffset,
		},
		{
			name:   "big offset",
			offset: 0x1_0000_0000,
			line:   0xffff,
			column: 0xffff,
			error:  invalidOffset,
		},
		{
			name:   "small line",
			offset: 0xffff_ffff,
			line:   0,
			column: 0xffff,
			error:  invalidLine,
		},
		{
			name:   "big line",
			offset: 0xffff_ffff,
			line:   0x1_0000,
			column: 0xffff,
			error:  invalidLine,
		},
		{
			name:   "small column",
			offset: 0xffff_ffff,
			line:   0xffff,
			column: 0,
			error:  invalidColumn,
		},
		{
			name:   "big column",
			offset: 0xffff_ffff,
			line:   0xffff,
			column: 0x1_0000,
			error:  invalidColumn,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pos, err := NewPosition(test.offset, test.line, test.column)
			if test.error != nil {
				if err != test.error {
					t.Fatalf("NewPosition(%d, %d, %d): got error %#v, want %#v", test.offset, test.line, test.column, err, test.error)
				}

				return
			}

			if err != nil {
				t.Fatalf("NewPosition(%d, %d, %d): got unexpected error %#v", test.offset, test.line, test.column, err)
			}

			if !pos.IsValid() {
				t.Fatalf("NewPosition(%d, %d, %d): got not valid", test.offset, test.line, test.column)
			}

			if got := pos.Offset(); got != test.offset {
				t.Fatalf("NewPosition(%d, %d, %d): got offset %d, want %d", test.offset, test.line, test.column, got, test.offset)
			}

			if got := pos.Line(); got != test.line {
				t.Fatalf("NewPosition(%d, %d, %d): got line %d, want %d", test.offset, test.line, test.column, got, test.line)
			}

			if got := pos.Column(); got != test.column {
				t.Fatalf("NewPosition(%d, %d, %d): got column %d, want %d", test.offset, test.line, test.column, got, test.column)
			}
		})
	}
}

func TestPosition_Advance(t *testing.T) {
	tests := []struct {
		offset  int
		line    int
		column  int
		advance int
	}{
		{
			offset:  0,
			line:    1,
			column:  1,
			advance: 0,
		},
		{
			offset:  0,
			line:    1,
			column:  1,
			advance: 1,
		},
		{
			offset:  0,
			line:    1,
			column:  1,
			advance: 10,
		},
		{
			offset:  15,
			line:    1,
			column:  16,
			advance: 10,
		},
		{
			offset:  15,
			line:    2,
			column:  6,
			advance: 10,
		},
	}

	for _, test := range tests {
		pos, err := NewPosition(test.offset, test.line, test.column)
		if err != nil {
			t.Errorf("Position(offset=%d, line=%d, column=%d): %v", test.offset, test.line, test.column, err)
			continue
		}

		got := pos.Advance(test.advance)
		if got.Offset() != pos.Offset()+test.advance {
			t.Errorf("Position(offset=%d, line=%d, column=%d).Advance(%d): got offset %d, want %d+%d=%d",
				test.offset, test.line, test.column,
				test.advance,
				got.Offset(), pos.Offset(), test.advance, pos.Offset()+test.advance)
		}

		if got.Line() != pos.Line() {
			t.Errorf("Position(offset=%d, line=%d, column=%d).Advance(%d): got line %d, want %d",
				test.offset, test.line, test.column,
				test.advance,
				got.Line(), pos.Line())
		}

		if got.Column() != pos.Column()+test.advance {
			t.Errorf("Position(offset=%d, line=%d, column=%d).Advance(%d): got column %d, want %d+%d=%d",
				test.offset, test.line, test.column,
				test.advance,
				got.Column(), pos.Column(), test.advance, pos.Column()+test.advance)
		}
	}
}

func TestPosition_File(t *testing.T) {
	tests := []struct {
		name   string
		line   Position
		column Position
		file   string
		want   string
		wantS  string
		wantGS string
	}{
		{
			name:   "full",
			line:   1,
			column: 2,
			file:   "foo.plan",
			want:   "foo.plan:1:2",
			wantS:  "1:2",
			wantGS: "token.Position{Offset: 0, Line: 1, Column: 2}",
		},
		{
			name:   "no_column",
			line:   1,
			column: 0,
			file:   "foo.plan",
			want:   "foo.plan:1",
			wantS:  "1",
			wantGS: "token.Position{Offset: 0, Line: 1, Column: 0}",
		},
		{
			name:   "no_file",
			line:   1,
			column: 2,
			file:   "",
			want:   "1:2",
			wantS:  "1:2",
			wantGS: "token.Position{Offset: 0, Line: 1, Column: 2}",
		},
		{
			name:   "no_file_or_column",
			line:   1,
			column: 0,
			file:   "",
			want:   "1",
			wantS:  "1",
			wantGS: "token.Position{Offset: 0, Line: 1, Column: 0}",
		},
		{
			name:   "no_position",
			line:   0,
			column: 0,
			file:   "foo.plan",
			want:   "foo.plan",
			wantS:  "?",
			wantGS: "token.Position{Offset: 0, Line: 0, Column: 0}",
		},
		{
			name:   "invalid",
			line:   0,
			column: 0,
			file:   "",
			want:   "?",
			wantS:  "?",
			wantGS: "token.Position{Offset: 0, Line: 0, Column: 0}",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pos := test.line<<lineShift | test.column<<columnShift
			got := pos.File(test.file)
			if got != test.want {
				t.Fatalf("Position{line: %d, column: %d}.File(%q):\n  Got  %q\n  Want %q", test.line, test.column, test.file, got, test.want)
			}

			got = pos.String()
			if got != test.wantS {
				t.Fatalf("Position(line: %d, column: %d}.String():\n  Got  %q\n  Want %q", test.line, test.column, got, test.wantS)
			}

			got = pos.GoString()
			if got != test.wantGS {
				t.Fatalf("Position(line: %d, column: %d}.GoString():\n  Got  %q\n  Want %q", test.line, test.column, got, test.wantGS)
			}
		})
	}
}

func TestPosition_MarshalJSON(t *testing.T) {
	type jsonPosition struct {
		Offset int `json:"offset"`
		Line   int `json:"line"`
		Column int `json:"column"`
	}

	tests := []struct {
		offset int
		line   int
		column int
	}{
		{
			offset: 0,
			line:   1,
			column: 1,
		},
		{
			offset: 7,
			line:   3,
			column: 2,
		},
	}

	for _, test := range tests {
		pos, err := NewPosition(test.offset, test.line, test.column)
		if err != nil {
			t.Errorf("Position{offset: %d, line: %d, column: %d}: got error %v", test.offset, test.line, test.column, err)
			continue
		}

		data, err := json.Marshal(pos)
		if err != nil {
			t.Errorf("json.Marshal(%#v): unexpected error: %v", pos, err)
			continue
		}

		var jpos jsonPosition
		err = json.Unmarshal(data, &jpos)
		if err != nil {
			t.Errorf("json.Unmarshal(%q): unexpected error: %v", data, err)
			continue
		}

		if jpos.Offset != test.offset {
			t.Errorf("json.Unmarshal(%q): got offset %d, want %d", data, jpos.Offset, test.offset)
			continue
		}

		if jpos.Line != test.line {
			t.Errorf("json.Unmarshal(%q): got line %d, want %d", data, jpos.Line, test.line)
			continue
		}

		if jpos.Column != test.column {
			t.Errorf("json.Unmarshal(%q): got column %d, want %d", data, jpos.Column, test.column)
			continue
		}
	}
}
