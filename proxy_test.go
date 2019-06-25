package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"bou.ke/monkey"
	"github.com/stretchr/testify/assert"
)

const (
	testGetResponse  = "Test Server Visited"
	testPostResponse = "Post Data Received"
)

type TestServer struct{}

func (ts *TestServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		fmt.Fprintf(w, testPostResponse)
		break
	default:
		fmt.Fprint(w, testGetResponse)
	}
}

func TestSimpleRequest(t *testing.T) {
	testServer := httptest.NewServer(&TestServer{})
	defer testServer.Close()

	u, _ := url.Parse(testServer.URL)
	proxy := NewProxy(u)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", bytes.NewBufferString(""))
	proxy.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, testGetResponse, w.Body.String())
}

func TestCachedRequest(t *testing.T) {
	testServer := httptest.NewServer(&TestServer{})
	defer testServer.Close()

	u, _ := url.Parse(testServer.URL)
	proxy := NewProxy(u)

	//First request, added to cache
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", bytes.NewBufferString(""))
	proxy.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, testGetResponse, w.Body.String())
	assert.False(t, isCachedResponse(w))

	//Cache hit
	req, _ = http.NewRequest(http.MethodGet, "/", bytes.NewBufferString(""))
	w = httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, testGetResponse, w.Body.String())
	assert.True(t, isCachedResponse(w))
}

func TestExpiredCachedRequest(t *testing.T) {
	testServer := httptest.NewServer(&TestServer{})
	defer testServer.Close()

	u, _ := url.Parse(testServer.URL)
	proxy := NewProxy(u)

	//First request, added to cache
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/", bytes.NewBufferString(""))
	proxy.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, testGetResponse, w.Body.String())
	assert.False(t, isCachedResponse(w))

	//Cache hit
	req, _ = http.NewRequest(http.MethodGet, "/", bytes.NewBufferString(""))
	w = httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, testGetResponse, w.Body.String())
	assert.True(t, isCachedResponse(w))

	waitExpiration, _ := time.ParseDuration(fmt.Sprintf("%ds", cacheExpirationInSec+1))
	futureTime := time.Now().Add(waitExpiration)
	timePatch := monkey.Patch(time.Now, func() time.Time { return futureTime })
	defer timePatch.Unpatch()

	//Cache expired
	req, _ = http.NewRequest(http.MethodGet, "/", bytes.NewBufferString(""))
	w = httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, testGetResponse, w.Body.String())
	assert.False(t, isCachedResponse(w))
}

func TestPostRequest(t *testing.T) {
	testServer := httptest.NewServer(&TestServer{})
	defer testServer.Close()

	u, _ := url.Parse(testServer.URL)
	proxy := NewProxy(u)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPost, "/", bytes.NewBufferString("{foo:bar}"))
	proxy.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, testPostResponse, w.Body.String())

	req, _ = http.NewRequest(http.MethodPost, "/", bytes.NewBufferString("{bar:foo}"))
	w = httptest.NewRecorder()
	proxy.ServeHTTP(w, req)

	//Should not cache POST requests
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, testPostResponse, w.Body.String())
	assert.False(t, isCachedResponse(w))
}

//Returns true if the response has X-Cached header set to 'true'
//and false if the value does not exists or has invalid data
func isCachedResponse(w *httptest.ResponseRecorder) bool {
	b, err := strconv.ParseBool(w.Header().Get("X-Cached"))
	if err != nil {
		return false
	}
	return b
}
