package nethttplibrary

import (
	"context"
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

func TestUserAgentHandlerNilRequest(t *testing.T) {
	handler := NewUserAgentHandler()
	pipeline := &stubPipeline{
		response: &nethttp.Response{
			StatusCode: nethttp.StatusOK,
			Header:     nethttp.Header{},
		},
	}
	assert.NotPanics(t, func() {
		resp, err := handler.Intercept(pipeline, 0, nil)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestUserAgentHandlerNilHeader(t *testing.T) {
	handler := NewUserAgentHandler()
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com", nil)
	assert.NoError(t, err)
	req.Header = nil
	pipeline := &stubPipeline{
		response: &nethttp.Response{
			StatusCode: nethttp.StatusOK,
			Header:     nethttp.Header{},
		},
	}
	assert.NotPanics(t, func() {
		resp, err := handler.Intercept(pipeline, 0, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})
	assert.NotEmpty(t, req.Header.Get(userAgentHeaderKey))
}

func TestUserAgentHandlerWithoutVersion(t *testing.T) {
	options := &UserAgentHandlerOptions{Enabled: true, ProductName: "MyApp", ProductVersion: ""}
	handler := NewUserAgentHandlerWithOptions(options)
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com", nil)
	assert.NoError(t, err)

	pipeline := &stubPipeline{
		response: &nethttp.Response{
			StatusCode: nethttp.StatusOK,
			Header:     nethttp.Header{},
		},
	}
	resp, err := handler.Intercept(pipeline, 0, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "MyApp", req.Header.Get(userAgentHeaderKey))
}

func TestUserAgentHandlerEmptyProductAndVersion(t *testing.T) {
	options := &UserAgentHandlerOptions{Enabled: true, ProductName: "", ProductVersion: ""}
	handler := NewUserAgentHandlerWithOptions(options)

	// Clean request: should not set empty token or trailing slash
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com", nil)
	assert.NoError(t, err)
	pipeline := &stubPipeline{
		response: &nethttp.Response{
			StatusCode: nethttp.StatusOK,
			Header:     nethttp.Header{},
		},
	}
	resp, err := handler.Intercept(pipeline, 0, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Empty(t, req.Header.Get(userAgentHeaderKey))

	// Pre-existing header: should not append " /"
	req2, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com", nil)
	assert.NoError(t, err)
	req2.Header.Set(userAgentHeaderKey, "ExistingApp/1.0")
	pipeline2 := &stubPipeline{
		response: &nethttp.Response{
			StatusCode: nethttp.StatusOK,
			Header:     nethttp.Header{},
		},
	}
	resp2, err := handler.Intercept(pipeline2, 0, req2)
	assert.NoError(t, err)
	assert.NotNil(t, resp2)
	assert.Equal(t, "ExistingApp/1.0", req2.Header.Get(userAgentHeaderKey))
}

func TestUserAgentHandlerVersionlessWithPreExistingHeader(t *testing.T) {
	options := &UserAgentHandlerOptions{Enabled: true, ProductName: "MyApp", ProductVersion: ""}
	handler := NewUserAgentHandlerWithOptions(options)
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com", nil)
	assert.NoError(t, err)
	req.Header.Set(userAgentHeaderKey, "MyAppHelper/1.0")

	pipeline := &stubPipeline{
		response: &nethttp.Response{
			StatusCode: nethttp.StatusOK,
			Header:     nethttp.Header{},
		},
	}
	resp, err := handler.Intercept(pipeline, 0, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "MyAppHelper/1.0 MyApp", req.Header.Get(userAgentHeaderKey))

	// Intercepting again should not duplicate "MyApp"
	resp, err = handler.Intercept(pipeline, 0, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "MyAppHelper/1.0 MyApp", req.Header.Get(userAgentHeaderKey))
}

func TestUserAgentHandlerNilOption(t *testing.T) {
	handler := NewUserAgentHandler()
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com", nil)
	assert.NoError(t, err)
	var nilOpt *UserAgentHandlerOptions
	ctx := context.WithValue(req.Context(), userAgentKeyValue, nilOpt)
	req = req.WithContext(ctx)

	mockResp := &nethttp.Response{
		StatusCode: nethttp.StatusOK,
		Header:     nethttp.Header{},
	}
	pipeline := &stubPipeline{response: mockResp}

	assert.NotPanics(t, func() {
		resp, err := handler.Intercept(pipeline, 0, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})
	assert.Contains(t, req.Header.Get(userAgentHeaderKey), "kiota-go")
}

func TestUserAgentHandlerTypedNilOptionInContext(t *testing.T) {
	handler := NewUserAgentHandler()
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com", nil)
	assert.NoError(t, err)

	ctx := context.WithValue(req.Context(), userAgentKeyValue, (*UserAgentHandlerOptions)(nil))
	req = req.WithContext(ctx)

	pipeline := &stubPipeline{
		response: &nethttp.Response{
			StatusCode: nethttp.StatusOK,
			Header:     nethttp.Header{},
		},
	}
	assert.NotPanics(t, func() {
		resp, err := handler.Intercept(pipeline, 0, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})
	assert.Contains(t, req.Header.Get(userAgentHeaderKey), "kiota-go")
}

func TestUserAgentHandlerOptionsNilSafety(t *testing.T) {
	var nilOptions *UserAgentHandlerOptions
	assert.NotPanics(t, func() {
		assert.False(t, nilOptions.GetEnabled())
		assert.Equal(t, "", nilOptions.GetProductName())
		assert.Equal(t, "", nilOptions.GetProductVersion())
	})
}
