package main

import (
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
)

func main() {
	var dst = flag.String("url", "", "destination URL to be proxied")
	flag.Parse()

	dstURL, err := url.Parse(*dst)
	if err != nil {
		log.Fatalf("Invalid URL: %s", *dst)
	}

	proxy := NewProxy(dstURL)
	proxy.Run()
}

type MyProxy struct {
	target *url.URL
}

func NewProxy(target *url.URL) *MyProxy {
	return &MyProxy{target}
}

func (p *MyProxy) Run() {
	http.ListenAndServe(":8080", p)
}

func (p *MyProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("request received %v", r)

	//Creates the new request to the target proxy URL
	pRequest := r.WithContext(r.Context())
	pRequest.Header = cloneHeader(r.Header)
	pRequest.URL = p.target.ResolveReference(r.URL)
	pRequest.URL.Scheme = p.target.Scheme
	pRequest.URL.Host = p.target.Host
	pRequest.Header.Set("User-Agent", "")
	pRequest.Close = false
	if clientIP, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		pRequest.Header.Set("X-Forwarded-For", clientIP)
	}

	log.Printf("sending request: %v", pRequest)
	res, err := http.DefaultTransport.RoundTrip(pRequest)
	if err != nil {
		log.Printf("error while trying to proxy request: %v\n%v", pRequest, err)
		return
	}
	defer res.Body.Close()

	copyHeader(w.Header(), res.Header)
	w.WriteHeader(res.StatusCode)

	buf := make([]byte, 16*1024)
	_, err = io.CopyBuffer(w, res.Body, buf)
	if err != nil {
		log.Printf("error copying response body: %v", err)
	}
}

func cloneHeader(h http.Header) http.Header {
	h2 := make(http.Header, len(h))
	for k, vv := range h {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		h2[k] = vv2
	}
	return h2
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
