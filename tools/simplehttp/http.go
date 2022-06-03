// Copyright 2022 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package simplehttp

import (
	"context"
	"net"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

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

var RateLimit = rate.NewLimiter(rate.Every(time.Second), 1) // 1 request per second.

func Request(req *http.Request, userAgent string) (*http.Response, error) {
	// Make sure we always use our User-Agent.
	req.Header.Set("User-Agent", userAgent)

	// Apply our rate limeter, waiting if necessary.
	err := RateLimit.Wait(context.Background())
	if err != nil {
		return nil, err
	}

	return Client.Do(req)
}
