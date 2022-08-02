// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package cratesio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"golang.org/x/time/rate"
	"rsc.io/diff"

	"firefly-os.dev/tools/simplehttp"
)

func TestFetchCrate(t *testing.T) {
	// Allow 1000 requests per second as we
	// don't need to rate-limit tests.
	simplehttp.RateLimit.SetLimit(rate.Every(time.Millisecond))
	const testUserAgent = "test-user-agent"
	simplehttp.UserAgent = testUserAgent

	// Start an HTTP server, serving a
	// captured copy of an actual response.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != testUserAgent {
			t.Errorf("Got request with User-Agent %q, want %q", got, testUserAgent)
		}

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
					"abi_x86_interrupt": {},
					"const_fn":          {},
					"default": {
						"nightly",
						"instructions",
					},
					"doc_cfg": {},
					"external_asm": {
						"cc",
					},
					"inline_asm":   {},
					"instructions": {},
					"nightly": {
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
					"abi_x86_interrupt": {},
					"const_fn":          {},
					"default": {
						"nightly",
						"instructions",
					},
					"doc_cfg": {},
					"external_asm": {
						"cc",
					},
					"inline_asm":   {},
					"instructions": {},
					"nightly": {
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
					"abi_x86_interrupt": {},
					"const_fn":          {},
					"default": {
						"nightly",
						"instructions",
					},
					"doc_cfg": {},
					"external_asm": {
						"cc",
					},
					"inline_asm":   {},
					"instructions": {},
					"nightly": {
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
					"abi_x86_interrupt": {},
					"const_fn":          {},
					"default": {
						"nightly",
						"instructions",
					},
					"doc_cfg": {},
					"external_asm": {
						"cc",
					},
					"inline_asm":   {},
					"instructions": {},
					"nightly": {
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
	got, err := lookup(ctx, srv.URL, "x86_64")
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, want) {
		// Encoding the values in JSON makes the error
		// message more useful and legible.
		gotJSON, err := json.MarshalIndent(got, "", "\t")
		if err != nil {
			t.Fatal(err)
		}

		wantJSON, err := json.MarshalIndent(want, "", "\t")
		if err != nil {
			t.Fatal(err)
		}

		t.Fatalf("Got unexpected crate data:\n%s", diff.Format(string(gotJSON), string(wantJSON)))
	}
}
