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
			outbuf := &Mutable{}
			var dest io.WriteCloser
			var enc string
			if canBrotli {
				dest = brotli.NewWriterLevel(outbuf, brotli.BestCompression)
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
			resp.Body.Close()
			dest.Close() // Finish the compression.
			resp.Body = outbuf
			resp.Header.Set("Content-Encoding", enc)
			// TODO: Content-Length ....
		}
	}
	return resp
}
