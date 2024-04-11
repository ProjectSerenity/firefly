// Copyright 2024 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

// Package perfdata provides helpers for collecting and
// printing performance time data.
package perfdata

import (
	"fmt"
	"io"
	"math"
	"text/tabwriter"
	"time"
)

// perfEntry contains the data for a single interval.
type perfEntry struct {
	Duration    time.Duration
	Description string
}

// Log contains data on successive performance intervals.
type Log struct {
	start time.Time
	steps []perfEntry
}

// New returns an initialised performance log.
func New() *Log {
	return &Log{
		start: time.Now(),
		steps: make([]perfEntry, 0, 10),
	}
}

// Record adds a performance entry with the given
// step description.
func (l *Log) Record(step string) {
	d := time.Now().Sub(l.start)
	l.steps = append(l.steps, perfEntry{Duration: d, Description: step})
}

// Print writes a descriptive table of the performance
// interval data to w. It does not modify the log.
func (l *Log) Print(w io.Writer) error {
	var cum time.Duration
	tw := tabwriter.NewWriter(w, 0, 8, 1, ' ', 0)

	fmt.Fprintf(tw, "Step\tCum\tDescription\n")
	for _, step := range l.steps {
		fmt.Fprintf(tw, "%s\t%s\t%s\n", trimDuration(step.Duration), trimDuration(step.Duration+cum), step.Description)
		cum += step.Duration
	}

	return tw.Flush()
}

// trimDuration returns the string format for d,
// capped at 3 significant figures.
func trimDuration(d time.Duration) string {
	// We calculate the order of the most significant
	// figure, then round d to two orders lower.
	order := int(math.Round(math.Log10(float64(d.Nanoseconds()))))
	limit := time.Duration(math.Pow10(order - 2))
	return d.Round(limit).String()
}
