package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

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
