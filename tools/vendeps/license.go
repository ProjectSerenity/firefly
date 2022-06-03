// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package vendeps

import (
	"strings"
	"unicode"
)

// Licenses is the set of acceptable software
// licenses, referenced by their SPDX id.
//
var Licenses = map[string]string{
	"0BSD":         "BSD Zero Clause License",
	"Apache-2.0":   "Apache License 2.0",
	"BSD-2-Clause": "BSD 2-Clause \"Simplified\" License",
	"BSD-3-Clause": "BSD 3-Clause \"New\" or \"Revised\" License",
	"MIT":          "MIT License",
}

// AcceptableLicense determines whether the
// given set of licenses includes at least
// one acceptable license as described above.
//
func AcceptableLicense(options string) (license string, ok bool) {
	licenses := strings.FieldsFunc(options, func(r rune) bool {
		return r == '/' || unicode.IsSpace(r)
	})

	for _, license := range licenses {
		match := Licenses[license]
		if match != "" {
			return match, true
		}
	}

	return "", false
}
