// Copyright (C) 2019 Christopher E. Miller
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package smallprox

import (
	"compress/flate"
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	"github.com/andybalholm/brotli"
)

type CompressResponder struct {
	toggle
}

func (er *CompressResponder) Response(req *http.Request, resp *http.Response) *http.Response {
	if !er.Enabled() {
		return resp
	}
	acceptEncs := req.Header["Accept-Encoding"]
	canBrotli := HasAnyHeaderValuePart(acceptEncs, "br")
	canGzip := HasAnyHeaderValuePart(acceptEncs, "gzip")
	canDeflate := HasAnyHeaderValuePart(acceptEncs, "deflate")
	if canBrotli || canGzip || canDeflate {
		respContentType := resp.Header.Get("Content-Type")
		respMIMEType := respContentType
		{
			isem := strings.IndexByte(respMIMEType, ';')
			if isem != -1 {
				respMIMEType = respMIMEType[:isem]
			}
		}
		if strings.HasPrefix(respMIMEType, "text/") ||
			respMIMEType == "application/javascript" ||
			respMIMEType == "application/json" ||
			strings.HasSuffix(respMIMEType, "+xml") ||
			strings.HasSuffix(respMIMEType, "+json") {
			if compressCheck(resp) {
				outbuf := &Mutable{}
				var dest io.WriteCloser
				var enc string
				if canBrotli {
					dest = brotli.NewWriterLevel(outbuf, brotli.DefaultCompression)
					enc = "br"
				} else if canGzip {
					// https://blog.klauspost.com/go-gzipdeflate-benchmarks/
					dest, _ = gzip.NewWriterLevel(outbuf, 5)
					enc = "gzip"
				} else if canDeflate {
					// https://blog.klauspost.com/go-gzipdeflate-benchmarks/
					dest, _ = flate.NewWriter(outbuf, 5)
					enc = "deflate"
				}
				io.Copy(dest, resp.Body)
				dest.Close() // Finish the compression.
				resp.Body.Close()
				resp.Body = outbuf
				resp.Header.Set("Content-Encoding", enc)
				// TODO: Content-Length ....
			}
		}
	}
	return resp
}

func compressCheck(resp *http.Response) bool {
	var pr peekCloser
	if x, ok := resp.Body.(peekCloser); ok {
		pr = x
	} else {
		pr = newPeekCloser(resp.Body)
		resp.Body = pr
	}
	const largeEnough = 256
	peekbuf, _ := pr.Peek(largeEnough)
	if len(peekbuf) < largeEnough {
		// Don't bother compressing smaller files.
		return false
	}
	return true
}
