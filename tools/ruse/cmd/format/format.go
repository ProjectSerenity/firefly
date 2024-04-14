// Copyright 2024 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package format rewrites Ruse source code to standard formatting.
package format

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"firefly-os.dev/tools/diff"
	"firefly-os.dev/tools/ruse/format"
	"firefly-os.dev/tools/ruse/parser"
	"firefly-os.dev/tools/ruse/token"
)

var program = filepath.Base(os.Args[0])

// Main formats a set of Ruse source files.
func Main(ctx context.Context, w io.Writer, args []string) error {
	flags := flag.NewFlagSet("format", flag.ExitOnError)

	var help, printDiffs, listDiffs, writeFormatted bool
	flags.BoolVar(&help, "h", false, "Show this message and exit.")
	flags.BoolVar(&printDiffs, "diff", false, "Print diffs instead of rewriting files.")
	flags.BoolVar(&listDiffs, "list", false, "Print the files whose formatting is incorrect.")
	flags.BoolVar(&writeFormatted, "fix", false, "Write the formatted source code to the original file instead of printing.")

	flags.Usage = func() {
		log.Printf("Usage:\n  %s %s OPTIONS FILE...\n\n", program, flags.Name())
		flags.PrintDefaults()
		os.Exit(2)
	}

	err := flags.Parse(args)
	if err != nil || help {
		flags.Usage()
	}

	filenames := flags.Args()
	if len(filenames) == 0 {
		flags.Usage()
	}

	if printDiffs && listDiffs {
		log.Printf("Flags -diff and -list are incompatible")
		flags.Usage()
	}

	if printDiffs && writeFormatted {
		log.Printf("Flags -diff and -fix are incompatible")
		flags.Usage()
	}
	if listDiffs && writeFormatted {
		log.Printf("Flags -list and -fix are incompatible")
		flags.Usage()
	}

	fset := token.NewFileSet()
	var buf bytes.Buffer
	for _, filename := range filenames {
		source, err := os.ReadFile(filename)
		if err != nil {
			return fmt.Errorf("failed to read %s: %v", filename, err)
		}

		file, err := parser.ParseFile(fset, filename, source, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %v", filename, err)
		}

		buf.Reset()
		var out io.Writer

		// The default behaviour is to print the formatted
		// code to stdout.
		out = w

		// If we want to build diffs, we need to keep the
		// formatted code.
		if printDiffs || listDiffs || writeFormatted {
			out = &buf
		}

		err = format.Fprint(out, fset, file)
		if err != nil {
			return fmt.Errorf("failed to format %s: %v", filename, err)
		}

		if writeFormatted {
			info, err := os.Stat(filename)
			if err != nil {
				return fmt.Errorf("failed to stat %s: %v", filename, err)
			}

			err = os.WriteFile(filename, buf.Bytes(), info.Mode())
			if err != nil {
				return fmt.Errorf("failed to write %s: %v", filename, err)
			}
		}

		if printDiffs || listDiffs {
			got := diff.Diff(filename+".orig", source, filename, buf.Bytes())
			if listDiffs {
				if len(got) != 0 {
					fmt.Fprintln(w, filename)
				}
			} else if printDiffs {
				_, err = w.Write(got)
				if err != nil {
					return fmt.Errorf("failed to diff %s: %v", filename, err)
				}
			}
		}
	}

	return nil
}
