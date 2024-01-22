package nethttplibrary

import (
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	abs "github.com/microsoft/kiota-abstractions-go"
	assert "github.com/stretchr/testify/assert"
)

func TestItCreateANewHeadersInspectionHandler(t *testing.T) {
	handler := NewHeadersInspectionHandler()
	assert.NotNil(t, handler)
	_, ok := any(handler).(Middleware)
	assert.True(t, ok, "handler does not implement Middleware")
}
func TestHeadersInspectionOptionsImplementTheOptionInterface(t *testing.T) {
	options := NewHeadersInspectionOptions()
	assert.NotNil(t, options)
	_, ok := any(options).(abs.RequestOption)
	assert.True(t, ok, "options does not implement optionsType")
}

func TestItGetsRequestHeaders(t *testing.T) {
	options := NewHeadersInspectionOptions()
	options.InspectRequestHeaders = true
	assert.Empty(t, options.GetRequestHeaders().ListKeys())
	handler := NewHeadersInspectionHandlerWithOptions(*options)
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.Header().Add("test", "test")
		res.WriteHeader(200)
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()
	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	req.Header.Add("test", "test")
	if err != nil {
		t.Error(err)
	}
	_, err = handler.Intercept(newNoopPipeline(), 0, req)
	if err != nil {
		t.Error(err)
	}

	assert.NotEmpty(t, options.GetRequestHeaders().ListKeys())
	assert.Equal(t, "test", options.GetRequestHeaders().Get("test")[0])
	assert.Empty(t, options.GetResponseHeaders().ListKeys())
}

func TestItGetsResponseHeaders(t *testing.T) {
	options := NewHeadersInspectionOptions()
	options.InspectResponseHeaders = true
	assert.Empty(t, options.GetRequestHeaders().ListKeys())
	handler := NewHeadersInspectionHandlerWithOptions(*options)
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.Header().Add("test", "test")
		res.WriteHeader(200)
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()
	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	req.Header.Add("test", "test")
	if err != nil {
		t.Error(err)
	}
	_, err = handler.Intercept(newNoopPipeline(), 0, req)
	if err != nil {
		t.Error(err)
	}

	assert.NotEmpty(t, options.GetResponseHeaders().ListKeys())
	assert.Equal(t, "test", options.GetResponseHeaders().Get("test")[0])
	assert.Empty(t, options.GetRequestHeaders().ListKeys())
}
