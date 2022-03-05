// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestParseRustBzl(t *testing.T) {
	bzlPath := filepath.Join("testdata", "rust_bzl")
	_, date, tools, crates, err := ParseRustBzl(bzlPath)
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
	}

	wantCrates := []struct {
		Name   string
		Semver string
	}{
		{
			Name:   "bitflags",
			Semver: "=1.3.2",
		},
		{
			Name:   "bit_field",
			Semver: "=0.10.1",
		},
		{
			Name:   "byteorder",
			Semver: "=1.4.3",
		},
		{
			Name:   "chacha20",
			Semver: "=0.8.1",
		},
		{
			Name:   "fixedvec",
			Semver: "=0.2.4",
		},
		{
			Name:   "hex-literal",
			Semver: "=0.3.4",
		},
		{
			Name:   "lazy_static",
			Semver: "=1.4.0",
		},
		{
			Name:   "libc",
			Semver: "=0.2.117",
		},
		{
			Name:   "linked_list_allocator",
			Semver: "=0.9.0",
		},
		{
			Name:   "llvm-tools",
			Semver: "=0.1.1",
		},
		{
			Name:   "managed",
			Semver: "=0.8",
		},
		{
			Name:   "pic8259",
			Semver: "=0.10.1",
		},
		{
			Name:   "raw-cpuid",
			Semver: "=10.2.0",
		},
		{
			Name:   "rlibc",
			Semver: "=1.0.0",
		},
		{
			Name:   "serde",
			Semver: "=1.0.116",
		},
		{
			Name:   "sha2",
			Semver: "=0.10.1",
		},
		{
			Name:   "spin",
			Semver: "=0.9.2",
		},
		{
			Name:   "thiserror",
			Semver: "=1.0.16",
		},
		{
			Name:   "toml",
			Semver: "=0.5.6",
		},
		{
			Name:   "usize_conversions",
			Semver: "=0.2.0",
		},
		{
			Name:   "volatile",
			Semver: "=0.4.4",
		},
		{
			Name:   "x86_64",
			Semver: "=0.14.7",
		},
		{
			Name:   "xmas-elf",
			Semver: "=0.6.2",
		},
		{
			Name:   "zero",
			Semver: "=0.1.2",
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

	if len(crates) != len(wantCrates) {
		t.Errorf("found %d crates, want %d:", len(crates), len(wantCrates))
		for _, crate := range crates {
			t.Logf("{%q, %q}", crate.Name, crate.Semver)
		}

		return
	}

	for i := range crates {
		got := crates[i]
		want := wantCrates[i]
		context := fmt.Sprintf("crate %d", i)
		checkField(t, context, "name", got.Name, want.Name)
		checkField(t, context, "sum", got.Semver, want.Semver)
	}
}

func TestFetchCrate(t *testing.T) {
	// Start an HTTP server, serving a
	// captured copy of an actual response.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/crates/x86_64" {
			http.ServeFile(w, r, filepath.Join("testdata", "crates-io-x86_64.json"))
		} else {
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	// I think the timezone should be UTC, but
	// Go reckons it's time.Local.
	timezone := time.Local
	want := &Crate{
		Categories: []*CrateCategory{
			{
				Category:    "No standard library",
				CratesCount: 3396,
				CreatedAt:   time.Date(2017, time.February, 10, 1, 52, 9, 447906000, timezone),
				Description: "Crates that are able to function without the Rust standard library.\n",
				Id:          "no-std",
				Slug:        "no-std",
			},
		},
		Crate: CrateData{
			Id:              "x86_64",
			Name:            "x86_64",
			Description:     "Support for x86_64 specific instructions, registers, and structures.",
			Documentation:   "https://docs.rs/x86_64",
			Repository:      "https://github.com/rust-osdev/x86_64",
			Downloads:       499458,
			RecentDownloads: 86588,
			Categories: []string{
				"no-std",
			},
			Keywords: []string{
				"no_std",
				"x86_64",
				"x86",
				"amd64",
			},
			Versions: []uint64{
				494296,
				470069,
				428243,
				424815,
				402862,
				381248,
				378135,
				375101,
				372987,
				364314,
				364317,
				359719,
				357210,
				352894,
				334493,
				319972,
				319617,
				319605,
				304459,
				288487,
				287026,
				286671,
				286667,
				282321,
				282319,
				279597,
				278856,
				278855,
				272410,
				253986,
				241277,
				240063,
				237710,
				232484,
				229879,
				219172,
				215735,
				215483,
				214388,
				211288,
				211277,
				211271,
				202521,
				196756,
				195489,
				195478,
				192337,
				190443,
				170683,
				163853,
				163449,
				162080,
				161966,
				156191,
				149395,
				148102,
				147976,
				143879,
				140827,
				138313,
				138174,
				138036,
				137130,
				130773,
				130769,
				133112,
				131602,
				130310,
				130193,
				129496,
				127483,
				116703,
				116700,
				116696,
				114711,
				114304,
				113747,
				111569,
				111396,
				101240,
				101238,
				99758,
				99757,
				99756,
				98558,
				97048,
				97022,
				96887,
				96653,
				96523,
				96522,
				96505,
				95848,
				95718,
				89025,
				89016,
				88890,
				88548,
				88115,
				88086,
				88084,
				87723,
				86013,
				85780,
				83441,
				80039,
				80038,
				79023,
				78796,
				78089,
				78087,
				77818,
				49080,
				48165,
				48160,
				40958,
				40956,
				40954,
				402842,
				402840,
				402833,
				40952,
			},
			MaxVersion: "0.14.8",
			Links: CrateLinks{
				OwnerTeam:           "/api/v1/crates/x86_64/owner_team",
				OwnerUser:           "/api/v1/crates/x86_64/owner_user",
				Owners:              "/api/v1/crates/x86_64/owners",
				ReverseDependencies: "/api/v1/crates/x86_64/reverse_dependencies",
				VersionDownloads:    "/api/v1/crates/x86_64/downloads",
			},
			CreatedAt: time.Date(2016, time.December, 27, 23, 3, 49, 101020000, timezone),
			UpdatedAt: time.Date(2022, time.February, 3, 13, 0, 9, 777698000, timezone),
		},
		Keywords: []*CrateKeyword{
			{
				Id:          "no_std",
				Keyword:     "no_std",
				CratesCount: 671,
				CreatedAt:   time.Date(2015, time.June, 20, 4, 34, 42, 753830000, timezone),
			},
			{
				Id:          "x86_64",
				Keyword:     "x86_64",
				CratesCount: 18,
				CreatedAt:   time.Date(2015, time.July, 12, 3, 14, 14, 199125000, timezone),
			},
			{
				Id:          "x86",
				Keyword:     "x86",
				CratesCount: 39,
				CreatedAt:   time.Date(2015, time.March, 18, 20, 35, 22, 262661000, timezone),
			},
			{
				Id:          "amd64",
				Keyword:     "amd64",
				CratesCount: 21,
				CreatedAt:   time.Date(2015, time.March, 28, 19, 46, 44, 950000, timezone),
			},
		},
		Versions: []*CrateVersion{
			{
				Crate:        "x86_64",
				CreatedAt:    time.Date(2022, time.February, 3, 13, 0, 9, 777698000, timezone),
				UpdatedAt:    time.Date(2022, time.February, 3, 13, 0, 9, 777698000, timezone),
				DownloadPath: "/api/v1/crates/x86_64/0.14.8/download",
				Downloads:    23040,
				Features: map[string][]string{
					"abi_x86_interrupt": []string{},
					"const_fn":          []string{},
					"default": []string{
						"nightly",
						"instructions",
					},
					"doc_cfg": []string{},
					"external_asm": []string{
						"cc",
					},
					"inline_asm":   []string{},
					"instructions": []string{},
					"nightly": []string{
						"inline_asm",
						"const_fn",
						"abi_x86_interrupt",
						"doc_cfg",
					},
				},
				Id:         494296,
				Number:     "0.14.8",
				Yanked:     false,
				License:    "MIT/Apache-2.0",
				ReadmePath: "/api/v1/crates/x86_64/0.14.8/readme",
				Links: CrateVersionLinks{
					Dependencies:     "/api/v1/crates/x86_64/0.14.8/dependencies",
					VersionDownloads: "/api/v1/crates/x86_64/0.14.8/downloads",
				},
				CrateSize: 74801,
				PublishedBy: &CrateUser{
					Avatar: "https://avatars.githubusercontent.com/u/87635370?v=4",
					Id:     127438,
					Login:  "rust-osdev-autorelease",
					URL:    "https://github.com/rust-osdev-autorelease",
				},
			},
			{
				Crate:        "x86_64",
				CreatedAt:    time.Date(2021, time.December, 18, 17, 27, 2, 564043000, timezone),
				UpdatedAt:    time.Date(2021, time.December, 18, 17, 27, 2, 564043000, timezone),
				DownloadPath: "/api/v1/crates/x86_64/0.14.7/download",
				Downloads:    25329,
				Features: map[string][]string{
					"abi_x86_interrupt": []string{},
					"const_fn":          []string{},
					"default": []string{
						"nightly",
						"instructions",
					},
					"doc_cfg": []string{},
					"external_asm": []string{
						"cc",
					},
					"inline_asm":   []string{},
					"instructions": []string{},
					"nightly": []string{
						"inline_asm",
						"const_fn",
						"abi_x86_interrupt",
						"doc_cfg",
					},
				},
				Id:         470069,
				Number:     "0.14.7",
				Yanked:     false,
				License:    "MIT/Apache-2.0",
				ReadmePath: "/api/v1/crates/x86_64/0.14.7/readme",
				Links: CrateVersionLinks{
					Dependencies:     "/api/v1/crates/x86_64/0.14.7/dependencies",
					VersionDownloads: "/api/v1/crates/x86_64/0.14.7/downloads",
				},
				CrateSize: 73953,
				PublishedBy: &CrateUser{
					Avatar: "https://avatars.githubusercontent.com/u/87635370?v=4",
					Id:     127438,
					Login:  "rust-osdev-autorelease",
					URL:    "https://github.com/rust-osdev-autorelease",
				},
			},
			{
				Crate:        "x86_64",
				CreatedAt:    time.Date(2021, time.September, 21, 8, 57, 32, 723270000, timezone),
				UpdatedAt:    time.Date(2021, time.September, 21, 8, 57, 32, 723270000, timezone),
				DownloadPath: "/api/v1/crates/x86_64/0.14.6/download",
				Downloads:    44972,
				Features: map[string][]string{
					"abi_x86_interrupt": []string{},
					"const_fn":          []string{},
					"default": []string{
						"nightly",
						"instructions",
					},
					"doc_cfg": []string{},
					"external_asm": []string{
						"cc",
					},
					"inline_asm":   []string{},
					"instructions": []string{},
					"nightly": []string{
						"inline_asm",
						"const_fn",
						"abi_x86_interrupt",
						"doc_cfg",
					},
				},
				Id:         428243,
				Number:     "0.14.6",
				Yanked:     false,
				License:    "MIT/Apache-2.0",
				ReadmePath: "/api/v1/crates/x86_64/0.14.6/readme",
				Links: CrateVersionLinks{
					Dependencies:     "/api/v1/crates/x86_64/0.14.6/dependencies",
					VersionDownloads: "/api/v1/crates/x86_64/0.14.6/downloads",
				},
				CrateSize: 70939,
				PublishedBy: &CrateUser{
					Avatar: "https://avatars.githubusercontent.com/u/87635370?v=4",
					Id:     127438,
					Login:  "rust-osdev-autorelease",
					URL:    "https://github.com/rust-osdev-autorelease",
				},
			},
			{
				Crate:        "x86_64",
				CreatedAt:    time.Date(2021, time.September, 13, 7, 15, 49, 31164000, timezone),
				UpdatedAt:    time.Date(2021, time.September, 13, 7, 15, 49, 31164000, timezone),
				DownloadPath: "/api/v1/crates/x86_64/0.14.5/download",
				Downloads:    6883,
				Features: map[string][]string{
					"abi_x86_interrupt": []string{},
					"const_fn":          []string{},
					"default": []string{
						"nightly",
						"instructions",
					},
					"doc_cfg": []string{},
					"external_asm": []string{
						"cc",
					},
					"inline_asm":   []string{},
					"instructions": []string{},
					"nightly": []string{
						"inline_asm",
						"const_fn",
						"abi_x86_interrupt",
						"doc_cfg",
					},
				},
				Id:         424815,
				Number:     "0.14.5",
				Yanked:     false,
				License:    "MIT/Apache-2.0",
				ReadmePath: "/api/v1/crates/x86_64/0.14.5/readme",
				Links: CrateVersionLinks{
					Dependencies:     "/api/v1/crates/x86_64/0.14.5/dependencies",
					VersionDownloads: "/api/v1/crates/x86_64/0.14.5/downloads",
				},
				CrateSize: 70621,
				PublishedBy: &CrateUser{
					Avatar: "https://avatars.githubusercontent.com/u/87635370?v=4",
					Id:     127438,
					Login:  "rust-osdev-autorelease",
					URL:    "https://github.com/rust-osdev-autorelease",
				},
			},
			// Subsequent versions snipped.
		},
	}

	ctx := context.Background()
	got, err := FetchCrate(ctx, srv.URL, "x86_64")
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Got unexpected crate data:\nGot:  %#v\nWant: %#v", got, want)
	}
}
