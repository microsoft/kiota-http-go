package nethttplibrary

import (
	"context"
	nethttp "net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
)

type SpyPipeline struct {
	receivedRequest *nethttp.Request
}

func newSpyPipeline() *SpyPipeline {
	return &SpyPipeline{}
}

func (pipeline *SpyPipeline) Next(req *nethttp.Request, middlewareIndex int) (*nethttp.Response, error) {
	pipeline.receivedRequest = req
	return nil, nil
}

func (pipeline *SpyPipeline) GetReceivedRequest() *nethttp.Request {
	return pipeline.receivedRequest
}

func TestNewUrlReplaceHandlerWithOptions(t *testing.T) {
	handler := NewUrlReplaceHandler(true, map[string]string{"foo": "bar"})
	assert.Equal(t, handler.options.GetReplacementPairs(), map[string]string{"foo": "bar"})
}

func TestUrlReplaceHandlerNilRequest(t *testing.T) {
	handler := NewUrlReplaceHandler(true, map[string]string{"foo": "bar"})
	assert.NotPanics(t, func() {
		resp, err := handler.Intercept(nil, 0, nil)
		assert.Nil(t, resp)
		assert.Equal(t, "req cannot be nil", err.Error())
	})
}

func TestUrlReplaceHandlerDisabled(t *testing.T) {
	handler := NewUrlReplaceHandler(false, map[string]string{"foo": "bar"})
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com/foo", nil)
	assert.NoError(t, err)

	pipeline := newSpyPipeline()
	_, _ = handler.Intercept(pipeline, 0, req)

	assert.Equal(t, "/foo", pipeline.GetReceivedRequest().URL.Path)
}

func TestUrlReplaceHandlerReplacesPath(t *testing.T) {
	handler := NewUrlReplaceHandler(true, map[string]string{"foo": "bar"})
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com/foo", nil)
	assert.NoError(t, err)

	pipeline := newSpyPipeline()
	_, _ = handler.Intercept(pipeline, 0, req)

	assert.Equal(t, "/bar", pipeline.GetReceivedRequest().URL.Path)
}

func TestUrlReplaceHandlerTypedNilOptionInContext(t *testing.T) {
	handler := NewUrlReplaceHandler(true, map[string]string{"/users/token-to-replace": "/me"})
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com/users/token-to-replace", nil)
	require.NoError(t, err)

	var nilOpt *UrlReplaceOptions
	ctx := context.WithValue(req.Context(), urlReplaceOptionKey, nilOpt)
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
		assert.Equal(t, nethttp.StatusOK, resp.StatusCode)
		assert.NotNil(t, pipeline.receivedRequest)
		assert.Equal(t, "/me", pipeline.receivedRequest.URL.Path)
	})
}

func TestUrlReplaceHandlerSpanRecordsURLWhenRequestURIEmpty(t *testing.T) {
	tracerProvider := newRecordingTracerProvider()
	prevProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(tracerProvider)
	defer otel.SetTracerProvider(prevProvider)

	handler := NewUrlReplaceHandler(true, map[string]string{"/users/token-to-replace": "/me"})

	t.Run("default does not record URL without EUII opt-in", func(t *testing.T) {
		req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com/users/token-to-replace", nil)
		require.NoError(t, err)

		obsOptions := &ObservabilityOptions{} // default: IncludeEUIIAttributes is false
		ctx := context.WithValue(req.Context(), observabilityOptionsKeyValue, obsOptions)
		req = req.WithContext(ctx)

		pipeline := &stubPipeline{
			response: &nethttp.Response{
				StatusCode: nethttp.StatusOK,
				Header:     nethttp.Header{},
			},
		}
		resp, err := handler.Intercept(pipeline, 0, req)
		require.NoError(t, err)
		require.NotNil(t, resp)

		tracer, ok := tracerProvider.tracers[obsOptions.GetTracerInstrumentationName()]
		require.True(t, ok)
		require.NotEmpty(t, tracer.spans)

		var replaceSpan *recordingSpan
		for _, s := range tracer.spans {
			if s.name == "UrlReplaceHandler_Intercept" {
				replaceSpan = s
			}
		}
		require.NotNil(t, replaceSpan)
		for _, attr := range replaceSpan.attributes {
			assert.NotEqual(t, "http.request_url", string(attr.Key), "http.request_url attribute must not be recorded when IncludeEUIIAttributes is false")
		}
	})

	t.Run("opt-in records URL when IncludeEUIIAttributes is true", func(t *testing.T) {
		req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com/users/token-to-replace", nil)
		require.NoError(t, err)

		obsOptions := &ObservabilityOptions{IncludeEUIIAttributes: true}
		ctx := context.WithValue(req.Context(), observabilityOptionsKeyValue, obsOptions)
		req = req.WithContext(ctx)

		pipeline := &stubPipeline{
			response: &nethttp.Response{
				StatusCode: nethttp.StatusOK,
				Header:     nethttp.Header{},
			},
		}
		resp, err := handler.Intercept(pipeline, 0, req)
		require.NoError(t, err)
		require.NotNil(t, resp)

		tracer, ok := tracerProvider.tracers[obsOptions.GetTracerInstrumentationName()]
		require.True(t, ok)
		require.NotEmpty(t, tracer.spans)

		var replaceSpan *recordingSpan
		for i := len(tracer.spans) - 1; i >= 0; i-- {
			if tracer.spans[i].name == "UrlReplaceHandler_Intercept" {
				replaceSpan = tracer.spans[i]
				break
			}
		}
		require.NotNil(t, replaceSpan)
		foundURL := false
		for _, attr := range replaceSpan.attributes {
			if attr.Key == "http.request_url" {
				foundURL = true
				assert.Equal(t, "https://example.com/me", attr.Value.AsString())
			}
		}
		assert.True(t, foundURL, "http.request_url attribute should be set with replaced URL when IncludeEUIIAttributes is true")
	})
}

func TestUrlReplaceHandlerTypedNilObservabilityOptions(t *testing.T) {
	tracerProvider := newRecordingTracerProvider()
	prevProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(tracerProvider)
	defer otel.SetTracerProvider(prevProvider)

	handler := NewUrlReplaceHandler(true, map[string]string{"/users/token-to-replace": "/me"})
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com/users/token-to-replace", nil)
	require.NoError(t, err)

	var nilObs *ObservabilityOptions
	ctx := context.WithValue(req.Context(), observabilityOptionsKeyValue, nilObs)
	req = req.WithContext(ctx)

	pipeline := &stubPipeline{
		response: &nethttp.Response{
			StatusCode: nethttp.StatusOK,
			Header:     nethttp.Header{},
		},
	}

	assert.NotPanics(t, func() {
		resp, err := handler.Intercept(pipeline, 0, req)
		require.NoError(t, err)
		require.NotNil(t, resp)
	})

	assert.Nil(t, GetObservabilityOptionsFromRequest(req))
	assert.False(t, nilObs.GetIncludeEUIIAttributes())
	assert.NotPanics(t, func() { nilObs.SetIncludeEUIIAttributes(true) })
}
