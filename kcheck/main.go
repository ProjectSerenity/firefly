// Command kcheck performs static analysis on firefly kernel C code.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/ProjectSerenity/firefly/cc"
)

func init() {
	log.SetFlags(0)
}

type Issue struct {
	Span  cc.Span
	Error error
}

func (i Issue) String() string {
	return fmt.Sprintf("%s (%d-%d): %s", i.Span, i.Span.Start.Byte, i.Span.End.Byte, i.Error)
}

var failed bool

func Errorf(c chan<- Issue, span cc.Span, format string, v ...interface{}) {
	err := fmt.Errorf(format, v...)
	issue := Issue{Span: span, Error: err}
	c <- issue
}

type Command func(issues chan<- Issue, args []string)

type command struct {
	flags *flag.FlagSet
	desc  string
	cmd   Command
}

var commands []command

func registerCommand(flags *flag.FlagSet, description string, cmd Command) {
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

	issues := make(chan Issue)
	for _, cmd := range commands {
		if os.Args[1] != cmd.flags.Name() {
			continue
		}

		done := make(chan struct{})
		firstTen := make([]Issue, 0, 10)
		go func() {
			for issue := range issues {
				if len(firstTen) < 10 {
					firstTen = append(firstTen, issue)
				}
			}

			close(done)
		}()

		cmd.flags.Parse(os.Args[2:])
		cmd.cmd(issues, cmd.flags.Args())
		close(issues)

		<-done
		for _, issue := range firstTen {
			log.Print(issue)
		}

		if len(firstTen) == 10 {
			log.Printf("Giving up after 10 issues.")
		}

		if len(firstTen) > 0 {
			os.Exit(1)
		}

		return
	}

	usage()
}
