package nethttplibrary

import (
	"github.com/stretchr/testify/assert"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
)

func TestItCreatesANewChaosHandler(t *testing.T) {
	handler := NewChaosHandler()
	if handler == nil {
		t.Error("handler is nil")
	}
}

func TestItCreatesANewChaosHandlerWithInvalidOptions(t *testing.T) {
	_, err := NewChaosHandlerWithOptions(&ChaosHandlerOptions{
		ChaosPercentage: 101,
		ChaosStrategy:   Random,
	})
	if err == nil || err.Error() != "ChaosPercentage must be between 0 and 100" {
		t.Error("Expected initialization ")
	}
}

func TestItCreatesANewChaosHandlerWithOptions(t *testing.T) {
	options := &ChaosHandlerOptions{
		ChaosPercentage: 100,
		ChaosStrategy:   Random,
		StatusCode:      400,
	}
	handler, err := NewChaosHandlerWithOptions(options)
	if err != nil {
		t.Error(err)
	}
	if handler == nil {
		t.Error("handler is nil")
	}

	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.WriteHeader(200)
		_, err := res.Write([]byte("body"))
		if err != nil {
			t.Error(err)
		}
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
	assert.Equal(t, 400, resp.StatusCode)
}

func TestChaosHandlerNilRequest(t *testing.T) {
	handler := NewChaosHandler()
	assert.NotPanics(t, func() {
		resp, err := handler.Intercept(newNoopPipeline(), 0, nil)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestChaosHandlerUnmappedMethodsDoNotPanic(t *testing.T) {
	options := &ChaosHandlerOptions{
		ChaosPercentage: 100,
		ChaosStrategy:   Random,
	}
	handler, err := NewChaosHandlerWithOptions(options)
	assert.NoError(t, err)

	unmappedMethods := []string{
		nethttp.MethodHead,
		nethttp.MethodOptions,
		nethttp.MethodTrace,
		nethttp.MethodConnect,
		"QUERY",
		"get",
		"post",
	}

	for _, method := range unmappedMethods {
		req, err := nethttp.NewRequest(method, "http://localhost/test", nil)
		assert.NoError(t, err)
		assert.NotPanics(t, func() {
			resp, err := handler.Intercept(newNoopPipeline(), 0, req)
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			assert.True(t, resp.StatusCode >= 400)
		}, "method %s should not panic", method)
	}
}

func TestChaosHandlerStatusMapWithClientRequest(t *testing.T) {
	statusMap := map[string]map[string]int{
		"/users": {
			nethttp.MethodGet: 501,
		},
	}
	options := &ChaosHandlerOptions{
		ChaosPercentage: 100,
		ChaosStrategy:   Random,
		BaseUrl:         "https://graph.microsoft.com/v1.0",
		StatusMap:       statusMap,
	}
	handler, err := NewChaosHandlerWithOptions(options)
	assert.NoError(t, err)

	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://graph.microsoft.com/v1.0/users", nil)
	assert.NoError(t, err)
	assert.Empty(t, req.RequestURI)

	resp, err := handler.Intercept(newNoopPipeline(), 0, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 501, resp.StatusCode)
}

func TestChaosHandlerStatusMapPathFallback(t *testing.T) {
	statusMap := map[string]map[string]int{
		"/messages": {
			nethttp.MethodGet: 503,
		},
	}
	options := &ChaosHandlerOptions{
		ChaosPercentage: 100,
		ChaosStrategy:   Random,
		StatusMap:       statusMap,
	}
	handler, err := NewChaosHandlerWithOptions(options)
	assert.NoError(t, err)

	req, err := nethttp.NewRequest(nethttp.MethodGet, "http://localhost/messages", nil)
	assert.NoError(t, err)

	resp, err := handler.Intercept(newNoopPipeline(), 0, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 503, resp.StatusCode)
}

func TestChaosHandlerResponseBodyStatusFallback(t *testing.T) {
	options := &ChaosHandlerOptions{
		ChaosPercentage: 100,
		ChaosStrategy:   Manual,
		StatusCode:      418, // I'm a teapot (not in custom httpStatusCode map)
	}
	handler, err := NewChaosHandlerWithOptions(options)
	assert.NoError(t, err)

	req, err := nethttp.NewRequest(nethttp.MethodGet, "http://localhost/tea", nil)
	assert.NoError(t, err)

	resp, err := handler.Intercept(newNoopPipeline(), 0, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 418, resp.StatusCode)
	assert.Equal(t, "I'm a teapot", resp.Status)
}

func TestChaosHandlerStatusMapBaseUrlPrecedenceOverPath(t *testing.T) {
	statusMap := map[string]map[string]int{
		"/v1.0/users": {
			nethttp.MethodGet: 418,
		},
		"^/users": {
			nethttp.MethodGet: 501,
		},
	}
	options := &ChaosHandlerOptions{
		ChaosPercentage: 100,
		ChaosStrategy:   Random,
		BaseUrl:         "https://example.test/v1.0",
		StatusMap:       statusMap,
	}
	handler, err := NewChaosHandlerWithOptions(options)
	assert.NoError(t, err)

	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.test/v1.0/users", nil)
	assert.NoError(t, err)

	resp, err := handler.Intercept(newNoopPipeline(), 0, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 501, resp.StatusCode)
}

func TestChaosHandlerStatusMapRegexWithPathFallback(t *testing.T) {
	statusMap := map[string]map[string]int{
		"^/users/[0-9]+": {
			nethttp.MethodGet: 503,
		},
	}
	options := &ChaosHandlerOptions{
		ChaosPercentage: 100,
		ChaosStrategy:   Random,
		StatusMap:       statusMap,
	}
	handler, err := NewChaosHandlerWithOptions(options)
	assert.NoError(t, err)

	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.test/users/123", nil)
	assert.NoError(t, err)

	resp, err := handler.Intercept(newNoopPipeline(), 0, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 503, resp.StatusCode)
}

func TestChaosHandlerStatusMapRegexWithFullUrlPreserved(t *testing.T) {
	statusMap := map[string]map[string]int{
		`^https://example\.test/items/[0-9]+`: {
			nethttp.MethodGet: 502,
		},
	}
	options := &ChaosHandlerOptions{
		ChaosPercentage: 100,
		ChaosStrategy:   Random,
		StatusMap:       statusMap,
	}
	handler, err := NewChaosHandlerWithOptions(options)
	assert.NoError(t, err)

	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.test/items/456", nil)
	assert.NoError(t, err)

	resp, err := handler.Intercept(newNoopPipeline(), 0, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, 502, resp.StatusCode)
}

func TestChaosHandlerStatusMapExactPathWhenFullUrlLacksMethod(t *testing.T) {
	statusMap := map[string]map[string]int{
		"https://example.test/items/a+b": {
			nethttp.MethodPost: 201,
		},
		"/items/a+b": {
			nethttp.MethodGet: 418,
		},
	}
	options := &ChaosHandlerOptions{
		ChaosPercentage: 100,
		ChaosStrategy:   Random,
		StatusMap:       statusMap,
	}
	handler, err := NewChaosHandlerWithOptions(options)
	assert.NoError(t, err)

	reqGet, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.test/items/a+b", nil)
	assert.NoError(t, err)

	respGet, err := handler.Intercept(newNoopPipeline(), 0, reqGet)
	assert.NoError(t, err)
	assert.NotNil(t, respGet)
	assert.Equal(t, 418, respGet.StatusCode)

	reqPost, err := nethttp.NewRequest(nethttp.MethodPost, "https://example.test/items/a+b", nil)
	assert.NoError(t, err)

	respPost, err := handler.Intercept(newNoopPipeline(), 0, reqPost)
	assert.NoError(t, err)
	assert.NotNil(t, respPost)
	assert.Equal(t, 201, respPost.StatusCode)
}
