// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package simplehttp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

// Client is the underlying HTTP client that
// will be used by Request.
//
var Client = &http.Client{
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

// UserAgent is the user-agent string that
// should be provided with requests. If
// UserAgent is empty, Request will return
// an error.
//
var UserAgent string

// RateLimit will be used by Request to ensure
// we do not overload the servers we use.
//
var RateLimit = rate.NewLimiter(rate.Every(time.Second), 1) // 1 request per second.

// Request performs the given HTTP request,
// returning the response.
//
func Request(req *http.Request) (*http.Response, error) {
	if UserAgent == "" {
		return nil, errors.New("simplehttp.UserAgent is unset")
	}

	// Make sure we always use our User-Agent.
	req.Header.Set("User-Agent", UserAgent)

	// Apply our rate limeter, waiting if necessary.
	err := RateLimit.Wait(context.Background())
	if err != nil {
		return nil, err
	}

	return Client.Do(req)
}
