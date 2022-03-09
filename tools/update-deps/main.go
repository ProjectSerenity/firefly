// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command update-deps helps identify and perform updates to Firefly's dependencies.
//
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/bazelbuild/buildtools/build"
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

var httpClient = &http.Client{
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
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

	// If we're being run with `bazel run`, we're in
	// a semi-random build directory, and need to move
	// to the caller's working directory.
	//
	workspace := os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	if workspace != "" {
		err := os.Chdir(workspace)
		if err != nil {
			log.Printf("Failed to change directory to %q: %v", workspace, err)
		}
	}

	if len(args) == 0 {
		// Run all the commands.
		for _, name := range commandsNames {
			log.SetPrefix(name + ": ")
			cmd := commandsMap[name]
			err := cmd.Func(context.Background(), os.Stdout, args)
			if err != nil {
				log.Fatal(err)
			}
		}

		return
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

// UnmarshalFields processes the AST node for a
// Starlark function call and stores its parameters
// into data.
//
// UnmarshalFields will return an error if any required
// fields were unset, or if any additional fields were
// found in the AST.
//
func UnmarshalFields(call *build.CallExpr, v interface{}) error {
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return fmt.Errorf("invalid set of fields: got %v, expected struct", val.Kind())
	}

	// Use reflection to extract the data for
	// each field in a format we can process
	// more easily as we iterate through the
	// call.

	type FieldData struct {
		Name     string
		Optional bool
		Ignore   bool
		Value    *string
		Ptr      **string
	}

	valType := val.Type()
	fieldType := reflect.TypeOf(StringField{})
	numFields := val.NumField()
	fields := make([]*FieldData, numFields)
	fieldsMap := make(map[string]*FieldData)
	for i := 0; i < numFields; i++ {
		valField := val.Field(i)
		typeField := valType.Field(i)
		if valField.Type() != fieldType {
			return fmt.Errorf("invalid set of fields: field %s has unexpected type %s, want %s", typeField.Name, valField.Type(), fieldType)
		}

		name, ok := typeField.Tag.Lookup("bzl")
		optional := false
		ignore := false
		if strings.HasSuffix(name, ",optional") {
			optional = true
			name = strings.TrimSuffix(name, ",optional")
		} else if strings.HasSuffix(name, ",ignore") {
			ignore = true
			name = strings.TrimSuffix(name, ",ignore")
		}

		if !ok {
			name = typeField.Name
		}

		if name == "" {
			return fmt.Errorf("invalid set of fields: field %s has no field name", typeField.Name)
		}

		// We already know valField is a struct.
		valPtr := valField.Field(0).Addr().Interface().(*string)
		ptrPtr := valField.Field(1).Addr().Interface().(**string)

		field := &FieldData{
			Name:     name,
			Optional: optional,
			Ignore:   ignore,
			Value:    valPtr,
			Ptr:      ptrPtr,
		}

		if fieldsMap[name] != nil {
			return fmt.Errorf("invalid set of fields: multiple fields have the name %q", name)
		}

		fields[i] = field
		fieldsMap[name] = field
	}

	// Now we have the field data ready, we can
	// start parsing the call.

	for i, expr := range call.List {
		assign, ok := expr.(*build.AssignExpr)
		if !ok {
			return fmt.Errorf("field %d in the call is not an assignment", i)
		}

		lhs, ok := assign.LHS.(*build.Ident)
		if !ok {
			return fmt.Errorf("field %d in the call assigns to a non-identifier value %#v", i, assign.LHS)
		}

		field := fieldsMap[lhs.Name]
		if field == nil {
			return fmt.Errorf("field %d in the call has unexpected field %q", i, lhs.Name)
		}

		if field.Ignore {
			continue
		}

		if *field.Ptr != nil {
			return fmt.Errorf("field %d in the call assigns to %s for the second time", i, lhs.Name)
		}

		rhs, ok := assign.RHS.(*build.StringExpr)
		if !ok {
			return fmt.Errorf("field %d in the call (%s) has non-string value %#v", i, lhs.Name, assign.RHS)
		}

		*field.Value = rhs.Value
		*field.Ptr = &rhs.Value
	}

	// Check we've got values for all required
	// fields.
	for _, field := range fields {
		if field.Optional || field.Ignore {
			continue
		}

		if *field.Ptr != nil {
			continue
		}

		return fmt.Errorf("function call had no value for required field %s", field.Name)
	}

	return nil
}

// StringField represents a field in a Starlark
// function that receives a string literal.
//
type StringField struct {
	// The parsed value.
	Value string

	// A pointer to the original AST node, which
	// can be modified to update the AST.
	Ptr *string
}

func (f StringField) String() string {
	return f.Value
}
