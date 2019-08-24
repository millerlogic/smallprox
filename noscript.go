// Copyright (C) 2019 Christopher E. Miller
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package smallprox

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"regexp"
	"strings"

	"github.com/tdewolff/parse/html"
	"golang.org/x/exp/errors/fmt"
)

type NoscriptResponder struct {
	toggle
}

func (er *NoscriptResponder) Response(req *http.Request, resp *http.Response) *http.Response {
	if !er.Enabled() {
		return resp
	}
	respContentType := resp.Header.Get("Content-Type")
	respMIMEType := respContentType
	{
		isem := strings.IndexByte(respMIMEType, ';')
		if isem != -1 {
			respMIMEType = respMIMEType[:isem]
		}
	}
	if respMIMEType == "text/html" {
		outbuf := &Mutable{}
		noscriptStreamer(resp.Body, outbuf)
		resp.Body.Close()
		resp.Body = outbuf
	} else {
		simpleMIME := respMIMEType
		if iplus := strings.IndexByte(simpleMIME, '+'); iplus != -1 {
			simpleMIME = simpleMIME[:iplus]
		}
		switch simpleMIME {
		case "application/javascript",
			"application/x-javascript",
			"text/javascript",
			"application/ecmascript",
			"text/ecmascript":
			resp.Body.Close()
			resp.Body = ioutil.NopCloser(bytes.NewBufferString("// noscript\n"))
			resp.Header.Set("Content-Type", "text/plain")
			resp.StatusCode = 521
			resp.Status = fmt.Sprintf("%v %v", resp.StatusCode, "Down")
		}
	}
	return resp
}

func noscriptStreamer(r io.Reader, w io.Writer) error {
	inScript := false
	startTagIsScript := false
	var finalErr error
	write := func(x []byte) {
		if !inScript && finalErr == nil {
			_, finalErr = w.Write(x)
		}
	}
	lex := html.NewLexer(r)
lexing:
	for finalErr == nil {
		tt, data := lex.Next()
		switch tt {
		case html.ErrorToken:
			if lex.Err() != io.EOF && finalErr == nil {
				finalErr = lex.Err()
			}
			break lexing
		case html.StartTagToken:
			if !inScript && scriptTagRegexp.Match(data) {
				// Skip.
				inScript = true
				startTagIsScript = true
			} else if noscriptTagRegexp.Match(data) {
				write([]byte("<div data-from-noscript=true"))
			} else {
				startTagIsScript = false
				write(data)
			}
		case html.StartTagCloseToken: // the > in <foo>
			startTagIsScript = false
			write(data)
		case html.StartTagVoidToken: // self closing, after StartTagToken
			if startTagIsScript {
				// Skip.
				inScript = false
				startTagIsScript = false
			} else {
				write(data)
			}
		case html.EndTagToken:
			if scriptTagEndRegexp.Match(data) {
				// Skip.
				inScript = false
				startTagIsScript = false
			} else if noscriptTagEndRegexp.Match(data) {
				write([]byte("</div>"))
			} else {
				write(data)
			}
		case html.AttributeToken:
			if eventAttribRegexp.Match(data) {
				// Skip.
			} else if hrefJsAttribRegexp.Match(data) {
				write([]byte(` href=#noscript`))
			} else if srcJsAttribRegexp.Match(data) {
				write([]byte(` src=#noscript`))
			} else {
				write(data)
			}
		default:
			write(data)
		}
	}
	return finalErr
}

var scriptTagRegexp = regexp.MustCompile(`(?si)^\s*<script$`)
var scriptTagEndRegexp = regexp.MustCompile(`(?si)^\s*</\s*script\s*>`)

var noscriptTagRegexp = regexp.MustCompile(`(?si)^\s*<noscript$`)
var noscriptTagEndRegexp = regexp.MustCompile(`(?si)^\s*</\s*noscript\s*>`)

var eventAttribRegexp = regexp.MustCompile(`(?si)^\s*on\w`)
var hrefJsAttribRegexp = regexp.MustCompile(`(?si)^\s*href\s*=.*javascript:`)
var srcJsAttribRegexp = regexp.MustCompile(`(?si)^\s*src\s*=.*javascript:`)
