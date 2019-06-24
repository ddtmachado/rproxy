package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"bou.ke/monkey"
	"github.com/stretchr/testify/assert"
)

type TestServer struct{}

func (ts *TestServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Test Server Visited")
}

func TestSimpleRequest(t *testing.T) {
	testServer := httptest.NewServer(&TestServer{})
	defer testServer.Close()

	u, _ := url.Parse(testServer.URL)
	proxy := NewProxy(u)

	w := httptest.NewRecorder()
	body := bytes.NewBufferString("")
	req, _ := http.NewRequest("GET", "/", body)
	proxy.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Test Server Visited", w.Body.String())
}

func TestCachedRequest(t *testing.T) {
	testServer := httptest.NewServer(&TestServer{})
	defer testServer.Close()

	u, _ := url.Parse(testServer.URL)
	proxy := NewProxy(u)

	//First request, added to cache
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/", bytes.NewBufferString(""))
	proxy.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Test Server Visited", w.Body.String())
	assert.Equal(t, "", w.Header().Get("X-Cached"))

	//Cache hit
	req, _ = http.NewRequest("GET", "/", bytes.NewBufferString(""))
	w = httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Test Server Visited", w.Body.String())
	assert.Equal(t, "true", w.Header().Get("X-Cached"))
}

func TestExpiredCachedRequest(t *testing.T) {
	testServer := httptest.NewServer(&TestServer{})
	defer testServer.Close()

	u, _ := url.Parse(testServer.URL)
	proxy := NewProxy(u)

	//First request, added to cache
	w := httptest.NewRecorder()
	body := bytes.NewBufferString("")
	req, _ := http.NewRequest("GET", "/", body)
	proxy.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Test Server Visited", w.Body.String())
	assert.Equal(t, "", w.Header().Get("X-Cached"))

	//Cache hit
	req, _ = http.NewRequest("GET", "/", bytes.NewBufferString(""))
	w = httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Test Server Visited", w.Body.String())
	assert.Equal(t, "true", w.Header().Get("X-Cached"))

	waitExpiration, _ := time.ParseDuration(fmt.Sprintf("%ds", cacheExpirationInSec+1))
	futureTime := time.Now().Add(waitExpiration)
	timePatch := monkey.Patch(time.Now, func() time.Time { return futureTime })
	defer timePatch.Unpatch()

	//Cache expired
	req, _ = http.NewRequest("GET", "/", bytes.NewBufferString(""))
	w = httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "Test Server Visited", w.Body.String())
	assert.Equal(t, "", w.Header().Get("X-Cached"))
}
