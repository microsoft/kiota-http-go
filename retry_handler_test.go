package nethttplibrary

import (
	"context"
	nethttp "net/http"
	httptest "net/http/httptest"
	"strconv"
	testing "testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type NoopPipeline struct {
	client *nethttp.Client
}

func (pipeline *NoopPipeline) Next(req *nethttp.Request, middlewareIndex int) (*nethttp.Response, error) {
	return pipeline.client.Do(req)
}

func newNoopPipeline() *NoopPipeline {
	return &NoopPipeline{
		client: getDefaultClientWithoutMiddleware(),
	}
}

func TestItCreatesANewRetryHandler(t *testing.T) {
	handler := NewRetryHandler()
	if handler == nil {
		t.Error("handler is nil")
	}
}

func TestItAddsRetryAttemptHeaders(t *testing.T) {
	retryAttemptInt := 0
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		retryAttempt := req.Header.Get("Retry-Attempt")
		if retryAttempt == "" {
			res.WriteHeader(429)
		} else {
			res.WriteHeader(200)
			retryAttemptInt, _ = strconv.Atoi(retryAttempt)
		}
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()
	handler := NewRetryHandler()
	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	if err != nil {
		t.Error(err)
	}
	resp, err := handler.Intercept(newNoopPipeline(), 0, req)
	if err != nil {
		t.Error(err)
	}
	assert.NotNil(t, resp)
	assert.Equal(t, 1, retryAttemptInt)
}

func TestItHonoursShouldRetry(t *testing.T) {
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		retryAttempt := req.Header.Get("Retry-Attempt")
		if retryAttempt == "" {
			res.WriteHeader(429)
		} else {
			res.WriteHeader(200)
		}
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()
	handler := NewRetryHandlerWithOptions(RetryHandlerOptions{
		ShouldRetry: func(delay time.Duration, executionCount int, request *nethttp.Request, response *nethttp.Response) bool {
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
	assert.Equal(t, 429, resp.StatusCode)
}

func TestItHonoursMaxRetries(t *testing.T) {
	retryAttemptInt := -1
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.WriteHeader(429)
		retryAttemptInt++
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()
	handler := NewRetryHandler()
	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	if err != nil {
		t.Error(err)
	}
	resp, err := handler.Intercept(newNoopPipeline(), 0, req)
	if err != nil {
		t.Error(err)
	}
	assert.NotNil(t, resp)
	assert.Equal(t, 429, resp.StatusCode)
	assert.Equal(t, defaultMaxRetries, retryAttemptInt)
}

func TestWithoutRetryAfterHeaderItRetriesWithExponentialBackoff(t *testing.T) {
	retryAttemptInt := -1
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		retryAttemptInt++
		res.WriteHeader(429)
		_, _ = res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()

	handler := NewRetryHandlerWithOptions(RetryHandlerOptions{
		ShouldRetry: func(delay time.Duration, executionCount int, request *nethttp.Request, response *nethttp.Response) bool {
			return true
		},
		MaxRetries:   3,
		DelaySeconds: 1,
	})

	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	require.NoError(t, err)

	start := time.Now()
	_, err = handler.Intercept(newNoopPipeline(), 0, req)
	elapsed := time.Now().Sub(start)

	require.NoError(t, err)
	assert.Equal(t, 3, retryAttemptInt)
	assert.Greater(t, elapsed, time.Duration(1+2+4)*time.Second)
}

func TestItHonoursRetryAfterDate(t *testing.T) {
	retryAttemptInt := -1
	start := time.Now()
	retryAfterTimeStr := start.Add(4 * time.Second).Format(time.RFC1123)
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.Header().Set("Retry-After", retryAfterTimeStr)
		res.WriteHeader(429)
		retryAttemptInt++
		res.Write([]byte("body"))
	}))

	defer func() { testServer.Close() }()
	handler := NewRetryHandler()
	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	if err != nil {
		t.Error(err)
	}
	resp, err := handler.Intercept(newNoopPipeline(), 0, req)
	if err != nil {
		t.Error(err)
	}
	assert.NotNil(t, resp)
	end := time.Now()

	assert.Equal(t, defaultMaxRetries, retryAttemptInt)
	assert.Greater(t, end.Sub(start), 3*time.Second) // delay should be greater than 3 seconds (ignoring microsecond differences)
}

func TestItHonoursContextExpiry(t *testing.T) {
	retryAttemptInt := -1
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.Header().Set("Retry-After", "5")
		res.WriteHeader(429)
		retryAttemptInt++
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	handler := NewRetryHandler()
	req, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodGet, testServer.URL, nil)
	if err != nil {
		t.Error(err)
	}
	start := time.Now()
	resp, err := handler.Intercept(newNoopPipeline(), 0, req)
	end := time.Now()
	assert.Error(t, err)
	assert.Nil(t, resp)
	// Should not have retried because context expired.
	assert.Equal(t, 0, retryAttemptInt)
	assert.Less(t, end.Sub(start), 4*time.Second)
}

func TestItHonoursContextCancelled(t *testing.T) {
	retryAttemptInt := -1
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.Header().Set("Retry-After", "5")
		res.WriteHeader(429)
		retryAttemptInt++
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()
	ctx, cancel := context.WithCancel(context.Background())
	handler := NewRetryHandler()
	req, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodGet, testServer.URL, nil)
	if err != nil {
		t.Error(err)
	}
	go func() {
		time.Sleep(1 * time.Second)
		cancel()
	}()
	start := time.Now()
	resp, err := handler.Intercept(newNoopPipeline(), 0, req)
	end := time.Now()
	assert.Error(t, err)
	assert.Nil(t, resp)
	// Should not have retried because context expired.
	assert.Equal(t, 0, retryAttemptInt)
	assert.Less(t, end.Sub(start), 4*time.Second)
}

func TestItDoesntRetryOnSuccess(t *testing.T) {
	retryAttemptInt := -1
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.WriteHeader(200)
		retryAttemptInt++
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()
	handler := NewRetryHandler()
	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	if err != nil {
		t.Error(err)
	}
	resp, err := handler.Intercept(newNoopPipeline(), 0, req)
	if err != nil {
		t.Error(err)
	}
	assert.NotNil(t, resp)
	assert.Equal(t, 0, retryAttemptInt)
}
