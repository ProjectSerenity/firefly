// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command ruse implements the Ruse language's tooling.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"

	"firefly-os.dev/tools/ruse/cmd/compile"
	"firefly-os.dev/tools/ruse/cmd/link"
	"firefly-os.dev/tools/ruse/cmd/rpkg"
)

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix("")
}

type Command struct {
	Name        string
	Description string
	Func        func(ctx context.Context, w io.Writer, args []string) error
}

var (
	commandsNames = make([]string, 0, 10)
	commandsMap   = make(map[string]*Command)

	program = filepath.Base(os.Args[0])
)

func RegisterCommand(name, description string, fun func(ctx context.Context, w io.Writer, args []string) error) {
	if commandsMap[name] != nil {
		panic("command " + name + " already registered")
	}

	if fun == nil {
		panic("command " + name + " registered with nil implementation")
	}

	commandsNames = append(commandsNames, name)
	commandsMap[name] = &Command{Name: name, Description: description, Func: fun}
}

func init() {
	RegisterCommand("compile", "Compile a Ruse package into an rpkg file", compile.Main)
	RegisterCommand("link", "Link one or more Ruse packages into an executable binary", link.Main)
	RegisterCommand("rpkg", "Print debug information about a Ruse package", rpkg.Main)
}

func main() {
	sort.Strings(commandsNames)

	var help bool
	flag.BoolVar(&help, "h", false, "Show this message and exit.")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage\n  %s COMMAND [OPTIONS]\n\n", program)
		fmt.Fprintf(os.Stderr, "Commands:\n")
		maxWidth := 0
		for _, name := range commandsNames {
			if maxWidth < len(name) {
				maxWidth = len(name)
			}
		}

		for _, name := range commandsNames {
			cmd := commandsMap[name]
			fmt.Fprintf(os.Stderr, "  %-*s  %s\n", maxWidth, name, cmd.Description)
		}

		os.Exit(2)
	}

	flag.Parse()

	args := flag.Args()
	if help {
		flag.Usage()
	}

	if len(args) == 0 {
		flag.Usage()
	}

	name := args[0]
	cmd, ok := commandsMap[args[0]]
	if !ok {
		flag.Usage()
	}

	log.SetPrefix(name + ": ")
	err := cmd.Func(context.Background(), os.Stdout, args[1:])
	if err != nil {
		log.Fatal(err)
	}
}
