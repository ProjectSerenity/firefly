// Process C files, enforcing several properties:
//
// 	- C files' use of printk is checked for the
// 	  correct number of args, with correct verb use.
//

package main

import (
	"flag"
	"log"
	"os"

	"github.com/ProjectSerenity/firefly/cc"
)

var (
	objectFlags = flag.NewFlagSet("object", flag.ExitOnError)
	objectDesc  = "FILE"
)

func objectCommand(issues chan<- Issue, args []string) {
	if len(args) == 0 {
		log.Printf("No object specified\n\n")
		usage()
	}

	for _, arg := range args {
		processObject(issues, arg)
	}
}

func init() {
	registerCommand(objectFlags, objectDesc, objectCommand)
}

const (
	buildIgnore = "// +build ignore"
)

func processObject(issues chan<- Issue, name string) {
	f, err := os.Open(name)
	if err != nil {
		log.Fatalf("failed to open %q: %v", name, err)
	}

	defer f.Close()

	prog, err := cc.Read(name, f)
	if err != nil {
		log.Fatalf("failed to parse %s: %v", name, err)
	}

	_ = prog
}
