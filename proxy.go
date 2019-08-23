package smallprox

import (
	"bytes"
	"context"
	"crypto/tls"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/elazarl/goproxy"
	"github.com/elazarl/goproxy/ext/auth"
	"golang.org/x/exp/errors"
	"golang.org/x/exp/errors/fmt"
	"golang.org/x/sync/errgroup"
)

// Requester allows handling a request.
// Return a non-nil response to finish the request early.
// Use req.Context()
type Requester interface {
	Request(req *http.Request) (*http.Request, *http.Response)
}

// Responder allows handling a response.
// Use req.Context()
type Responder interface {
	Response(req *http.Request, resp *http.Response) *http.Response
}

// Options includes the proxy options.
type Options struct {
	Verbose            bool
	Addresses          []string
	InsecureSkipVerify bool
	BlockHosts         []string // list of hosts
	ConnectMITM        bool
	HTTPSMITM          bool
	CA                 tls.Certificate // Do not modify the pointers/arrays!
	Auth               string
}

// Copy performs a readonly copy.
func (opts *Options) Copy() Options {
	newopts := *opts
	newopts.Addresses = append([]string(nil), opts.Addresses...)
	newopts.BlockHosts = append([]string(nil), opts.BlockHosts...)
	return newopts
}

// Proxy is the proxy.
type Proxy struct {
	mx            sync.RWMutex
	opts          Options
	dialer        net.Dialer
	server        *goproxy.ProxyHttpServer
	httpservers   []*http.Server
	tlsConfigFunc func(host string, ctx *goproxy.ProxyCtx) (*tls.Config, error)
	ctx           context.Context
	cancel        func()
	requesters    []Requester // Do not remove from this array, see getRequesters
	responders    []Responder // Do not remove from this array, see getResponders
}

// NewProxy creates a new proxy.
func NewProxy(opts Options) *Proxy {
	proxy := &Proxy{
		opts:   opts.Copy(),
		dialer: net.Dialer{Timeout: 30 * time.Second, KeepAlive: 30 * time.Second},
		server: goproxy.NewProxyHttpServer(),
		ctx:    context.Background(),
	}
	proxy.server.Tr = &http.Transport{
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: proxy.opts.InsecureSkipVerify},
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           proxy.dialContext,
		Dial:                  proxy.dial, // Directly used by default ConnectDial.
		MaxConnsPerHost:       50,
		IdleConnTimeout:       5 * time.Minute,
		ResponseHeaderTimeout: 30 * time.Second,
	}
	proxy.server.Verbose = proxy.opts.Verbose
	proxy.server.CertStore = newCertStore()
	proxy.optsChanged() // Lock not needed yet.
	proxy.addHandlers()
	return proxy
}

// GetOptions gets the current options,
func (proxy *Proxy) GetOptions() Options {
	proxy.mx.RLock()
	opts := proxy.opts
	proxy.mx.RUnlock()
	return opts.Copy()
}

func (proxy *Proxy) SetOptions(opts Options) {
	// Clone the arrays for safety, they will be readonly memory.
	opts = opts.Copy()
	// Set the new opts by value.
	proxy.mx.Lock()
	defer proxy.mx.Unlock()
	proxy.opts = opts
	proxy.optsChanged()
}

func (proxy *Proxy) AddRequester(req Requester) {
	proxy.mx.Lock()
	defer proxy.mx.Unlock()
	proxy.requesters = append(proxy.requesters, req)
}

func (proxy *Proxy) AddResponder(resp Responder) {
	proxy.mx.Lock()
	defer proxy.mx.Unlock()
	proxy.responders = append(proxy.responders, resp)
}

func (proxy *Proxy) getRequesters() []Requester {
	proxy.mx.RLock()
	x := proxy.requesters
	proxy.mx.RUnlock()
	return x
}

func (proxy *Proxy) getResponders() []Responder {
	proxy.mx.RLock()
	x := proxy.responders
	proxy.mx.RUnlock()
	return x
}

func (proxy *Proxy) IsHostBlocked(host string) bool {
	proxy.mx.RLock()
	defer proxy.mx.RUnlock()
	return ContainsHost(proxy.opts.BlockHosts, host)
}

func (proxy *Proxy) dialContext(ctx context.Context, network string, addr string) (net.Conn, error) {
	//log.Printf("Dial: %s %s", network, addr)
	host := addr
	ilcolon := strings.LastIndexByte(host, ':')
	if ilcolon != -1 {
		host = addr[:ilcolon]
	}
	if proxy.IsHostBlocked(host) {
		return nil, &net.DNSError{Err: "Blocked", Name: host}
	}
	return proxy.dialer.DialContext(ctx, network, addr)
}

func (proxy *Proxy) dial(network string, addr string) (net.Conn, error) {
	return proxy.dialContext(context.Background(), network, addr)
}

// Call within lock.
func (proxy *Proxy) optsChanged() {
	ca := proxy.opts.CA
	if ca.PrivateKey == nil {
		ca = goproxy.GoproxyCa
	}
	proxy.tlsConfigFunc = func(host string, ctx *goproxy.ProxyCtx) (*tls.Config, error) {
		hostname := host
		{
			ix := strings.IndexRune(hostname, ':')
			if ix != -1 {
				hostname = hostname[:ix]
			}
		}
		getCert := func() (*tls.Certificate, error) {
			//log.Printf("getCert for %s", hostname)
			return signHost(ca, []string{hostname})
		}
		var cert *tls.Certificate
		var err error
		if proxy.server.CertStore != nil {
			cert, err = proxy.server.CertStore.Fetch(hostname, getCert)
		} else {
			cert, err = getCert()
		}
		if err != nil {
			return nil, err
		}
		return &tls.Config{Certificates: []tls.Certificate{*cert}}, nil
	}
}

/*
func (proxy *Proxy) Context() context.Context {
	return proxy.ctx
}
*/

type ctx2 struct {
	context.Context
	Context2 context.Context
}

func (ctx *ctx2) Value(key interface{}) interface{} {
	val := ctx.Context.Value(key)
	if val == nil {
		val = ctx.Context2.Value(key)
	}
	return val
}

func (proxy *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if proxy.ctx != nil {
		// TODO: revisit this...
		// Consider github.com/teivah/onecontext
		r = r.WithContext(&ctx2{proxy.ctx, r.Context()})
	}
	proxy.server.ServeHTTP(w, r)
}

func (proxy *Proxy) ListenAndServeContext(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	eg, ctx := errgroup.WithContext(ctx)
	finalErr := func() error {
		proxy.mx.Lock()
		defer proxy.mx.Unlock()
		if proxy.httpservers != nil {
			return errors.New("Already listening")
		}
		if len(proxy.opts.Addresses) == 0 {
			return errors.New("No addresses")
		}
		for _, addr := range proxy.opts.Addresses {
			proxy.httpservers = append(proxy.httpservers, &http.Server{Addr: addr, Handler: proxy})
		}
		proxy.cancel = cancel
		proxy.ctx = ctx
		for _, httpserver := range proxy.httpservers {
			eg.Go(func() error {
				return httpserver.ListenAndServe()
			})
		}
		return nil
	}()
	if finalErr != nil {
		return finalErr
	}
	return eg.Wait()
}

func (proxy *Proxy) ListenAndServe() error {
	return proxy.ListenAndServeContext(context.Background())
}

func (proxy *Proxy) Shutdown(ctx context.Context) error {
	proxy.cancel()
	eg := &errgroup.Group{}
	func() {
		proxy.mx.Lock()
		defer proxy.mx.Unlock()
		for _, httpserver := range proxy.httpservers {
			eg.Go(func() error {
				return httpserver.Shutdown(ctx)
			})
		}
	}()
	return eg.Wait()
}

func (proxy *Proxy) getAuth() string {
	proxy.mx.RLock()
	x := proxy.opts.Auth
	proxy.mx.RUnlock()
	return x
}

func (proxy *Proxy) getConnectMITM() bool {
	proxy.mx.RLock()
	x := proxy.opts.ConnectMITM
	proxy.mx.RUnlock()
	return x
}

func (proxy *Proxy) getHTTPSMITM() bool {
	proxy.mx.RLock()
	x := proxy.opts.HTTPSMITM
	proxy.mx.RUnlock()
	return x
}

func (proxy *Proxy) authCheck(u, p string) bool {
	proxyauth := proxy.getAuth()
	icolon := strings.IndexByte(proxyauth, ':')
	if icolon == -1 {
		return false
	}
	username := proxyauth[:icolon]
	password := proxyauth[icolon+1:]
	return strings.EqualFold(u, username) && p == password
}

type reqData struct {
	acceptEncoding string // original
	withinCONNECT  bool
}

func getReqData(ctx *goproxy.ProxyCtx) *reqData {
	if ctx.UserData != nil {
		return ctx.UserData.(*reqData)
	}
	rd := &reqData{}
	ctx.UserData = rd
	return rd
}

func (proxy *Proxy) addHandlers() {
	// Auth:
	realm := "Proxy"
	authBasic := auth.Basic(realm, proxy.authCheck)
	proxy.server.OnRequest().DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		if proxy.getAuth() != "" {
			rd := getReqData(ctx)
			if !rd.withinCONNECT {
				// ONLY do this if we haven't already done this for a CONNECT!
				return authBasic.Handle(req, ctx)
			}
		}
		return req, nil
	})
	authBasicConnect := auth.BasicConnect(realm, proxy.authCheck)
	proxy.server.OnRequest().HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		rd := getReqData(ctx)
		rd.withinCONNECT = true
		if proxy.getAuth() != "" {
			todo, newhost := authBasicConnect.HandleConnect(host, ctx)
			host = newhost
			if todo != nil && todo.Action == goproxy.ConnectReject {
				return &goproxy.ConnectAction{
					Action: goproxy.ConnectReject,
				}, host
			}
			// Granted...
		}
		return nil, host
	})

	// Handle CONNECT for text port 80:
	proxy.server.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile(":80$"))).HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		rd := getReqData(ctx)
		rd.withinCONNECT = true
		//log.Printf("handle connect func for :80 %+v", ctx)
		if proxy.getConnectMITM() {
			return &goproxy.ConnectAction{
				Action: goproxy.ConnectHTTPMitm,
			}, host
		}
		return &goproxy.ConnectAction{
			Action: goproxy.ConnectAccept,
		}, host
	})
	// Handle CONNECT for TLS port 443:
	proxy.server.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile(":443$"))).HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		rd := getReqData(ctx)
		rd.withinCONNECT = true
		//log.Printf("handle connect func for :443 %+v", ctx)
		if proxy.getHTTPSMITM() {
			return &goproxy.ConnectAction{
				Action:    goproxy.ConnectMitm,
				TLSConfig: proxy.tlsConfigFunc,
			}, host
		}
		return &goproxy.ConnectAction{
			Action: goproxy.ConnectAccept,
		}, host
	})
	// Handle CONNECT for any other port:
	proxy.server.OnRequest(goproxy.ReqHostMatches(regexp.MustCompile(""))).HandleConnectFunc(func(host string, ctx *goproxy.ProxyCtx) (*goproxy.ConnectAction, string) {
		//log.Printf("handle connect func for OTHER %+v", ctx)
		return &goproxy.ConnectAction{
			Action: goproxy.ConnectReject,
		}, host
	})

	// Handle requests:
	proxy.server.OnRequest().DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		//log.Printf("got regular OnRequest().DoFunc %+v", req)
		req = req.WithContext(proxy.ctx)
		rd := getReqData(ctx)
		rd.acceptEncoding = req.Header.Get("Accept-Encoding") // Preserve original.
		for _, er := range proxy.getRequesters() {
			newReq, resp := er.Request(req)
			if resp != nil {
				if m, ok := resp.Body.(*Mutable); ok {
					resp.ContentLength = int64(m.Len())
				}
				return newReq, resp
			}
			req = newReq
		}
		return req, nil
	})

	// Handle responses:
	proxy.server.OnResponse().DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		//log.Printf("got regular OnResponse().DoFunc %+v", resp)
		rd := getReqData(ctx)
		if resp == nil {
			// Apparently this can happen if there was an error during the server request.
			log.Printf("Error during round trip: %+v", ctx.Error)
			//return nil
			var status int
			var statusText string
			xerr := ctx.Error
			if netoperr, ok := xerr.(*net.OpError); ok {
				xerr = netoperr.Err // Unwrap
			}
			if _, ok := xerr.(*net.DNSError); ok {
				status = 521
				statusText = "Down"
			} else {
				status = http.StatusBadGateway
				statusText = http.StatusText(status)
			}
			return &http.Response{
				Request:    ctx.Req,
				Header:     make(http.Header),
				StatusCode: status,
				Status:     fmt.Sprintf("%v %v", status, statusText),
				Body:       ioutil.NopCloser(bytes.NewBufferString(statusText)),
			}
		}
		// The timeout is only for the responders, not the returned stream.
		goctx, cancel := context.WithTimeout(proxy.ctx, time.Minute*1) // TODO: revisit... too short?
		req := ctx.Req.WithContext(goctx)
		defer cancel()
		if rd.acceptEncoding != "" {
			// Put back the accept encoding so I know what the client supports.
			ctx.Req.Header.Set("Accept-Encoding", rd.acceptEncoding)
		}
		for _, er := range proxy.getResponders() {
			resp = er.Response(req, resp)
		}
		if m, ok := resp.Body.(*Mutable); ok {
			resp.ContentLength = int64(m.Len())
		}
		//log.Printf("Final response: %+v", resp)
		return resp
	})
}
