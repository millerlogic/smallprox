// Copyright (C) 2019 Christopher E. Miller
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package smallprox

import (
	"io"
	"net/http"
	"strings"
	"sync/atomic"
)

type LimitBytesResponder struct {
	limit int64 // atomic
}

func (er *LimitBytesResponder) Limit() int64 {
	return atomic.LoadInt64(&er.limit)
}

func (er *LimitBytesResponder) SetLimit(limitBytes int64) {
	atomic.StoreInt64(&er.limit, limitBytes)
}

func (er *LimitBytesResponder) Response(req *http.Request, resp *http.Response) *http.Response {
	limitBytes := er.Limit()
	if limitBytes <= 0 {
		return resp
	}
	respMIMEType := resp.Header.Get("Content-Type")
	if respMIMEType != "" && !strings.HasPrefix(respMIMEType, "application/octet-stream") {
		// Limit everything that doesn't look like a download.
		resp.Body = &limitCloser{io.LimitedReader{R: resp.Body, N: limitBytes}}
	}
	return resp
}

type limitCloser struct {
	io.LimitedReader
}

func (r *limitCloser) Close() error {
	if c, ok := r.R.(io.Closer); ok {
		return c.Close()
	}
	return nil
}
