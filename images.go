// Copyright (C) 2019 Christopher E. Miller
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package smallprox

import (
	"bytes"
	"image/jpeg"

	"io/ioutil"
	"log"
	"net/http"
	"strings"

	"github.com/chai2010/webp"
	"github.com/disintegration/imaging"
	"golang.org/x/exp/errors/fmt"
)

type ImageShrinkResponder struct {
	toggle
}

func (er *ImageShrinkResponder) Response(req *http.Request, resp *http.Response) *http.Response {
	if !er.Enabled() {
		return resp
	}
	respContentType := resp.Header.Get("Content-Type")
	reqAccept := req.Header["Accept"]
	if strings.HasPrefix(respContentType, "image/") &&
		respContentType != "image/x-icon" && respContentType != "image/vnd.microsoft.icon" && // not supported
		!strings.HasSuffix(respContentType, "+xml") &&
		!strings.HasSuffix(respContentType, "+json") {
		const webpType = "image/webp"
		const jpegType = "image/jpeg"
		canWebp := HasAnyHeaderValuePart(reqAccept, webpType) ||
			(respContentType == webpType && (len(reqAccept) == 0 || HasAnyHeaderValuePart(reqAccept, "*/*")))
		outbuf := &Mutable{}
		// TODO: use specific decoder per the content type...
		//img, _, err := image.Decode(r)
		img, err := imaging.Decode(resp.Body, imaging.AutoOrientation(true))
		resp.Body.Close()
		if err != nil {
			log.Printf("Error loading image as %s: %s", respContentType, err)
			resp.Body = ioutil.NopCloser(bytes.NewBuffer(badImg))
			resp.Header.Set("Content-Type", badImgType)
			resp.StatusCode = http.StatusInternalServerError
			resp.Status = fmt.Sprintf("%v %v", resp.StatusCode, http.StatusText(resp.StatusCode))
		} else {
			const maxImgDim = 1024
			w := img.Bounds().Dx()
			h := img.Bounds().Dy()
			if w > maxImgDim || h > maxImgDim {
				var r float64
				if w > h {
					r = maxImgDim / float64(w)
				} else {
					r = maxImgDim / float64(h)
				}
				w = int(float64(w) * r)
				h = int(float64(h) * r)
				img = imaging.Resize(img, w, h, imaging.Lanczos)
			}
			destType := jpegType
			if canWebp {
				destType = webpType
				err = webp.Encode(outbuf, img, &webp.Options{Quality: 10})
			} else {
				// jpeg needs higher quality, https://developers.google.com/speed/webp/docs/webp_study
				jpeg.Encode(outbuf, img, &jpeg.Options{Quality: 20})
			}
			if err != nil {
				log.Printf("Error converting image from %s to %s: %s ", respContentType, destType, err)
				resp.Body = ioutil.NopCloser(bytes.NewBuffer(badImg))
				resp.Header.Set("Content-Type", badImgType)
				resp.StatusCode = http.StatusInternalServerError
				resp.Status = fmt.Sprintf("%v %v", resp.StatusCode, http.StatusText(resp.StatusCode))
			} else {
				resp.Body = outbuf
				resp.Header.Set("Content-Type", destType)
			}
		}
	}
	return resp
}

var badImg = mustDecodeStringBase64(`iVBORw0KGgoAAAANSUhEUgAAACIAAAAiCAMAAAANmfvwAAABa1BMVEUAAAAiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyIiiyKKpJssAAAAeHRSTlMAAQIDBAUGBwgJCwwNDg8SFBUWFxgZGhwdHiAhIyQmJyosLzEyNjg5Ojw9P0BBSUpMTU5RUlRWV1hZW1xdXmFjZ2xtcXR1eHl+gIOFi46Rl5qdnqCio6WmqqutsLK8wMPFx8jKzM/R09XZ2tze4uTo6fHz9ff5+/1dImJTAAABnklEQVQYGa3BaVsSYQCG0WcaoaAUFDfKNAmkXa0sK9M2K5dKNCu3ytCC0IxY5v75zQsOFyB88xydvr7pVLZQyKYe9qq15AE1uTGdNLhHRXY3j5EeUJNxXB/Hzsl15tLrEnBHDeaBzR7VnH0LzKvOBPBADWJlGFdNFIipSaQMUXn2YULyjQbkGQ5JcdjTsRuwKtl/KAVVtQiD0hIkVXUIF6QeIK6qI5iSApBTxQAsS7J2yPlU9Yh8l6QF6JcxA1dkdFryBG25RuCxjHWw1ZINazIOKKqNIr9llEirjTRFGQX21cZP/snIUFYbDhkZH8AnIzC9+KxfdfywIuMuxOW6Wca1pIqOTktKwD0ZIViRFOUwFujdYEbGJBFpDbpV8R3C0joRSVam6Jcr5XQoAr9UNQRbElkZo3zySZO8kbYhpmOfYVbXr6piFufrXzYszcE3eYJ5uCVP5NXm+5h0G5xu1VwEnqvBCyCpOgngx5BqLqeBp2rQdwTs3g/7LMsfnkoDTkJN/As02ArppK5lB8/OiFqzh5+8+7K9+vLaeZ26/w2hg7si3OgUAAAAAElFTkSuQmCC`)
var badImgType = "image/png"
