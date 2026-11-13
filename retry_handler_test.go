package nethttplibrary

import (
	"context"
	"errors"
	"go.opentelemetry.io/otel"
	"io"
	nethttp "net/http"
	httptest "net/http/httptest"
	"strconv"
	"strings"
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

func TestRetryHandler_GetRetryDelay_FractionalSeconds(t *testing.T) {
	handler := NewRetryHandler()
	opt := &RetryHandlerOptions{DelaySeconds: 3, MaxRetries: 3}
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	resp15 := &nethttp.Response{
		StatusCode: 429,
		Header:     nethttp.Header{"Retry-After": []string{"1.5"}},
	}
	delay15 := handler.getRetryDelay(req, resp15, opt, 1)
	assert.Equal(t, 1500*time.Millisecond, delay15)

	resp025 := &nethttp.Response{
		StatusCode: 429,
		Header:     nethttp.Header{"Retry-After": []string{"0.25"}},
	}
	delay025 := handler.getRetryDelay(req, resp025, opt, 1)
	assert.Equal(t, 250*time.Millisecond, delay025)
}

func TestRetryHandler_GetRetryDelay_NegativeSecondsFallback(t *testing.T) {
	handler := NewRetryHandler()
	opt := &RetryHandlerOptions{DelaySeconds: 2, MaxRetries: 3}
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	respNeg := &nethttp.Response{
		StatusCode: 429,
		Header:     nethttp.Header{"Retry-After": []string{"-10"}},
	}
	delay := handler.getRetryDelay(req, respNeg, opt, 1)
	// Must fall back to exponential backoff and never return negative duration
	assert.GreaterOrEqual(t, delay, 2*time.Second)
	assert.LessOrEqual(t, delay, 3*time.Second+100*time.Millisecond)
}

func TestRetryHandler_GetRetryDelay_InvalidFloatFallback(t *testing.T) {
	handler := NewRetryHandler()
	opt := &RetryHandlerOptions{DelaySeconds: 1, MaxRetries: 3}
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	for _, invalid := range []string{"NaN", "+Inf", "-Inf"} {
		resp := &nethttp.Response{
			StatusCode: 429,
			Header:     nethttp.Header{"Retry-After": []string{invalid}},
		}
		delay := handler.getRetryDelay(req, resp, opt, 1)
		assert.GreaterOrEqual(t, delay, 1*time.Second, "delay for %s should be non-negative exponential fallback", invalid)
	}
}

func TestRetryHandler_GetRetryDelay_HttpDateRFC850AndANSIC(t *testing.T) {
	handler := NewRetryHandler()
	opt := &RetryHandlerOptions{DelaySeconds: 2, MaxRetries: 3}
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	futureTime := time.Now().Add(10 * time.Second)

	// RFC 850 format: "Sunday, 06-Nov-94 08:49:37 GMT"
	respRFC850 := &nethttp.Response{
		StatusCode: 429,
		Header:     nethttp.Header{"Retry-After": []string{futureTime.UTC().Format(time.RFC850)}},
	}
	delayRFC850 := handler.getRetryDelay(req, respRFC850, opt, 1)
	assert.Greater(t, delayRFC850, 7*time.Second)
	assert.LessOrEqual(t, delayRFC850, 11*time.Second)

	// ANSI C asctime format: "Sun Nov  6 08:49:37 1994"
	respANSIC := &nethttp.Response{
		StatusCode: 429,
		Header:     nethttp.Header{"Retry-After": []string{futureTime.UTC().Format(time.ANSIC)}},
	}
	delayANSIC := handler.getRetryDelay(req, respANSIC, opt, 1)
	assert.Greater(t, delayANSIC, 7*time.Second)
	assert.LessOrEqual(t, delayANSIC, 11*time.Second)
}

func TestRetryHandler_GetRetryDelay_PastHttpDateReturnsZero(t *testing.T) {
	handler := NewRetryHandler()
	opt := &RetryHandlerOptions{DelaySeconds: 2, MaxRetries: 3}
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	pastTime := time.Now().Add(-10 * time.Second)
	respPast := &nethttp.Response{
		StatusCode: 429,
		Header:     nethttp.Header{"Retry-After": []string{pastTime.UTC().Format(time.RFC1123)}},
	}
	delay := handler.getRetryDelay(req, respPast, opt, 1)
	// Per RFC 9110 Section 10.2.3, an HTTP-date in the past is treated as a delay of 0 seconds.
	assert.Equal(t, time.Duration(0), delay)
}

func TestRetryHandler_GetRetryDelay_CapsAtAbsoluteMaxDelay(t *testing.T) {
	handler := NewRetryHandler()
	opt := &RetryHandlerOptions{DelaySeconds: 2, MaxRetries: 3}
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	testCases := []string{
		"86400",
		"10000000000",
		"100000000000000",
		time.Now().Add(100000 * time.Hour).UTC().Format(time.RFC1123),
	}

	expectedMaxDelay := time.Duration(absoluteMaxDelaySeconds) * time.Second
	for _, retryAfterVal := range testCases {
		resp := &nethttp.Response{
			StatusCode: 429,
			Header:     nethttp.Header{"Retry-After": []string{retryAfterVal}},
		}
		delay := handler.getRetryDelay(req, resp, opt, 1)
		assert.Equal(t, expectedMaxDelay, delay, "failed for Retry-After value: %s", retryAfterVal)
	}
}

func TestRetryHandler_OptionsNilSafety(t *testing.T) {
	var nilOpt *RetryHandlerOptions
	assert.Equal(t, defaultDelaySeconds, nilOpt.GetDelaySeconds())
	assert.Equal(t, defaultMaxRetries, nilOpt.GetMaxRetries())
	shouldRetryFn := nilOpt.GetShouldRetry()
	require.NotNil(t, shouldRetryFn)
	assert.True(t, shouldRetryFn(0, 0, nil, nil))

	emptyOpt := &RetryHandlerOptions{}
	assert.Equal(t, defaultDelaySeconds, emptyOpt.GetDelaySeconds())
	assert.Equal(t, defaultMaxRetries, emptyOpt.GetMaxRetries())
	emptyShouldRetryFn := emptyOpt.GetShouldRetry()
	require.NotNil(t, emptyShouldRetryFn)
	assert.True(t, emptyShouldRetryFn(0, 0, nil, nil))

	optBounds := &RetryHandlerOptions{
		DelaySeconds: 9999,
		MaxRetries:   9999,
	}
	assert.Equal(t, absoluteMaxDelaySeconds, optBounds.GetDelaySeconds())
	assert.Equal(t, absoluteMaxRetries, optBounds.GetMaxRetries())
}

func TestItResendsTheQueryBodyOnRetry(t *testing.T) {
	const body = `{"filter":"name eq 'kiota'"}`
	receivedBodies := make([]string, 0, 2)
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		received, _ := io.ReadAll(req.Body)
		receivedBodies = append(receivedBodies, string(received))
		if req.Header.Get("Retry-Attempt") == "" {
			res.WriteHeader(429)
		} else {
			res.WriteHeader(200)
		}
	}))
	defer testServer.Close()
	handler := NewRetryHandler()
	req, err := nethttp.NewRequest("QUERY", testServer.URL, nil)
	require.NoError(t, err)
	req.Body = NopCloser(strings.NewReader(body))
	req.ContentLength = int64(len(body))

	resp, err := handler.Intercept(newNoopPipeline(), 0, req)

	require.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, []string{body, body}, receivedBodies)
}

func TestItDoesntRetryQueryWithUnbufferedBody(t *testing.T) {
	requestCount := 0
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		requestCount++
		res.WriteHeader(429)
	}))
	defer testServer.Close()
	handler := NewRetryHandler()
	req, err := nethttp.NewRequest("QUERY", testServer.URL, nil)
	require.NoError(t, err)
	req.Body = NopCloser(strings.NewReader("body"))
	req.ContentLength = -1

	resp, err := handler.Intercept(newNoopPipeline(), 0, req)

	require.NoError(t, err)
	assert.Equal(t, 429, resp.StatusCode)
	assert.Equal(t, 1, requestCount)
}

func TestRetryHandlerClosesResponseBodyOnRetry(t *testing.T) {
	handler := NewRetryHandler()
	bodyTracker := &trackingReadCloser{Reader: strings.NewReader("temporary error")}
	resp503 := &nethttp.Response{
		StatusCode: 503,
		Header:     nethttp.Header{retryAfterHeader: []string{"0"}},
		Body:       bodyTracker,
	}
	resp200 := &nethttp.Response{
		StatusCode: nethttp.StatusOK,
		Header:     nethttp.Header{},
	}
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com/data", nil)
	assert.NoError(t, err)

	callCount := 0
	pipeline := &dynamicPipeline{
		next: func(r *nethttp.Request, index int) (*nethttp.Response, error) {
			callCount++
			if callCount == 1 {
				return resp503, nil
			}
			return resp200, nil
		},
	}
	resp, err := handler.Intercept(pipeline, 0, req)
	assert.NoError(t, err)
	assert.Equal(t, nethttp.StatusOK, resp.StatusCode)
	assert.True(t, bodyTracker.closed, "retried response body should be closed")
}

