// Command kcheck performs static analysis on firefly kernel C code.
package main

import (
	"flag"
	"log"
	"os"
	"path/filepath"
	"sort"
)

func init() {
	log.SetFlags(0)
}

var failed bool

func errorf(format string, v ...interface{}) {
	log.Printf(format, v...)
	failed = true
}

type command struct {
	flags *flag.FlagSet
	desc  string
	cmd   func(args []string)
}

var commands []command

func registerCommand(flags *flag.FlagSet, description string, cmd func(args []string)) {
	name := flags.Name()
	for _, cmd := range commands {
		if cmd.flags.Name() == name {
			panic("duplicate command: " + name)
		}
	}

	commands = append(commands, command{flags: flags, desc: description, cmd: cmd})
}

func usage() {
	log.Printf("Usage:\n  %s COMMAND [ARGS...]\n\n", filepath.Base(os.Args[0]))
	log.Printf("Commands:")
	for _, cmd := range commands {
		log.Printf("\t%s %s", cmd.flags.Name(), cmd.desc)
		cmd.flags.PrintDefaults()
	}

	os.Exit(2)
}

func main() {
	sort.Slice(commands, func(i, j int) bool { return commands[i].flags.Name() < commands[j].flags.Name() })

	if len(os.Args) == 1 {
		usage()
	}

	for _, cmd := range commands {
		if os.Args[1] != cmd.flags.Name() {
			continue
		}

		cmd.flags.Parse(os.Args[2:])
		cmd.cmd(cmd.flags.Args())
		if failed {
			os.Exit(1)
		}

		return
	}

	usage()
}
