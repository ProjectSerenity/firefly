// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package x86

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func fieldNames(typ reflect.Type) []string {
	fields := make([]string, typ.NumField())
	for i := range fields {
		fields[i] = typ.Field(i).Name
	}

	return fields
}

func TestJSON(t *testing.T) {
	// Make sure that all types we encode
	// to JSON remain synchronised as fields
	// are added and removed.
	tests := []struct {
		Name string
		Base any
		JSON any
	}{
		{
			Name: "Encoding",
			Base: Encoding{},
			JSON: jsonEncoding{},
		},
	}

	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			base := reflect.TypeOf(test.Base)
			json := reflect.TypeOf(test.JSON)
			baseFields := fieldNames(base)
			jsonFields := fieldNames(json)

			if diff := cmp.Diff(jsonFields, baseFields); diff != "" {
				t.Fatalf("%s: (-json, +base)\n%s", test.Name, diff)
			}
		})
	}
}
