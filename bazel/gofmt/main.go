// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
)

func main() {
	var (
		gofmt string
		out   string
	)

	flag.StringVar(&gofmt, "gofmt", "", "Path to the gofmt binary, which must be executable.")
	flag.StringVar(&out, "out", "", "Path to where the output should be written.")

	flag.Parse()

	if gofmt == "" {
		fmt.Fprintln(os.Stderr, "No gofmt tool provided.")
		os.Exit(2)
	}

	if out == "" {
		fmt.Fprintln(os.Stderr, "No output path provided.")
		os.Exit(2)
	}

	files := flag.Args()
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "No sources provided to format.")
		os.Exit(2)
	}

	args := make([]string, 2, 2+len(files))
	args[0] = "-s" // Simplify.
	args[1] = "-l" // List unformatted files.
	args = append(args, files...)

	f, err := os.Create(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output file %q: %v\n", out, err)
		os.Exit(1)
	}

	var buf bytes.Buffer
	w := io.MultiWriter(f, &buf)
	cmd := exec.Command(gofmt, args...)
	cmd.Stdout = w
	cmd.Stderr = w
	err = cmd.Run()
	if err != nil {
		os.Stderr.Write(buf.Bytes())
		fmt.Fprintf(os.Stderr, "Failed to run gofmt: %v\n", err)
		os.Exit(1)
	}

	if len(bytes.TrimSpace(buf.Bytes())) > 0 {
		// There was a diff.
		fmt.Fprintln(os.Stderr, "The following files are not correctly formatted:")
		os.Stderr.Write(buf.Bytes())
		os.Exit(1)
	}

	err = f.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to close output file: %v\n", err)
		os.Exit(1)
	}
}
