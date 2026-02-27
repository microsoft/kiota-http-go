package nethttplibrary

import (
	nethttp "net/http"
	httptest "net/http/httptest"
	"net/url"
	testing "testing"

	"strconv"

	assert "github.com/stretchr/testify/assert"
)

func TestItCreatesANewRedirectHandler(t *testing.T) {
	handler := NewRedirectHandler()
	if handler == nil {
		t.Error("handler is nil")
	}
}

func TestItDoesntRedirectWithoutMiddleware(t *testing.T) {
	requestCount := int64(0)
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		requestCount++
		res.Header().Set("Location", "/"+strconv.FormatInt(requestCount, 10))
		res.WriteHeader(301)
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()
	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	if err != nil {
		t.Error(err)
	}
	client := getDefaultClientWithoutMiddleware()
	resp, err := client.Do(req)
	if err != nil {
		t.Error(err)
	}
	assert.NotNil(t, resp)
	assert.Equal(t, int64(1), requestCount)
}

func TestItHonoursShouldRedirect(t *testing.T) {
	requestCount := int64(0)
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		requestCount++
		res.Header().Set("Location", "/"+strconv.FormatInt(requestCount, 10))
		res.WriteHeader(301)
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()
	handler := NewRedirectHandlerWithOptions(RedirectHandlerOptions{
		ShouldRedirect: func(req *nethttp.Request, res *nethttp.Response) bool {
			return false
		},
	})
	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	if err != nil {
		t.Error(err)
	}
	resp, err := handler.Intercept(newNoopPipeline(), 0, req)
	if err != nil {
		t.Error(err)
	}
	assert.NotNil(t, resp)
	assert.Equal(t, int64(1), requestCount)
}

func TestItHonoursMaxRedirect(t *testing.T) {
	requestCount := int64(0)
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		requestCount++
		res.Header().Set("Location", "/"+strconv.FormatInt(requestCount, 10))
		res.WriteHeader(301)
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()
	handler := NewRedirectHandler()
	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	if err != nil {
		t.Error(err)
	}
	resp, err := handler.Intercept(newNoopPipeline(), 0, req)
	if err != nil {
		t.Error(err)
	}
	assert.NotNil(t, resp)
	assert.Equal(t, int64(defaultMaxRedirects+1), requestCount)
}

func TestItStripsAuthorizationHeaderOnDifferentHost(t *testing.T) {
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.Header().Set("Location", "https://www.bing.com/")
		res.WriteHeader(301)
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()
	handler := NewRedirectHandler()
	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	if err != nil {
		t.Error(err)
	}
	req.Header.Set("Authorization", "Bearer 12345")
	client := getDefaultClientWithoutMiddleware()
	resp, err := client.Do(req)
	if err != nil {
		t.Error(err)
	}
	result, err := handler.getRedirectRequest(req, resp)
	if err != nil {
		t.Error(err)
	}
	assert.NotNil(t, result)
	assert.Equal(t, "www.bing.com", result.Host)
	assert.Equal(t, "", result.Header.Get("Authorization"))
}

func TestItStripsSensitiveHeadersOnCrossHostRedirect(t *testing.T) {
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.Header().Set("Location", "https://other.example.com/api")
		res.WriteHeader(301)
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()

	handler := NewRedirectHandler()
	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	if err != nil {
		t.Error(err)
	}
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Cookie", "session=SECRET")

	client := getDefaultClientWithoutMiddleware()
	resp, err := client.Do(req)
	if err != nil {
		t.Error(err)
	}

	result, err := handler.getRedirectRequest(req, resp)
	if err != nil {
		t.Error(err)
	}

	assert.NotNil(t, result)
	assert.Equal(t, "", result.Header.Get("Authorization"))
	assert.Equal(t, "", result.Header.Get("Cookie"))
}

func TestItStripsSensitiveHeadersOnSchemeChange(t *testing.T) {
	handler := NewRedirectHandler()
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com/v1/api", nil)
	if err != nil {
		t.Error(err)
	}
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Cookie", "session=SECRET")

	resp := &nethttp.Response{
		StatusCode: 301,
		Header:     nethttp.Header{},
	}
	resp.Header.Set("Location", "http://example.com/v1/api")

	result, err := handler.getRedirectRequest(req, resp)
	if err != nil {
		t.Error(err)
	}

	assert.NotNil(t, result)
	assert.Equal(t, "", result.Header.Get("Authorization"))
	assert.Equal(t, "", result.Header.Get("Cookie"))
}

func TestItKeepsSensitiveHeadersOnSameHostAndScheme(t *testing.T) {
	handler := NewRedirectHandler()
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com/v1/api", nil)
	if err != nil {
		t.Error(err)
	}
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Cookie", "session=SECRET")
	req.Header.Set("Content-Type", "application/json")

	resp := &nethttp.Response{
		StatusCode: 301,
		Header:     nethttp.Header{},
	}
	resp.Header.Set("Location", "https://example.com/v2/api")

	result, err := handler.getRedirectRequest(req, resp)
	if err != nil {
		t.Error(err)
	}

	assert.NotNil(t, result)
	assert.Equal(t, "Bearer token", result.Header.Get("Authorization"))
	assert.Equal(t, "session=SECRET", result.Header.Get("Cookie"))
	assert.Equal(t, "application/json", result.Header.Get("Content-Type"))
}

func TestItUsesCustomScrubber(t *testing.T) {
	customScrubber := func(request *nethttp.Request, originalURL, newURL *url.URL) {
		// Custom logic: never remove headers
	}

	handler := NewRedirectHandlerWithOptions(RedirectHandlerOptions{
		MaxRedirects:          defaultMaxRedirects,
		ScrubSensitiveHeaders: customScrubber,
		ShouldRedirect: func(req *nethttp.Request, res *nethttp.Response) bool {
			return true
		},
	})

	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://graph.microsoft.com/v1.0/me", nil)
	if err != nil {
		t.Error(err)
	}
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Cookie", "session=SECRET")

	resp := &nethttp.Response{
		StatusCode: 301,
		Header:     nethttp.Header{},
	}
	resp.Header.Set("Location", "https://evil.attacker.com/steal")

	result, err := handler.getRedirectRequest(req, resp)
	if err != nil {
		t.Error(err)
	}

	assert.NotNil(t, result)
	// Headers should be kept because custom scrubber doesn't remove them
	assert.Equal(t, "Bearer token", result.Header.Get("Authorization"))
	assert.Equal(t, "session=SECRET", result.Header.Get("Cookie"))
}

func TestDefaultScrubberHandlesNilGracefully(t *testing.T) {
	// Should not panic with nil values
	assert.NotPanics(t, func() {
		DefaultScrubSensitiveHeaders(nil, nil, nil)
	})

	req, _ := nethttp.NewRequest(nethttp.MethodGet, "https://example.com", nil)
	assert.NotPanics(t, func() {
		DefaultScrubSensitiveHeaders(req, nil, nil)
	})
}

func TestItKeepsHeadersOnRelativeUrlRedirect(t *testing.T) {
	handler := NewRedirectHandler()
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com/v1/api", nil)
	if err != nil {
		t.Error(err)
	}
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Cookie", "session=SECRET")

	resp := &nethttp.Response{
		StatusCode: 307,
		Header:     nethttp.Header{},
	}
	resp.Header.Set("Location", "/v2/api")

	result, err := handler.getRedirectRequest(req, resp)
	if err != nil {
		t.Error(err)
	}

	assert.NotNil(t, result)
	assert.Equal(t, "Bearer token", result.Header.Get("Authorization"))
	assert.Equal(t, "session=SECRET", result.Header.Get("Cookie"))
	assert.Equal(t, "https://example.com/v2/api", result.URL.String())
}
