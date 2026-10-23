package nethttplibrary

import (
	"context"
	"errors"
	"fmt"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	abs "github.com/microsoft/kiota-abstractions-go"
	assert "github.com/stretchr/testify/assert"
	require "github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type stubPipeline struct {
	response *nethttp.Response
	err      error
}

func (s *stubPipeline) Next(req *nethttp.Request, middlewareIndex int) (*nethttp.Response, error) {
	return s.response, s.err
}

type recordedEvent struct {
	Name       string
	Attributes []attribute.KeyValue
}

type recordingSpan struct {
	noop.Span
	name           string
	ended          bool
	recordedErrors []error
	events         []recordedEvent
	attributes     []attribute.KeyValue
}

func (s *recordingSpan) RecordError(err error, options ...trace.EventOption) {
	if err == nil {
		return
	}
	s.recordedErrors = append(s.recordedErrors, err)
	s.AddEvent("exception", trace.WithAttributes(
		attribute.String("exception.message", err.Error()),
		attribute.String("exception.type", fmt.Sprintf("%T", err)),
	))
}

func (s *recordingSpan) AddEvent(name string, options ...trace.EventOption) {
	cfg := trace.NewEventConfig(options...)
	s.events = append(s.events, recordedEvent{
		Name:       name,
		Attributes: cfg.Attributes(),
	})
}

func (s *recordingSpan) SetAttributes(attrs ...attribute.KeyValue) {
	s.attributes = append(s.attributes, attrs...)
}

func (s *recordingSpan) End(options ...trace.SpanEndOption) {
	s.ended = true
}

type recordingTracer struct {
	noop.Tracer
	spans []*recordingSpan
}

func (t *recordingTracer) Start(ctx context.Context, spanName string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	span := &recordingSpan{name: spanName}
	t.spans = append(t.spans, span)
	return trace.ContextWithSpan(ctx, span), span
}

type recordingTracerProvider struct {
	noop.TracerProvider
	tracers map[string]*recordingTracer
}

func newRecordingTracerProvider() *recordingTracerProvider {
	return &recordingTracerProvider{
		tracers: make(map[string]*recordingTracer),
	}
}

func (p *recordingTracerProvider) Tracer(name string, options ...trace.TracerOption) trace.Tracer {
	if tracer, ok := p.tracers[name]; ok {
		return tracer
	}
	tracer := &recordingTracer{}
	p.tracers[name] = tracer
	return tracer
}

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

func TestHeadersInspectionWithEmptyHeaderValuesDoesNotPanic(t *testing.T) {
	options := NewHeadersInspectionOptions()
	options.InspectRequestHeaders = true
	options.InspectResponseHeaders = true
	handler := NewHeadersInspectionHandlerWithOptions(*options)

	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com", nil)
	assert.NoError(t, err)
	req.Header["empty-req-header"] = []string{}
	req.Header.Add("valid-req-header", "valid")

	mockResp := &nethttp.Response{
		StatusCode: 200,
		Header: nethttp.Header{
			"empty-res-header": []string{},
			"valid-res-header": []string{"valid"},
		},
	}
	pipeline := &stubPipeline{response: mockResp}

	assert.NotPanics(t, func() {
		resp, err := handler.Intercept(pipeline, 0, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, mockResp, resp)
	})

	assert.Equal(t, "valid", options.GetRequestHeaders().Get("valid-req-header")[0])
	assert.Equal(t, "valid", options.GetResponseHeaders().Get("valid-res-header")[0])
	assert.Empty(t, options.GetRequestHeaders().Get("empty-req-header"))
	assert.Empty(t, options.GetResponseHeaders().Get("empty-res-header"))
}

func TestHeadersInspectionNilRequestDoesNotPanic(t *testing.T) {
	handler := NewHeadersInspectionHandler()
	assert.NotPanics(t, func() {
		resp, err := handler.Intercept(&stubPipeline{}, 0, nil)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestHeadersInspectionNilHeadersOptionsDoesNotPanic(t *testing.T) {
	options := &HeadersInspectionOptions{
		InspectRequestHeaders:  true,
		InspectResponseHeaders: true,
		RequestHeaders:         nil,
		ResponseHeaders:        nil,
	}
	handler := NewHeadersInspectionHandlerWithOptions(*options)
	mockResp := &nethttp.Response{
		StatusCode: 200,
		Header: nethttp.Header{
			"res-header": []string{"res-val"},
		},
	}
	pipeline := &stubPipeline{response: mockResp}

	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com", nil)
	assert.NoError(t, err)
	req.Header.Add("req-header", "req-val")

	assert.NotPanics(t, func() {
		resp, err := handler.Intercept(pipeline, 0, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})
}

func TestHeadersInspectionIsolatesHeadersBetweenRequests(t *testing.T) {
	options := NewHeadersInspectionOptions()
	options.InspectRequestHeaders = true
	options.InspectResponseHeaders = true
	handler := NewHeadersInspectionHandlerWithOptions(*options)

	// First request
	req1, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com/1", nil)
	assert.NoError(t, err)
	req1.Header.Add("x-req-one", "val1")

	resp1 := &nethttp.Response{
		StatusCode: 200,
		Header: nethttp.Header{
			"x-res-one": []string{"resval1"},
		},
	}
	pipeline1 := &stubPipeline{response: resp1}

	_, err = handler.Intercept(pipeline1, 0, req1)
	assert.NoError(t, err)
	assert.True(t, options.GetRequestHeaders().ContainsKey("x-req-one"))
	assert.True(t, options.GetResponseHeaders().ContainsKey("x-res-one"))

	// Second request: previous headers should be cleared, isolating requests
	req2, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com/2", nil)
	assert.NoError(t, err)
	req2.Header.Add("x-req-two", "val2")

	resp2 := &nethttp.Response{
		StatusCode: 200,
		Header: nethttp.Header{
			"x-res-two": []string{"resval2"},
		},
	}
	pipeline2 := &stubPipeline{response: resp2}

	_, err = handler.Intercept(pipeline2, 0, req2)
	assert.NoError(t, err)
	assert.True(t, options.GetRequestHeaders().ContainsKey("x-req-two"))
	assert.False(t, options.GetRequestHeaders().ContainsKey("x-req-one"))
	assert.True(t, options.GetResponseHeaders().ContainsKey("x-res-two"))
	assert.False(t, options.GetResponseHeaders().ContainsKey("x-res-one"))
}

func TestHeadersInspectionPipelineErrorRecordsError(t *testing.T) {
	tracerProvider := newRecordingTracerProvider()
	prevProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(tracerProvider)
	defer otel.SetTracerProvider(prevProvider)

	handler := NewHeadersInspectionHandler()
	expectedErr := errors.New("network failure")
	pipeline := &stubPipeline{err: expectedErr}

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
		if s.name == "HeadersInspectionHandler_Intercept" {
			interceptSpan = s
			break
		}
	}
	require.NotNil(t, interceptSpan, "HeadersInspectionHandler_Intercept span should be recorded")
	assert.True(t, interceptSpan.ended, "span should be ended")
	assert.Contains(t, interceptSpan.recordedErrors, expectedErr, "recorded errors should contain pipeline error")

	hasExceptionEvent := false
	for _, event := range interceptSpan.events {
		if event.Name == "exception" {
			hasExceptionEvent = true
			for _, attr := range event.Attributes {
				if attr.Key == "exception.message" {
					assert.Equal(t, "network failure", attr.Value.AsString())
				}
			}
			break
		}
	}
	assert.True(t, hasExceptionEvent, "completed HeadersInspectionHandler_Intercept span should contain an exception event for the pipeline error")
}

func TestHeadersInspectionPipelineErrorWithoutObservabilityOptions(t *testing.T) {
	handler := NewHeadersInspectionHandler()
	expectedErr := errors.New("network failure")
	pipeline := &stubPipeline{err: expectedErr}

	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com", nil)
	require.NoError(t, err)

	resp, err := handler.Intercept(pipeline, 0, req)
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, resp)
}
