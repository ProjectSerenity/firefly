// Process C header files, enforcing several properties:
//
// 	- Header files cannot include other header files.
// 	- Header files must start with "#pragma once" and
// 	  be surrounded with include guards.
//

package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

var (
	headersFlags = flag.NewFlagSet("headers", flag.ExitOnError)
	headersDesc  = "[FILE...]"
)

func headersCommand(args []string) {
	if len(args) == 0 {
		log.Printf("No headers specified\n\n")
		usage()
	}

	for _, arg := range args {
		processHeader(arg)
	}
}

func init() {
	registerCommand(headersFlags, headersDesc, headersCommand)
}

const (
	pragmaOnce = "#pragma once"
	include    = "#include"
)

func mapHeaderName(r rune) rune {
	if 'a' <= r && r <= 'z' {
		return r + ('A' - 'a')
	}

	switch r {
	case ' ', '.':
		return '_'
	}

	return r
}

func processHeader(name string) {
	base := filepath.Base(name)
	f, err := os.Open(name)
	if err != nil {
		log.Fatalf("failed to open %q: %v", name, err)
	}

	defer f.Close()

	upperName := strings.Map(mapHeaderName, base)
	ifndef := fmt.Sprintf("#ifndef %s", upperName)
	define := fmt.Sprintf("#define %s", upperName)
	endif := fmt.Sprintf("#endif // %s", upperName)

	s := bufio.NewScanner(f)
	var finalLine string
	for line := 1; s.Scan(); line++ {
		text := s.Text()
		runes := []rune(text)

		if line == 1 && text != pragmaOnce {
			errorf("%s: missing %q", name, pragmaOnce)
		}

		if line == 2 && text != ifndef {
			errorf("%s: missing %q", name, ifndef)
		}

		if line == 3 && text != define {
			errorf("%s: missing %q", name, define)
		}

		if strings.Contains(text, include) {
			errorf("%s:%d: nested %s", name, line, include)
		}

		// Check indentation.
		for _, r := range runes {
			if !unicode.IsSpace(r) {
				break
			}

			if r != '\t' {
				errorf("%s:%d: non-tab indentation (%s)", name, line, strconv.QuoteRune(r))
				break
			}
		}

		// Check for trailing spaces.
		for i := len(runes); i > 0; i-- {
			r := runes[i-1]
			if !unicode.IsSpace(r) {
				break
			}

			errorf("%s:%d: trailing whitespace", name, line)
			break
		}

		finalLine = text
	}

	if err := s.Err(); err != nil {
		log.Fatalf("failed to parse %q: %v", name, err)
	}

	if finalLine != endif {
		errorf("%s: missing %q", name, endif)
	}
}
