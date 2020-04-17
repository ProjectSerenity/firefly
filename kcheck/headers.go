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

	"github.com/ProjectSerenity/firefly/cc"
)

var (
	headersFlags = flag.NewFlagSet("headers", flag.ExitOnError)
	headersDesc  = "[FILE...]"
)

func headersCommand(issues chan<- Issue, args []string) {
	if len(args) == 0 {
		log.Printf("No headers specified\n\n")
		usage()
	}

	for _, arg := range args {
		processHeader(issues, arg)
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

func processHeader(issues chan<- Issue, name string) {
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
	var finalLineNum int
	for line := 1; s.Scan(); line++ {
		text := s.Text()
		runes := []rune(text)

		span := func() cc.Span {
			return cc.Span{
				Start: cc.Pos{
					File: name,
					Line: line,
				},
				End: cc.Pos{
					File: name,
					Line: line,
				},
			}
		}

		if line == 1 && text != pragmaOnce {
			Errorf(issues, span(), "missing %q", pragmaOnce)
		}

		if line == 2 && text != ifndef {
			Errorf(issues, span(), "missing %q", ifndef)
		}

		if line == 3 && text != define {
			Errorf(issues, span(), "missing %q", define)
		}

		if strings.Contains(text, include) {
			Errorf(issues, span(), "nested %s", include)
		}

		// Check indentation.
		for _, r := range runes {
			if !unicode.IsSpace(r) {
				break
			}

			if r != '\t' {
				Errorf(issues, span(), "non-tab indentation (%s)", strconv.QuoteRune(r))
				break
			}
		}

		// Check for trailing spaces.
		for i := len(runes); i > 0; i-- {
			r := runes[i-1]
			if !unicode.IsSpace(r) {
				break
			}

			Errorf(issues, span(), "trailing whitespace")
			break
		}

		finalLine = text
		finalLineNum = line
	}

	if err := s.Err(); err != nil {
		log.Fatalf("failed to parse %q: %v", name, err)
	}

	if finalLine != endif {
		span := cc.Span{
			Start: cc.Pos{
				File: name,
				Line: finalLineNum,
			},
			End: cc.Pos{
				File: name,
				Line: finalLineNum,
			},
		}

		Errorf(issues, span, "missing %q", endif)
	}
}
