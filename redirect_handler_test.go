package nethttplibrary

import (
	"context"
	"io"
	nethttp "net/http"
	httptest "net/http/httptest"
	"net/url"
	"strings"
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
	customScrubber := func(request *nethttp.Request, originalURL *url.URL) {
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

func TestItStripsSensitiveHeadersOnPortChange(t *testing.T) {
	handler := NewRedirectHandler()
	req, err := nethttp.NewRequest(nethttp.MethodGet, "http://example.org:8080/foo", nil)
	if err != nil {
		t.Error(err)
	}
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Cookie", "session=SECRET")

	resp := &nethttp.Response{
		StatusCode: 301,
		Header:     nethttp.Header{},
	}
	resp.Header.Set("Location", "http://example.org:9090/bar")

	result, err := handler.getRedirectRequest(req, resp)
	if err != nil {
		t.Error(err)
	}

	assert.NotNil(t, result)
	assert.Equal(t, "example.org:9090", result.URL.Host)
	assert.Equal(t, "", result.Header.Get("Authorization"))
	assert.Equal(t, "", result.Header.Get("Cookie"))
}

func TestItKeepsSensitiveHeadersOnSamePort(t *testing.T) {
	handler := NewRedirectHandler()
	req, err := nethttp.NewRequest(nethttp.MethodGet, "http://example.org:8080/foo", nil)
	if err != nil {
		t.Error(err)
	}
	req.Header.Set("Authorization", "Bearer token")
	req.Header.Set("Cookie", "session=SECRET")
	req.Header.Set("Content-Type", "application/json")

	resp := &nethttp.Response{
		StatusCode: 302,
		Header:     nethttp.Header{},
	}
	resp.Header.Set("Location", "http://example.org:8080/bar")

	result, err := handler.getRedirectRequest(req, resp)
	if err != nil {
		t.Error(err)
	}

	assert.NotNil(t, result)
	assert.Equal(t, "example.org:8080", result.URL.Host)
	assert.Equal(t, "Bearer token", result.Header.Get("Authorization"))
	assert.Equal(t, "session=SECRET", result.Header.Get("Cookie"))
	assert.Equal(t, "application/json", result.Header.Get("Content-Type"))
}

func TestDefaultScrubberHandlesNilGracefully(t *testing.T) {
	// Should not panic with nil values
	assert.NotPanics(t, func() {
		DefaultScrubSensitiveHeaders(nil, nil)
	})

	req, _ := nethttp.NewRequest(nethttp.MethodGet, "https://example.com", nil)
	assert.NotPanics(t, func() {
		DefaultScrubSensitiveHeaders(req, nil)
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

func TestRedirectMethodAndBodySemantics(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		method         string
		expectedMethod string
		expectBody     bool
	}{
		{name: "301 POST becomes GET", statusCode: movedPermanently, method: nethttp.MethodPost, expectedMethod: nethttp.MethodGet},
		{name: "302 POST becomes GET", statusCode: found, method: nethttp.MethodPost, expectedMethod: nethttp.MethodGet},
		{name: "303 PUT becomes GET", statusCode: seeOther, method: nethttp.MethodPut, expectedMethod: nethttp.MethodGet},
		{name: "303 GET remains GET", statusCode: seeOther, method: nethttp.MethodGet, expectedMethod: nethttp.MethodGet},
		{name: "303 HEAD remains HEAD", statusCode: seeOther, method: nethttp.MethodHead, expectedMethod: nethttp.MethodHead},
		{name: "307 preserves POST and body", statusCode: temporaryRedirect, method: nethttp.MethodPost, expectedMethod: nethttp.MethodPost, expectBody: true},
		{name: "308 preserves POST and body", statusCode: permanentRedirect, method: nethttp.MethodPost, expectedMethod: nethttp.MethodPost, expectBody: true},
	}

	bodyHeaders := []string{
		"Content-Length",
		"Transfer-Encoding",
		"Content-Type",
		"Content-Encoding",
		"Content-Language",
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := NewRedirectHandler()
			req, err := nethttp.NewRequest(test.method, "https://example.com/source", strings.NewReader("request body"))
			assert.NoError(t, err)
			req.Header.Set("Content-Length", "12")
			req.Header.Set("Transfer-Encoding", "chunked")
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Content-Encoding", "gzip")
			req.Header.Set("Content-Language", "en-US")
			req.Header.Set("X-Custom", "preserved")
			req.TransferEncoding = []string{"chunked"}

			_, err = io.ReadAll(req.Body)
			assert.NoError(t, err)

			resp := &nethttp.Response{
				StatusCode: test.statusCode,
				Header:     nethttp.Header{"Location": []string{"https://example.com/target"}},
			}
			result, err := handler.getRedirectRequest(req, resp)
			assert.NoError(t, err)
			assert.Equal(t, test.expectedMethod, result.Method)
			assert.Equal(t, "preserved", result.Header.Get("X-Custom"))

			if test.expectBody {
				replayedBody, readErr := io.ReadAll(result.Body)
				assert.NoError(t, readErr)
				assert.Equal(t, "request body", string(replayedBody))
				assert.NotNil(t, result.GetBody)
				assert.Equal(t, req.ContentLength, result.ContentLength)
				assert.Equal(t, req.TransferEncoding, result.TransferEncoding)
				for _, header := range bodyHeaders {
					assert.Equal(t, req.Header.Get(header), result.Header.Get(header))
				}
			} else {
				assert.Nil(t, result.Body)
				assert.Nil(t, result.GetBody)
				assert.Zero(t, result.ContentLength)
				assert.Nil(t, result.TransferEncoding)
				assert.Nil(t, result.Trailer)
				for _, header := range bodyHeaders {
					assert.Empty(t, result.Header.Get(header))
				}
			}
		})
	}
}

func TestItDoesNotFollow307Or308WithUnreplayableBody(t *testing.T) {
	for _, statusCode := range []int{temporaryRedirect, permanentRedirect} {
		t.Run(strconv.Itoa(statusCode), func(t *testing.T) {
			requestCount := 0
			testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
				requestCount++
				res.Header().Set("Location", "/redirected")
				res.WriteHeader(statusCode)
			}))
			defer testServer.Close()

			req, err := nethttp.NewRequest(nethttp.MethodPost, testServer.URL, io.NopCloser(strings.NewReader("request body")))
			assert.NoError(t, err)
			assert.Nil(t, req.GetBody)

			resp, err := NewRedirectHandler().Intercept(newNoopPipeline(), 0, req)
			assert.NoError(t, err)
			assert.Equal(t, statusCode, resp.StatusCode)
			assert.Equal(t, 1, requestCount)
		})
	}
}

func TestRedirectDropsBodyHeadersAndSensitiveHeadersAcrossOrigins(t *testing.T) {
	handler := NewRedirectHandler()
	req, err := nethttp.NewRequest(nethttp.MethodPost, "https://example.com/source", strings.NewReader("request body"))
	assert.NoError(t, err)
	req.Header.Set("Authorization", "******")
	req.Header.Set("Cookie", "session=SECRET")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Content-Encoding", "gzip")

	resp := &nethttp.Response{
		StatusCode: found,
		Header:     nethttp.Header{"Location": []string{"https://other.example.com/target"}},
	}
	result, err := handler.getRedirectRequest(req, resp)
	assert.NoError(t, err)
	assert.Equal(t, nethttp.MethodGet, result.Method)
	assert.Nil(t, result.Body)
	assert.Empty(t, result.Header.Get("Content-Type"))
	assert.Empty(t, result.Header.Get("Content-Encoding"))
	assert.Empty(t, result.Header.Get("Authorization"))
	assert.Empty(t, result.Header.Get("Cookie"))
}

func TestRedirectResolvesRelativeReferences(t *testing.T) {
	handler := NewRedirectHandler()

	testCases := []struct {
		name        string
		baseURL     string
		location    string
		expectedURL string
	}{
		{
			name:        "root relative",
			baseURL:     "https://example.com/api/v1/users",
			location:    "/v2/items",
			expectedURL: "https://example.com/v2/items",
		},
		{
			name:        "relative path without leading slash",
			baseURL:     "https://example.com/api/v1/users",
			location:    "details",
			expectedURL: "https://example.com/api/v1/details",
		},
		{
			name:        "dot slash relative path",
			baseURL:     "https://example.com/api/v1/users",
			location:    "./profile",
			expectedURL: "https://example.com/api/v1/profile",
		},
		{
			name:        "parent relative path",
			baseURL:     "https://example.com/api/v1/users",
			location:    "../auth",
			expectedURL: "https://example.com/api/auth",
		},
		{
			name:        "query string relative",
			baseURL:     "https://example.com/api/v1/users",
			location:    "?page=2",
			expectedURL: "https://example.com/api/v1/users?page=2",
		},
		{
			name:        "absolute url unchanged",
			baseURL:     "https://example.com/api/v1/users",
			location:    "https://other.example.com/callback",
			expectedURL: "https://other.example.com/callback",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := nethttp.NewRequest(nethttp.MethodGet, tc.baseURL, nil)
			assert.NoError(t, err)

			resp := &nethttp.Response{
				StatusCode: found,
				Header:     nethttp.Header{"Location": []string{tc.location}},
			}

			result, err := handler.getRedirectRequest(req, resp)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedURL, result.URL.String())
			assert.Equal(t, result.URL.Host, result.Host)
		})
	}
}

func TestRedirectMissingLocationReturnsError(t *testing.T) {
	handler := NewRedirectHandler()
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com/test", nil)
	assert.NoError(t, err)

	resp := &nethttp.Response{
		StatusCode: found,
		Header:     nethttp.Header{},
	}
	result, err := handler.getRedirectRequest(req, resp)
	assert.Error(t, err)
	assert.Nil(t, result)
}

type trackingReadCloser struct {
	io.Reader
	closed bool
}

func (t *trackingReadCloser) Close() error {
	t.closed = true
	return nil
}

type testFuncPipeline struct {
	next func(req *nethttp.Request, middlewareIndex int) (*nethttp.Response, error)
}

func (p *testFuncPipeline) Next(req *nethttp.Request, middlewareIndex int) (*nethttp.Response, error) {
	return p.next(req, middlewareIndex)
}

func TestRedirectClosesIntermediateResponseBody(t *testing.T) {
	handler := NewRedirectHandler()
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com/start", nil)
	assert.NoError(t, err)

	body1 := &trackingReadCloser{Reader: strings.NewReader("redirected body"), closed: false}
	resp1 := &nethttp.Response{
		StatusCode: found,
		Header:     nethttp.Header{"Location": []string{"https://example.com/end"}},
		Body:       body1,
	}

	body2 := &trackingReadCloser{Reader: strings.NewReader("final body"), closed: false}
	resp2 := &nethttp.Response{
		StatusCode: 200,
		Header:     nethttp.Header{},
		Body:       body2,
	}

	callCount := 0
	pipeline := &testFuncPipeline{
		next: func(req *nethttp.Request, middlewareIndex int) (*nethttp.Response, error) {
			callCount++
			if callCount == 1 {
				return resp1, nil
			}
			return resp2, nil
		},
	}

	finalResp, err := handler.Intercept(pipeline, 0, req)
	assert.NoError(t, err)
	assert.Equal(t, 200, finalResp.StatusCode)
	assert.True(t, body1.closed, "intermediate redirect response body must be closed to prevent connection leaks")
	assert.False(t, body2.closed, "final response body must remain open for caller")
}

func TestItUsesRequestContextCustomScrubber(t *testing.T) {
	handler := NewRedirectHandler()
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com/source", nil)
	assert.NoError(t, err)
	req.Header.Set("Custom-Secret", "secret-value")

	customScrubberCalled := false
	options := &RedirectHandlerOptions{
		ScrubSensitiveHeaders: func(request *nethttp.Request, originalURL *url.URL) {
			customScrubberCalled = true
			request.Header.Del("Custom-Secret")
		},
	}
	ctx := context.WithValue(req.Context(), redirectKeyValue, options)
	req = req.WithContext(ctx)

	resp := &nethttp.Response{
		StatusCode: found,
		Header:     nethttp.Header{"Location": []string{"https://example.com/target"}},
	}
	result, err := handler.getRedirectRequest(req, resp)
	assert.NoError(t, err)
	assert.True(t, customScrubberCalled)
	assert.Empty(t, result.Header.Get("Custom-Secret"))
}
