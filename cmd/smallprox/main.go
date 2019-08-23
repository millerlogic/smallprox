// Copyright (C) 2019 Christopher E. Miller
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this file,
// You can obtain one at https://mozilla.org/MPL/2.0/.

package main

import (
	"crypto/tls"
	"flag"
	"io/ioutil"
	"log"
	"math"
	"os"
	"strings"

	humanize "github.com/dustin/go-humanize"
	"github.com/millerlogic/smallprox"
	"golang.org/x/exp/errors"
	"golang.org/x/exp/errors/fmt"
)

func run() error {
	limiter := &smallprox.LimitBytesResponder{}
	limiter.SetLimit(1024 * 1024 * 100)
	flimiter := &limiterFlag{limiter: limiter}

	imageShrinker := &smallprox.ImageShrinkResponder{}
	imageShrinker.SetEnabled(false)
	fimageShrinker := &toggleFlag{toggle: imageShrinker}

	noscript := &smallprox.NoscriptResponder{}
	noscript.SetEnabled(false)
	fnoscript := &toggleFlag{toggle: noscript}

	compressor := &smallprox.CompressResponder{}
	compressor.SetEnabled(true)
	fcompressor := &toggleFlag{toggle: compressor}

	opts := smallprox.Options{
		ConnectMITM: true,
	}
	fs := flag.CommandLine
	fs.BoolVar(&opts.Verbose, "v", opts.Verbose, "Verbose output")
	fs.Var((*arrayFlags)(&opts.Addresses), "addr", "Proxy listen address(es)")
	fs.BoolVar(&opts.InsecureSkipVerify, "insecure", opts.InsecureSkipVerify, "TLS config InsecureSkipVerify")
	var blockHostsFiles []string
	fs.Var((*arrayFlags)(&blockHostsFiles), "blockHostsFile", "Block the hosts found in this file(s), one per line or in /etc/hosts format")
	fs.BoolVar(&opts.ConnectMITM, "connectMITM", opts.ConnectMITM, "Enable man-in-the-middle for HTTP CONNECT connections")
	fs.BoolVar(&opts.HTTPSMITM, "httpsMITM", opts.HTTPSMITM, "Enable man-in-the-middle for HTTPS CONNECT connections")
	var cacert, cakey string
	fs.StringVar(&cacert, "cacert", cacert, "CA certificate file for HTTPS MITM")
	fs.StringVar(&cakey, "cakey", cakey, "CA private key file for HTTPS MITM")
	fs.Var(fnoscript, "noscript", "Remove JavaScript from HTML content *")
	fs.Var(fcompressor, "compress", "Compress highly compressable content *")
	fs.Var(flimiter, "limitContent", "Limit content to minimize excessive memory usage *")
	fs.Var(fimageShrinker, "shrinkImages", "Make images/pictures smaller *")
	fs.StringVar(&opts.Auth, "auth", opts.Auth, "Proxy authentication, username:password")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(fs.Output(), "* only applies to CONNECT if MITM enabled\n")
	}
	fs.Parse(os.Args[1:])

	for _, fp := range blockHostsFiles {
		blockHosts, err := smallprox.LoadHostsFile(fp)
		if err != nil {
			return fmt.Errorf("-blockHostsFile error: %w", err)
		}
		opts.BlockHosts = append(opts.BlockHosts, blockHosts...)
	}

	if cacert != "" || cakey != "" {
		if !opts.HTTPSMITM {
			return errors.New("-cacert and -cakey require -httpsMITM")
		}
		caCert, err := ioutil.ReadFile(cacert)
		if err != nil {
			return fmt.Errorf("-cacert error: %w", err)
		}
		caKey, err := ioutil.ReadFile(cakey)
		if err != nil {
			return fmt.Errorf("-cakey error: %w", err)
		}
		proxyCa, err := tls.X509KeyPair(caCert, caKey)
		if err != nil {
			return fmt.Errorf("CA error: %w", err)
		}
		opts.CA = proxyCa
	}

	proxy := smallprox.NewProxy(opts)

	proxy.AddResponder(limiter) // First.
	proxy.AddResponder(imageShrinker)
	proxy.AddResponder(noscript)
	proxy.AddResponder(compressor)

	return proxy.ListenAndServe()
}

func main() {
	err := run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %+v\n", err)
		log.Fatalf("ERROR %v", err)
	}
}

type arrayFlags []string

var _ flag.Getter = &arrayFlags{}

func (f *arrayFlags) String() string {
	return strings.Join(*f, ", ")
}

func (f *arrayFlags) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func (f *arrayFlags) Get() interface{} {
	return []string(*f)
}

type toggleFlag struct {
	toggle interface {
		Enabled() bool
		SetEnabled(bool)
	}
}

func (f *toggleFlag) IsBoolFlag() bool {
	return true
}

func (f *toggleFlag) String() string {
	if f.toggle != nil && f.toggle.Enabled() {
		return "true"
	}
	return "false"
}

func (f *toggleFlag) Set(x string) error {
	if x == "" {
		return errors.New("Value expected")
	}
	switch x[0] {
	case '1', 't', 'T':
		f.toggle.SetEnabled(true)
	case '0', 'f', 'F':
		f.toggle.SetEnabled(false)
	default:
		return errors.New("Unexpected value")
	}
	return nil
}

func (f *toggleFlag) Get() interface{} {
	if f.toggle.Enabled() {
		return true
	}
	return false
}

type limiterFlag struct {
	limiter interface {
		Limit() int64
		SetLimit(int64)
	}
}

func (f *limiterFlag) String() string {
	if f.limiter == nil {
		return "0"
	}
	limit := f.limiter.Limit()
	//return strconv.FormatInt(limit, 10)
	if limit <= 0 {
		return "0"
	}
	return strings.Replace(humanize.IBytes(uint64(limit)), " ", "", -1)
}

func (f *limiterFlag) Set(x string) error {
	bytes, err := humanize.ParseBytes(x)
	if err != nil {
		return err
	}
	if bytes > math.MaxInt64 {
		return errors.New("Too large")
	}
	f.limiter.SetLimit(int64(bytes))
	return nil
}

func (f *limiterFlag) Get() interface{} {
	return f.limiter.Limit()
}
