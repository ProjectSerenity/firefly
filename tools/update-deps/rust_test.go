// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestParseRustBzl(t *testing.T) {
	bzlPath := filepath.Join("testdata", "rust_bzl")
	_, date, tools, err := ParseRustBzl(bzlPath)
	if err != nil {
		t.Fatal(err)
	}

	checkField(t, "date", "date", *date, "2022-02-01")

	wantTools := []struct {
		Name string
		Sum  string
	}{
		{
			Name: "llvm-tools-nightly-x86_64-unknown-linux-gnu",
			Sum:  "3eeba27c46ac7f2fd9092ed5baf8616c04021ac359f136a484b5942229e590fc",
		},
		{
			Name: "rust-nightly-x86_64-unknown-linux-gnu",
			Sum:  "fe928a3f280355a1b87eb414ac9ab1333a38a3e5e6be1f1d6fa3e990527aec80",
		},
		{
			Name: "rust-src-nightly",
			Sum:  "6177a62bd2c56dfeda4552d64d9f840ce3bbdef7206b9bcd7047c0b5af56f4a8",
		},
		{
			Name: "rust-std-nightly-x86_64-unknown-linux-gnu",
			Sum:  "882f458492f7efa8a9af5e5ffc8b70183107447fe4604a8c47a120b4f319e72e",
		},
		{
			Name: "rustfmt-nightly-x86_64-unknown-linux-gnu",
			Sum:  "6cd904d0413a858a6073f1a553d2aa46e32124574da996dcd0d8aaeb706bd035",
		},
		{
			Name: "rust-std-nightly-x86_64-unknown-none",
			Sum:  "35cd94ae9a6efc1839c227470041038e3c51f50db1f2c59ed7f5b32d03f4cd2f",
		},
	}

	if len(tools) != len(wantTools) {
		t.Errorf("found %d tools, want %d:", len(tools), len(wantTools))
		for _, tool := range tools {
			t.Logf("{%q, %q}", tool.Name, tool.Sum)
		}

		return
	}

	for i := range tools {
		got := tools[i]
		want := wantTools[i]
		context := fmt.Sprintf("tool %d", i)
		checkField(t, context, "name", got.Name, want.Name)
		checkField(t, context, "sum", got.Sum, want.Sum)
	}
}
