// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"runtime"
)

type Error struct {
	Path string // The file:line of the Go code where the error originated.
	Page int    // The page number of the PDF where the error occurred.
	Err  string // The error message
}

func (err *Error) Error() string {
	if err.Page == 0 {
		return fmt.Sprintf("%s%s", err.Path, err.Err)
	}

	return fmt.Sprintf("%sp.%d: %s", err.Path, err.Page, err.Err)
}

func Errorf(page int, format string, v ...any) error {
	// First, we see whether we are
	// wrapping another of our Errors.
	if len(v) != 0 {
		last := v[len(v)-1]
		switch e := last.(type) {
		case *Error:
			v[len(v)-1] = e.Err
			return &Error{
				Path: e.Path,
				Page: e.Page,
				Err:  fmt.Sprintf(format, v...),
			}
		case error:
			var err *Error
			if errors.As(e, &err) {
				path := err.Path
				page := err.Page
				err.Path = ""
				err.Page = 0
				return &Error{
					Path: path,
					Page: page,
					Err:  fmt.Sprintf(format, v...),
				}
			}
		}
	}

	// Determine the location
	// ourself.
	var path string
	_, file, line, ok := runtime.Caller(1)
	if ok {
		path = fmt.Sprintf("%s:%d: ", file, line)
	}

	return &Error{
		Path: path,
		Page: page,
		Err:  fmt.Sprintf(format, v...),
	}
}
