// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package types

import (
	"fmt"
	"strconv"

	"firefly-os.dev/tools/ruse/ast"
	"firefly-os.dev/tools/ruse/binary"
	"firefly-os.dev/tools/ruse/token"
)

// A Section represents a contiguous region
// of memory in a compiled Ruse binary.
type Section struct {
	section   *binary.Section
	fixedAddr bool
}

var _ Type = Section{}

func NewSection(section *binary.Section, fixedAddr bool) Section {
	return Section{section: section, fixedAddr: fixedAddr}
}

func NewRawSection(name, fixedAddr *ast.Literal, permissions *ast.Identifier) (Section, error) {
	// Check the name.
	if name == nil {
		return Section{}, fmt.Errorf("invalid section: name missing")
	}
	if name.Kind != token.String {
		return Section{}, fmt.Errorf("invalid section: got %s name, want string", name.Kind)
	}
	parsedName, err := strconv.Unquote(name.Value)
	if err != nil {
		return Section{}, fmt.Errorf("invalid section: got bad name %s: %v", name.Value, err)
	}

	// Check any fixed address.
	if fixedAddr != nil && fixedAddr.Kind != token.Integer {
		return Section{}, fmt.Errorf("invalid section: got %s fixed-address, want integer", fixedAddr.Kind)
	}
	address := uintptr(0)
	if fixedAddr != nil {
		fixed, err := strconv.ParseUint(fixedAddr.Value, 0, 64)
		if err != nil {
			return Section{}, fmt.Errorf("invalid section: invalid fixed-address %q: %v", fixedAddr.Value, err)
		}

		address = uintptr(fixed)
	}

	// Check the permissions.
	if permissions == nil {
		return Section{}, fmt.Errorf("invalid section: permissions missing")
	}
	if len(permissions.Name) != 3 ||
		(permissions.Name[0] != 'r' && permissions.Name[0] != 'R' && permissions.Name[0] != '_') ||
		(permissions.Name[1] != 'w' && permissions.Name[1] != 'W' && permissions.Name[1] != '_') ||
		(permissions.Name[2] != 'x' && permissions.Name[2] != 'X' && permissions.Name[2] != '_') {
		return Section{}, fmt.Errorf("invalid section: got permissions %q, want form rwx (_ to drop a permission)", permissions.Name)
	}
	parsedPermissions := binary.Permissions(0)
	if permissions.Name[0] != '_' {
		parsedPermissions |= binary.Read
	}
	if permissions.Name[1] != '_' {
		parsedPermissions |= binary.Write
	}
	if permissions.Name[2] != '_' {
		parsedPermissions |= binary.Execute
	}

	section := Section{
		section: &binary.Section{
			Name:        parsedName,
			Address:     address,
			Permissions: parsedPermissions,
		},
		fixedAddr: fixedAddr != nil,
	}

	return section, nil
}

func (s Section) Section() *binary.Section { return s.section }
func (s Section) FixedAddr() bool          { return s.fixedAddr }
func (s Section) Underlying() Type         { return s }
func (s Section) String() string           { return "section" }
