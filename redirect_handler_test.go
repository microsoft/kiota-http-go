package nethttplibrary

import (
	nethttp "net/http"
	httptest "net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	testing "testing"

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
	result, err := handler.getRedirectRequest(newNoopPipeline(), req, resp)
	if err != nil {
		t.Error(err)
	}
	assert.NotNil(t, result)
	assert.Equal(t, "www.bing.com", result.Host)
	assert.Equal(t, "", result.Header.Get("Authorization"))
}

// Cookie Header Tests

func TestItStripsCookieHeaderOnDifferentHost(t *testing.T) {
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
	req.Header.Set("Cookie", "session=abc123; user=john")

	client := getDefaultClientWithoutMiddleware()
	resp, err := client.Do(req)
	if err != nil {
		t.Error(err)
	}

	result, err := handler.getRedirectRequest(newNoopPipeline(), req, resp)
	if err != nil {
		t.Error(err)
	}

	assert.NotNil(t, result)
	assert.Equal(t, "www.bing.com", result.Host)
	assert.Equal(t, "", result.Header.Get("Cookie"))
}

func TestItKeepsCookieHeaderOnSameHost(t *testing.T) {
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.Header().Set("Location", "/newpath")
		res.WriteHeader(301)
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()

	handler := NewRedirectHandler()
	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	if err != nil {
		t.Error(err)
	}
	req.Header.Set("Cookie", "session=abc123")

	client := getDefaultClientWithoutMiddleware()
	resp, err := client.Do(req)
	if err != nil {
		t.Error(err)
	}

	result, err := handler.getRedirectRequest(newNoopPipeline(), req, resp)
	if err != nil {
		t.Error(err)
	}

	assert.NotNil(t, result)
	assert.NotEqual(t, "", result.Header.Get("Cookie"))
}

// Proxy-Authorization Tests

func TestItStripsProxyAuthorizationWhenNoProxy(t *testing.T) {
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
	req.Header.Set("Proxy-Authorization", "Basic dXNlcjpwYXNz")

	client := getDefaultClientWithoutMiddleware()
	resp, err := client.Do(req)
	if err != nil {
		t.Error(err)
	}

	result, err := handler.getRedirectRequest(newNoopPipeline(), req, resp)
	if err != nil {
		t.Error(err)
	}

	assert.NotNil(t, result)
	assert.Equal(t, "", result.Header.Get("Proxy-Authorization"))
}

func TestItKeepsProxyAuthorizationWhenProxyActive(t *testing.T) {
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.Header().Set("Location", "https://www.bing.com/")
		res.WriteHeader(301)
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()

	// Create a custom pipeline with proxy transport
	proxyURL, _ := url.Parse("http://proxy.example.com:8080")
	transport := &nethttp.Transport{
		Proxy: nethttp.ProxyURL(proxyURL),
	}
	pipeline := &middlewarePipeline{
		transport:   transport,
		middlewares: []Middleware{},
	}

	handler := NewRedirectHandler()
	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	if err != nil {
		t.Error(err)
	}
	req.Header.Set("Proxy-Authorization", "Basic dXNlcjpwYXNz")

	client := getDefaultClientWithoutMiddleware()
	resp, err := client.Do(req)
	if err != nil {
		t.Error(err)
	}

	result, err := handler.getRedirectRequest(pipeline, req, resp)
	if err != nil {
		t.Error(err)
	}

	assert.NotNil(t, result)
	assert.NotEqual(t, "", result.Header.Get("Proxy-Authorization"))
}

func TestItStripsProxyAuthorizationWhenProxyBypassed(t *testing.T) {
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.Header().Set("Location", "https://internal.local/")
		res.WriteHeader(301)
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()

	// Create transport with proxy that bypasses internal.local
	proxyURL, _ := url.Parse("http://proxy.example.com:8080")
	transport := &nethttp.Transport{
		Proxy: func(req *nethttp.Request) (*url.URL, error) {
			// Bypass proxy for internal.local
			if strings.Contains(req.URL.Host, "internal.local") {
				return nil, nil
			}
			return proxyURL, nil
		},
	}
	pipeline := &middlewarePipeline{
		transport:   transport,
		middlewares: []Middleware{},
	}

	handler := NewRedirectHandler()
	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	if err != nil {
		t.Error(err)
	}
	req.Header.Set("Proxy-Authorization", "Basic dXNlcjpwYXNz")

	client := getDefaultClientWithoutMiddleware()
	resp, err := client.Do(req)
	if err != nil {
		t.Error(err)
	}

	result, err := handler.getRedirectRequest(pipeline, req, resp)
	if err != nil {
		t.Error(err)
	}

	assert.NotNil(t, result)
	assert.Equal(t, "", result.Header.Get("Proxy-Authorization"))
}

// Custom Callback Tests

func TestItUsesCustomScrubSensitiveHeadersCallback(t *testing.T) {
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.Header().Set("Location", "https://www.bing.com/")
		res.WriteHeader(301)
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()

	callbackInvoked := false
	customCallback := func(req *nethttp.Request, orig *url.URL, new *url.URL, pr ProxyResolverFunc) {
		callbackInvoked = true
		// Custom logic: remove X-Api-Key on host change
		if !strings.EqualFold(new.Host, orig.Host) {
			req.Header.Del("X-Api-Key")
		}
	}

	handler := NewRedirectHandlerWithOptions(RedirectHandlerOptions{
		MaxRedirects:          defaultMaxRedirects,
		ScrubSensitiveHeaders: customCallback,
	})

	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	if err != nil {
		t.Error(err)
	}
	req.Header.Set("X-Api-Key", "secret123")
	req.Header.Set("X-Safe-Header", "safe-value")

	client := getDefaultClientWithoutMiddleware()
	resp, err := client.Do(req)
	if err != nil {
		t.Error(err)
	}

	result, err := handler.getRedirectRequest(newNoopPipeline(), req, resp)
	if err != nil {
		t.Error(err)
	}

	assert.True(t, callbackInvoked)
	assert.Equal(t, "", result.Header.Get("X-Api-Key"))
	assert.NotEqual(t, "", result.Header.Get("X-Safe-Header"))
}

func TestItUsesDefaultCallbackWhenNil(t *testing.T) {
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.Header().Set("Location", "https://www.bing.com/")
		res.WriteHeader(301)
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()

	// Don't set ScrubSensitiveHeaders - should default to DefaultScrubSensitiveHeaders
	handler := NewRedirectHandlerWithOptions(RedirectHandlerOptions{
		MaxRedirects: defaultMaxRedirects,
		// ScrubSensitiveHeaders is nil
	})

	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	if err != nil {
		t.Error(err)
	}
	req.Header.Set("Authorization", "Bearer token123")
	req.Header.Set("Cookie", "session=abc")

	client := getDefaultClientWithoutMiddleware()
	resp, err := client.Do(req)
	if err != nil {
		t.Error(err)
	}

	result, err := handler.getRedirectRequest(newNoopPipeline(), req, resp)
	if err != nil {
		t.Error(err)
	}

	// Default behavior should remove both
	assert.Equal(t, "", result.Header.Get("Authorization"))
	assert.Equal(t, "", result.Header.Get("Cookie"))
}

// Scheme Change Tests

func TestItStripsHeadersOnSchemeChange(t *testing.T) {
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		// Redirect from http to https (scheme change, same host)
		httpsURL := "https://" + req.Host + "/secure"
		res.Header().Set("Location", httpsURL)
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
	req.Header.Set("Cookie", "session=123")

	client := getDefaultClientWithoutMiddleware()
	resp, err := client.Do(req)
	if err != nil {
		t.Error(err)
	}

	result, err := handler.getRedirectRequest(newNoopPipeline(), req, resp)
	if err != nil {
		t.Error(err)
	}

	assert.NotNil(t, result)
	assert.Equal(t, "", result.Header.Get("Authorization"))
	assert.Equal(t, "", result.Header.Get("Cookie"))
}

// Port Change Tests

func TestItStripsHeadersOnPortChange(t *testing.T) {
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		// Redirect to different port
		res.Header().Set("Location", "http://example.com:8080/")
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
	req.Header.Set("Cookie", "session=123")

	client := getDefaultClientWithoutMiddleware()
	resp, err := client.Do(req)
	if err != nil {
		t.Error(err)
	}

	// Update the request URL to simulate coming from port 80
	req.URL.Host = "example.com:80"

	result, err := handler.getRedirectRequest(newNoopPipeline(), req, resp)
	if err != nil {
		t.Error(err)
	}

	assert.NotNil(t, result)
	assert.Equal(t, "example.com:8080", result.URL.Host)
	assert.Equal(t, "", result.Header.Get("Authorization"))
	assert.Equal(t, "", result.Header.Get("Cookie"))
}

func TestItKeepsHeadersOnSamePort(t *testing.T) {
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		// Redirect to same host, scheme, and port
		res.Header().Set("Location", "http://example.com:8080/newpath")
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
	req.Header.Set("Cookie", "session=123")

	client := getDefaultClientWithoutMiddleware()
	resp, err := client.Do(req)
	if err != nil {
		t.Error(err)
	}

	// Update the request URL to simulate coming from the same port
	req.URL.Host = "example.com:8080"

	result, err := handler.getRedirectRequest(newNoopPipeline(), req, resp)
	if err != nil {
		t.Error(err)
	}

	assert.NotNil(t, result)
	assert.Equal(t, "example.com:8080", result.URL.Host)
	assert.NotEqual(t, "", result.Header.Get("Authorization"))
	assert.NotEqual(t, "", result.Header.Get("Cookie"))
}
