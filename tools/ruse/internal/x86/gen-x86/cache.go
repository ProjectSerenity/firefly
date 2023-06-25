// Copyright 2023 The Firefly Authors.
//
// Use of this source code is governed by a BSD 3-clause
// license that can be found in the LICENSE file.

package main

import (
	"errors"
	"io"
)

// The cache reader caches disk reads to reduce
// I/O when the same parts of a reader are read
// many times.

const (
	cacheBlockSize = 64 * 1024
	numCacheBlock  = 16
)

type cachedReaderAt struct {
	r     io.ReaderAt
	cache *cacheBlock
}

type cacheBlock struct {
	next   *cacheBlock
	buf    []byte
	offset int64
	err    error
}

func newCachedReaderAt(r io.ReaderAt) *cachedReaderAt {
	c := &cachedReaderAt{
		r: r,
	}
	for i := 0; i < numCacheBlock; i++ {
		c.cache = &cacheBlock{next: c.cache}
	}
	return c
}

func (c *cachedReaderAt) ReadAt(p []byte, offset int64) (n int, err error) {
	// Assume large reads indicate a caller that doesn't need caching.
	if len(p) >= cacheBlockSize {
		return c.r.ReadAt(p, offset)
	}

	for n < len(p) {
		o := offset + int64(n)
		f := o & (cacheBlockSize - 1)
		b := c.readBlock(o - f)
		n += copy(p[n:], b.buf[f:])
		if n < len(p) && b.err != nil {
			return n, b.err
		}
	}
	return n, nil
}

var errShortRead = errors.New("short read")

func (c *cachedReaderAt) readBlock(offset int64) *cacheBlock {
	if offset&(cacheBlockSize-1) != 0 {
		panic("misuse of cachedReaderAt.readBlock")
	}

	// Look in cache.
	var b, prev *cacheBlock
	for b = c.cache; ; prev, b = b, b.next {
		if b.buf != nil && b.offset == offset {
			// Move to front.
			if prev != nil {
				prev.next = b.next
				b.next = c.cache
				c.cache = b
			}
			return b
		}
		if b.next == nil {
			break
		}
	}

	// Otherwise b is LRU block in cache, prev points at b.
	if b.buf == nil {
		b.buf = make([]byte, cacheBlockSize)
	}

	b.offset = offset
	n, err := c.r.ReadAt(b.buf[:cacheBlockSize], offset)
	b.buf = b.buf[:n]
	b.err = err
	if n > 0 {
		// Move to front.
		prev.next = nil
		b.next = c.cache
		c.cache = b
	}

	return b
}
