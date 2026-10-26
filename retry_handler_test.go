package nethttplibrary

import (
	"context"
	"errors"
	"go.opentelemetry.io/otel"
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

func TestRetryHandlerNilRequest(t *testing.T) {
	handler := NewRetryHandler()
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

func TestRetryHandlerNilResponse(t *testing.T) {
	handler := NewRetryHandler()
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com", nil)
	require.NoError(t, err)
	pipeline := &stubPipeline{
		response: nil,
		err:      nil,
	}
	assert.NotPanics(t, func() {
		resp, err := handler.Intercept(pipeline, 0, req)
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, "response cannot be nil", err.Error())
	})
}

type sequencePipeline struct {
	responses []*nethttp.Response
	errors    []error
	callCount int
}

func (s *sequencePipeline) Next(req *nethttp.Request, middlewareIndex int) (*nethttp.Response, error) {
	idx := s.callCount
	s.callCount++
	if idx < len(s.responses) && idx < len(s.errors) {
		return s.responses[idx], s.errors[idx]
	}
	return nil, errors.New("unexpected call")
}

func TestRetryHandlerTerminalRetryErrorRecordedOnSpan(t *testing.T) {
	tracerProvider := newRecordingTracerProvider()
	prevProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(tracerProvider)
	defer otel.SetTracerProvider(prevProvider)

	handler := NewRetryHandlerWithOptions(RetryHandlerOptions{
		ShouldRetry: func(delay time.Duration, executionCount int, request *nethttp.Request, response *nethttp.Response) bool {
			return true
		},
		MaxRetries:   1,
		DelaySeconds: 0,
	})

	expectedErr := errors.New("downstream network reset")
	pipeline := &sequencePipeline{
		responses: []*nethttp.Response{
			{StatusCode: 429, Header: nethttp.Header{}},
			nil,
		},
		errors: []error{
			nil,
			expectedErr,
		},
	}

	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	obsOptions := &ObservabilityOptions{}
	ctx := context.WithValue(req.Context(), observabilityOptionsKeyValue, obsOptions)
	req = req.WithContext(ctx)

	resp, err := handler.Intercept(pipeline, 0, req)
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, resp)

	tracer, ok := tracerProvider.tracers[obsOptions.GetTracerInstrumentationName()]
	require.True(t, ok, "tracer should be created")
	require.NotEmpty(t, tracer.spans, "spans should be recorded")

	var interceptSpan *recordingSpan
	for _, s := range tracer.spans {
		if s.name == "RetryHandler_Intercept" {
			interceptSpan = s
			break
		}
	}
	require.NotNil(t, interceptSpan, "RetryHandler_Intercept span should be recorded")
	assert.True(t, interceptSpan.ended, "span should be ended")
	assert.Contains(t, interceptSpan.recordedErrors, expectedErr, "recorded errors should contain terminal retry error")
}
