// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Command check-deps uses package vendeps to check external dependencies for issues.
//
package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"golang.org/x/time/rate"

	"firefly-os.dev/tools/vendeps"
)

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stderr)
	log.SetPrefix("")
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

const userAgent = "Firefly-dependency-vendoring/1 (github.com/ProjectSerenity/firefly)"

var rateLimit = rate.NewLimiter(rate.Every(time.Second), 1) // 1 request per second.

func httpRequest(req *http.Request) (*http.Response, error) {
	// Make sure we always use our User-Agent.
	req.Header.Set("User-Agent", userAgent)

	// Apply our rate limeter, waiting if necessary.
	err := rateLimit.Wait(context.Background())
	if err != nil {
		return nil, err
	}

	return httpClient.Do(req)
}

func main() {
	// If we're being run with `bazel run`, we're in
	// a semi-random build directory, and need to move
	// to the workspace root directory.
	//
	workspace := os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	if workspace != "" {
		err := os.Chdir(workspace)
		if err != nil {
			log.Printf("Failed to change directory to %q: %v", workspace, err)
		}
	}

	fsys := os.DirFS(".")
	err := vendeps.CheckDependencies(fsys)
	if err != nil {
		log.Fatalf("Failed to check dependencies: %v", err)
	}
}
