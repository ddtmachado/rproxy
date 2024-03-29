package main

import (
	"bufio"
	"bytes"
	"crypto/sha1"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"
)

const cacheExpirationInSec = 10

func main() {
	var dst = flag.String("url", "", "destination URL to be proxied")
	flag.Parse()

	dstURL, err := url.Parse(*dst)
	if err != nil {
		log.Fatalf("Invalid URL: %s", *dst)
	}

	proxy := NewProxy(dstURL)
	proxy.Run(":8080")
}

//MemCache is a simple cache structure to hold the data and an expiration time
type MemCache struct {
	data      []byte
	expiresAt time.Time
}

//NewMemCache returns a new MemCache with the default expiration time
func NewMemCache(data []byte) *MemCache {
	expireIn, _ := time.ParseDuration(fmt.Sprintf("%ds", cacheExpirationInSec))

	return &MemCache{data, time.Now().Add(expireIn)}
}

//MyProxy is a simple reverse proxy supporting a minimal cache feature
type MyProxy struct {
	target      *url.URL
	memoryCache map[string]*MemCache
	transport   *http.Transport
}

//NewProxy return an initialized MyProxy struct pointing to the target URL
func NewProxy(target *url.URL) *MyProxy {
	return &MyProxy{
		target:      target,
		memoryCache: make(map[string]*MemCache),
		transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout: 5 * time.Second,
			}).Dial},
	}
}

//Run starts the proxy server at the specified local address
func (p *MyProxy) Run(addr string) {
	http.ListenAndServe(addr, p)
}

func (p *MyProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("request received %v", r)

	//Fetch response from the proxied URL
	res, err := p.fetchResponse(r)
	if err != nil {
		log.Printf("error while trying to proxy\n request: %v\n response:%v\n%v", r, res, err)
		return
	}

	//Write response to the client
	copyHeader(w.Header(), res.Header)
	w.WriteHeader(res.StatusCode)
	_, err = io.Copy(w, res.Body)
	if err != nil {
		log.Printf("error copying response body: %v", err)
	}
	defer res.Body.Close()
}

func (p *MyProxy) fetchResponse(r *http.Request) (*http.Response, error) {
	var response *http.Response

	shouldCache := r.Method == http.MethodGet
	rHash := hashRequest(r)
	//Check cache
	if cacheData, exists := p.memoryCache[rHash]; exists && shouldCache {
		if time.Now().Before(cacheData.expiresAt) {
			log.Printf("cache hit %s", rHash)
			respReader := bufio.NewReader(bytes.NewReader(cacheData.data))
			response, err := http.ReadResponse(respReader, nil)
			if err != nil {
				log.Fatalf("error decoding cached response: %v", err)
			}
			response.Header.Set("X-Cached", "true")
			return response, nil
		}
		log.Printf("cache expired %s", rHash)
		delete(p.memoryCache, rHash)
	}

	//Creates the new request to the target proxy URL
	pRequest := r.WithContext(r.Context())
	pRequest.Header = cloneHeader(r.Header)
	pRequest.URL = p.target.ResolveReference(asRelativeURL(r.URL))
	pRequest.Host = p.target.Host
	pRequest.URL.Scheme = p.target.Scheme
	pRequest.URL.Host = p.target.Host
	pRequest.Header.Set("User-Agent", "")
	pRequest.Header.Add("X-Forwarded-Host", r.Host)
	pRequest.Header.Add("X-Origin-Host", p.target.Host)
	if clientIP, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		pRequest.Header.Set("X-Forwarded-For", clientIP)
	}
	pRequest.Close = false

	log.Printf("sending request: %v", pRequest)
	response, err := p.transport.RoundTrip(pRequest)

	//Add to cache
	if shouldCache && err == nil {
		dumpedBody, err := httputil.DumpResponse(response, true)
		p.memoryCache[rHash] = NewMemCache(dumpedBody)
		if err != nil {
			log.Fatal(err)
		}
	}
	return response, err
}

//asRelativeURL returns a new url.URL without absolute path information
func asRelativeURL(u *url.URL) *url.URL {
	return &url.URL{
		Scheme:     "",
		Opaque:     u.Opaque,
		User:       nil,
		Host:       "",
		Path:       u.Path,
		RawPath:    u.RawPath,
		ForceQuery: u.ForceQuery,
		RawQuery:   u.RawQuery,
		Fragment:   u.Fragment,
	}
}

func hashRequest(r *http.Request) string {
	//Trying to hash an http request without bothering
	//with it's internal structure and order of attributes
	hashedBytes := sha1.New()
	hashedBytes.Write([]byte(r.URL.String()))
	return fmt.Sprintf("%x", hashedBytes.Sum(nil))
}

func cloneHeader(h http.Header) http.Header {
	//Straight from official go code
	h2 := make(http.Header, len(h))
	for k, vv := range h {
		vv2 := make([]string, len(vv))
		copy(vv2, vv)
		h2[k] = vv2
	}
	return h2
}

func copyHeader(dst, src http.Header) {
	//Straight from official go code
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}
