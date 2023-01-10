package nethttplibrary

import (
	nethttp "net/http"
	httptest "net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestItAddsTheUserAgentHeader(t *testing.T) {
	handler := NewUserAgentHandler()
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.WriteHeader(200)
		res.Write([]byte("body"))
	}))
	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	if err != nil {
		t.Error(err)
	}
	resp, err := handler.Intercept(newNoopPipeline(), 0, req)
	if err != nil {
		t.Error(err)
	}
	assert.NotNil(t, resp)
	assert.Equal(t, "kiota-go", strings.Split(req.Header.Get("User-Agent"), "/")[0])
}

func TestItAddsTheUserAgentHeaderOnce(t *testing.T) {
	handler := NewUserAgentHandler()
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.WriteHeader(200)
		res.Write([]byte("body"))
	}))
	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	if err != nil {
		t.Error(err)
	}
	resp, err := handler.Intercept(newNoopPipeline(), 0, req)
	if err != nil {
		t.Error(err)
	}
	resp, err = handler.Intercept(newNoopPipeline(), 0, req)
	if err != nil {
		t.Error(err)
	}
	assert.NotNil(t, resp)
	assert.Equal(t, 1, len(strings.Split(req.Header.Get("User-Agent"), "kiota-go"))-1)
}

func TestItDoesNotAddTheUserAgentHeaderWhenDisabled(t *testing.T) {
	options := NewUserAgentHandlerOptions()
	options.Enabled = false
	handler := NewUserAgentHandlerWithOptions(options)
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.WriteHeader(200)
		res.Write([]byte("body"))
	}))
	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	if err != nil {
		t.Error(err)
	}
	resp, err := handler.Intercept(newNoopPipeline(), 0, req)
	if err != nil {
		t.Error(err)
	}
	resp, err = handler.Intercept(newNoopPipeline(), 0, req)
	if err != nil {
		t.Error(err)
	}
	assert.NotNil(t, resp)
	assert.Equal(t, false, strings.Contains(req.Header.Get("User-Agent"), "kiota-go"))
}
