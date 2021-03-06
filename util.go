// Copyright (C) 2019 Christopher E. Miller
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package smallprox

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"io"
	"io/ioutil"
	"log"
	"strings"
	"sync/atomic"
)

func readBytes(r io.Reader) ([]byte, error) {
	if m, ok := r.(*Mutable); ok {
		return m.Bytes(), nil
	}
	if m, ok := r.(*bytes.Buffer); ok {
		return m.Bytes(), nil
	}
	return ioutil.ReadAll(r)
}

// Mutable is a buffer which may be directly mutated.
type Mutable struct {
	bytes.Buffer
}

// Close implements io.Closer
func (m *Mutable) Close() error {
	m.Buffer.Reset()
	return nil
}

func (m *Mutable) Peek(n int) ([]byte, error) {
	b := m.Bytes()
	if n <= len(b) {
		return b[:n], nil
	}
	return b, io.EOF
}

func HasAnyFold(all []string, s string) bool {
	for _, x := range all {
		if strings.EqualFold(s, x) {
			return true
		}
	}
	return false
}

func HasHeaderValuePart(x string, value string) bool {
	start := 0
	for {
		off := strings.Index(x[start:], value)
		if off == -1 {
			break
		}
		pos := start + off
		if pos == 0 || x[pos-1] == ',' || (pos-1 != 0 && x[pos-1] == ' ' && x[pos-2] == ',') {
			end := pos + len(value)
			if end == len(x) || x[end] == ',' || x[end] == ';' {
				return true
			}
		}
		start += off + 1
	}
	return false
}

func HasAnyHeaderValuePart(from []string, value string) bool {
	for _, x := range from {
		if HasHeaderValuePart(x, value) {
			return true
		}
	}
	return false
}

func mustDecodeStringBase64(s string) []byte {
	x, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		log.Fatalf("Unable to decode base64: %s", err)
	}
	return x
}

type toggle struct {
	_false int32 // atomic
}

func (t *toggle) Enabled() bool {
	return atomic.LoadInt32(&t._false) == 0
}

func (t *toggle) SetEnabled(enabled bool) {
	if enabled {
		atomic.StoreInt32(&t._false, 0)
	} else {
		atomic.StoreInt32(&t._false, 1)
	}
}

type peeker interface {
	io.Reader
	Peek(n int) ([]byte, error)
}

type peekCloser interface {
	peeker
	io.Closer
}

// If it's already a peek closer, it's returned.
func newPeekCloser(r io.ReadCloser) peekCloser {
	if pc, ok := r.(peekCloser); ok {
		return pc
	}
	return struct {
		*bufio.Reader
		io.Closer
	}{bufio.NewReader(r), r}
}
