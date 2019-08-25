// Copyright (C) 2019 Christopher E. Miller
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package smallprox

import (
	"net/http"
	"strings"
	"sync"

	"golang.org/x/exp/errors/fmt"
)

type TypeFilterResponder struct {
	toggle
	blocklist []string
	mx        sync.RWMutex
}

// Block a file; a block is either a file extension (without preceeding dot), or a MIME type (with a slash).
func (er *TypeFilterResponder) Block(blocks ...string) {
	er.mx.Lock()
	defer er.mx.Unlock()
	er.blocklist = append(er.blocklist, blocks...)
}

// path is the virtual URL such as /foo.x from url.URL.Path
func (er *TypeFilterResponder) isBlockedRLocked(contentType string, path string) bool {
	return inTypeFilter(contentType, path, er.blocklist)
}

func (er *TypeFilterResponder) Response(req *http.Request, resp *http.Response) *http.Response {
	if !er.Enabled() {
		return resp
	}
	er.mx.RLock()
	defer er.mx.RUnlock()
	respContentType := resp.Header.Get("Content-Type")
	path := resp.Request.URL.Path // Get it from the response in case of redirect.
	if er.isBlockedRLocked(respContentType, path) {
		resp.Body.Close()
		resp.Body = &Mutable{}
		for x := range resp.Header {
			delete(resp.Header, x)
		}
		resp.StatusCode = 521
		resp.Status = fmt.Sprintf("%v %v", resp.StatusCode, "Down")
	}
	return resp
}

// path is the virtual URL such as /foo.x from url.URL.Path
func inTypeFilter(contentType string, path string, filterlist []string) bool {
	for _, x := range filterlist {
		if strings.IndexByte(x, '/') != -1 { // MIME-type:
			if x == contentType {
				return true
			}
			if len(contentType) > len(x) {
				if contentType[:len(x)] == x && (contentType[len(x)] == ';' || contentType[len(x)] == '+') {
					return true
				}
			}
		} else { // Extension:
			if len(path) > len(x) {
				if path[len(path)-len(x)-1] == '.' {
					if strings.EqualFold(x, path[len(path)-len(x):]) {
						return true
					}
				}
			}
		}
	}
	return false
}

var TypeFilterFonts = []string{
	"application/x-font-ttf",
	"application/x-font-truetype",
	"application/x-font-opentype",
	"application/font-woff",
	"application/font-woff2",
	"application/vnd.ms-fontobject",
	"application/font-sfnt",
	"font/woff2",
	"font/opentype",
	// Extensions:
	"fon",
	"woff",
	"woff2",
	"otf",
	"ttf",
	"eot",
}
