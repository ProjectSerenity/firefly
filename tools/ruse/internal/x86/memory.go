// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package x86

import (
	"fmt"
	"strings"
)

// Memory represents an x86 memory
// reference.
type Memory struct {
	Segment      *Register
	Base         *Register
	Index        *Register
	Scale        uint8
	Displacement int64
}

func (m *Memory) String() string {
	// See https://blog.yossarian.net/2020/06/13/How-x86-addresses-memory
	segment := m.Segment != nil
	base := m.Base != nil
	index := m.Index != nil
	scale := m.Scale != 0
	displacement := m.Displacement != 0
	switch {
	case segment && base && index && scale && displacement:
		return fmt.Sprintf("(+ %s %s (* %s %d) %d)", m.Segment, m.Base, m.Index, m.Scale, m.Displacement)
	case base && index && scale && displacement:
		return fmt.Sprintf("(+ %s (* %s %d) %d)", m.Base, m.Index, m.Scale, m.Displacement)
	case segment && index && scale && displacement:
		return fmt.Sprintf("(+ %s (* %s %d) %d)", m.Segment, m.Index, m.Scale, m.Displacement)
	case index && scale && displacement:
		return fmt.Sprintf("(+ (* %s %d) %d)", m.Index, m.Scale, m.Displacement)
	case segment && base && index && scale:
		return fmt.Sprintf("(+ %s %s (* %s %d))", m.Segment, m.Base, m.Index, m.Scale)
	case base && index && scale:
		return fmt.Sprintf("(+ %s (* %s %d))", m.Base, m.Index, m.Scale)
	case segment && base && index && displacement:
		return fmt.Sprintf("(+ %s %s %s %d)", m.Segment, m.Base, m.Index, m.Displacement)
	case base && index && displacement:
		return fmt.Sprintf("(+ %s %s %d)", m.Base, m.Index, m.Displacement)
	case segment && base && displacement:
		return fmt.Sprintf("(+ %s %s %d)", m.Segment, m.Base, m.Displacement)
	case base && displacement:
		return fmt.Sprintf("(+ %s %d)", m.Base, m.Displacement)
	case segment && base && index:
		return fmt.Sprintf("(+ %s %s %s)", m.Segment, m.Base, m.Index)
	case base && index:
		return fmt.Sprintf("(+ %s %s)", m.Base, m.Index)
	case segment && base:
		return fmt.Sprintf("(%s %s)", m.Segment, m.Base)
	case base:
		return fmt.Sprintf("(%s)", m.Base)
	case segment && displacement:
		return fmt.Sprintf("(%s %d)", m.Segment, m.Displacement)
	default:
		// Zero displacement.
		fallthrough
	case displacement:
		return fmt.Sprintf("(%d)", m.Displacement)
	}
}

func (m *Memory) GoString() string {
	first := true
	var s strings.Builder
	join := func() {
		if !first {
			s.WriteString(", ")
		}

		first = false
	}

	s.WriteByte('{')
	if m.Segment != nil {
		first = false
		fmt.Fprintf(&s, "Segment: %s", m.Segment)
	}
	if m.Base != nil {
		join()
		fmt.Fprintf(&s, "Base: %s", m.Base)
	}
	if m.Index != nil {
		join()
		fmt.Fprintf(&s, "Index: %s", m.Index)
	}
	if m.Scale != 0 {
		join()
		fmt.Fprintf(&s, "Scale: %d", m.Scale)
	}
	if m.Displacement != 0 || first {
		join()
		fmt.Fprintf(&s, "Displacement: %#x", m.Displacement)
	}
	s.WriteByte('}')

	return s.String()
}
