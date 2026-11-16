package nethttplibrary

import (
	"context"
	"errors"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
)

type SpyPipeline struct {
	client          *nethttp.Client
	receivedRequest *nethttp.Request
}

func (pipeline *SpyPipeline) Next(req *nethttp.Request, middlewareIndex int) (*nethttp.Response, error) {
	pipeline.receivedRequest = req
	return nil, errors.New("Spy executor only")
}
func newSpyPipeline() *SpyPipeline {
	return &SpyPipeline{
		client: getDefaultClientWithoutMiddleware(),
	}
}
func (pipeline *SpyPipeline) GetReceivedRequest() *nethttp.Request {
	return pipeline.receivedRequest
}

func TestURLReplacementHandler(t *testing.T) {

	handler := NewUrlReplaceHandler(true, map[string]string{"/users/me-token-to-replace": "/me"})
	if handler == nil {
		t.Error("handler is nil")
	}
	url := "https://msgraph.com/users/me-token-to-replace/contactFolders"
	req, err := nethttp.NewRequest(nethttp.MethodGet, url, nil)
	if err != nil {
		t.Error(err)
	}

	pipeline := newSpyPipeline()
	_, _ = handler.Intercept(pipeline, 0, req)

	assert.Equal(t, pipeline.GetReceivedRequest().URL.Path, "/me/contactFolders")
}

func TestNewUrlReplaceOptions(t *testing.T) {
	options := NewUrlReplaceOptions()
	assert.NotNil(t, options)
	assert.True(t, options.IsEnabled())
	assert.Nil(t, options.GetReplacementPairs())
	assert.Equal(t, options.GetKey(), urlReplaceOptionKey)
}

func TestNewUrlReplaceHandlerWithOptions(t *testing.T) {
	options := UrlReplaceOptions{
		Enabled:          true,
		ReplacementPairs: map[string]string{"foo": "bar"},
	}
	handler := NewUrlReplaceHandlerWithOptions(options)
	assert.NotNil(t, handler)
	assert.True(t, handler.options.IsEnabled())
	assert.Equal(t, handler.options.GetReplacementPairs(), map[string]string{"foo": "bar"})
}

func TestUrlReplaceHandlerNilRequest(t *testing.T) {
	handler := NewUrlReplaceHandler(true, map[string]string{"foo": "bar"})
	assert.NotPanics(t, func() {
		resp, err := handler.Intercept(&stubPipeline{}, 0, nil)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestUrlReplaceHandlerNilURL(t *testing.T) {
	handler := NewUrlReplaceHandler(true, map[string]string{"foo": "bar"})
	req := &nethttp.Request{}
	assert.NotPanics(t, func() {
		resp, err := handler.Intercept(&stubPipeline{}, 0, req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestUrlReplaceOptionsNilSafety(t *testing.T) {
	var nilOpts *UrlReplaceOptions
	assert.False(t, nilOpts.IsEnabled())
	assert.Nil(t, nilOpts.GetReplacementPairs())
}

func TestUrlReplaceHandlerTypedNilOptionInContext(t *testing.T) {
	handler := NewUrlReplaceHandler(true, map[string]string{"/users/token-to-replace": "/me"})
	req, err := nethttp.NewRequest("GET", "https://example.com/users/token-to-replace", nil)
	assert.NoError(t, err)

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

func TestUrlReplaceHandlerSynchronizesRawPath(t *testing.T) {
	handler := NewUrlReplaceHandler(true, map[string]string{
		"/users/token-to-replace": "/me",
	})
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com/users/token-to-replace/items", nil)
	assert.NoError(t, err)
	req.URL.RawPath = "/users/token%2Dto%2Dreplace/items"

	pipeline := newSpyPipeline()
	_, _ = handler.Intercept(pipeline, 0, req)

	assert.Equal(t, "/me/items", pipeline.GetReceivedRequest().URL.Path)
	assert.Equal(t, "/me/items", pipeline.GetReceivedRequest().URL.RawPath)
}

func TestUrlReplaceHandlerPreservesUntouchedEscapesAndWireURL(t *testing.T) {
	t.Run("does not alter percent escapes when token does not match Path", func(t *testing.T) {
		handler := NewUrlReplaceHandler(true, map[string]string{
			"2": "3",
		})
		req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com/items/a%2Fb", nil)
		require.NoError(t, err)
		assert.Equal(t, "/items/a/b", req.URL.Path)
		assert.Equal(t, "/items/a%2Fb", req.URL.RawPath)

		pipeline := newSpyPipeline()
		_, _ = handler.Intercept(pipeline, 0, req)

		received := pipeline.GetReceivedRequest()
		assert.Equal(t, "/items/a/b", received.URL.Path)
		assert.Equal(t, "/items/a%2Fb", received.URL.RawPath)
		assert.Equal(t, "/items/a%2Fb", received.URL.RequestURI())
		assert.Equal(t, "https://example.com/items/a%2Fb", received.URL.String())

		// Verify on the wire with an actual HTTP server
		serverReceivedURI := ""
		ts := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
			serverReceivedURI = r.RequestURI
			w.WriteHeader(nethttp.StatusOK)
		}))
		defer ts.Close()

		wireReq, err := nethttp.NewRequest(nethttp.MethodGet, ts.URL+"/items/a%2Fb", nil)
		require.NoError(t, err)
		_, _ = handler.Intercept(pipeline, 0, wireReq)

		resp, err := ts.Client().Do(pipeline.GetReceivedRequest())
		require.NoError(t, err)
		_ = resp.Body.Close()

		assert.Equal(t, "/items/a%2Fb", serverReceivedURI, "wire URL must preserve encoded slash instead of converting to path separator")
	})

	t.Run("updates only matched span and preserves other escaped segments", func(t *testing.T) {
		handler := NewUrlReplaceHandler(true, map[string]string{
			"/users/token-to-replace": "/me",
		})
		req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com/users/token-to-replace/items/a%2Fb", nil)
		require.NoError(t, err)
		assert.Equal(t, "/users/token-to-replace/items/a/b", req.URL.Path)
		assert.Equal(t, "/users/token-to-replace/items/a%2Fb", req.URL.RawPath)

		pipeline := newSpyPipeline()
		_, _ = handler.Intercept(pipeline, 0, req)

		received := pipeline.GetReceivedRequest()
		assert.Equal(t, "/me/items/a/b", received.URL.Path)
		assert.Equal(t, "/me/items/a%2Fb", received.URL.RawPath)
		assert.Equal(t, "/me/items/a%2Fb", received.URL.RequestURI())
		assert.Equal(t, "https://example.com/me/items/a%2Fb", received.URL.String())

		serverReceivedURI := ""
		ts := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
			serverReceivedURI = r.RequestURI
			w.WriteHeader(nethttp.StatusOK)
		}))
		defer ts.Close()

		wireReq, err := nethttp.NewRequest(nethttp.MethodGet, ts.URL+"/users/token-to-replace/items/a%2Fb", nil)
		require.NoError(t, err)
		_, _ = handler.Intercept(pipeline, 0, wireReq)

		resp, err := ts.Client().Do(pipeline.GetReceivedRequest())
		require.NoError(t, err)
		_ = resp.Body.Close()

		assert.Equal(t, "/me/items/a%2Fb", serverReceivedURI, "wire URL must apply replacement while preserving untouched encoded slash")
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

func TestUrlReplaceHandlerEmptyStringKeyPreservesSemanticsWithRawPath(t *testing.T) {
	handler := NewUrlReplaceHandler(true, map[string]string{
		"": "/v1",
	})

	t.Run("prepends replacement to unencoded path", func(t *testing.T) {
		req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com/items/a-b", nil)
		require.NoError(t, err)
		assert.Equal(t, "/items/a-b", req.URL.Path)
		assert.Equal(t, "", req.URL.RawPath)

		pipeline := newSpyPipeline()
		_, _ = handler.Intercept(pipeline, 0, req)

		received := pipeline.GetReceivedRequest()
		assert.Equal(t, "/v1/items/a-b", received.URL.Path)
		assert.Equal(t, "https://example.com/v1/items/a-b", received.URL.String())
	})

	t.Run("prepends replacement to escaped path preserving raw escapes", func(t *testing.T) {
		req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com/items/a%2Fb", nil)
		require.NoError(t, err)
		assert.Equal(t, "/items/a/b", req.URL.Path)
		assert.Equal(t, "/items/a%2Fb", req.URL.RawPath)

		pipeline := newSpyPipeline()
		_, _ = handler.Intercept(pipeline, 0, req)

		received := pipeline.GetReceivedRequest()
		assert.Equal(t, "/v1/items/a/b", received.URL.Path)
		assert.Equal(t, "/v1/items/a%2Fb", received.URL.RawPath)
		assert.Equal(t, "/v1/items/a%2Fb", received.URL.RequestURI())
		assert.Equal(t, "https://example.com/v1/items/a%2Fb", received.URL.String())

		// Verify on the wire with an actual HTTP server
		serverReceivedURI := ""
		ts := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
			serverReceivedURI = r.RequestURI
			w.WriteHeader(nethttp.StatusOK)
		}))
		defer ts.Close()

		wireReq, err := nethttp.NewRequest(nethttp.MethodGet, ts.URL+"/items/a%2Fb", nil)
		require.NoError(t, err)
		_, _ = handler.Intercept(pipeline, 0, wireReq)

		resp, err := ts.Client().Do(pipeline.GetReceivedRequest())
		require.NoError(t, err)
		_ = resp.Body.Close()

		assert.Equal(t, "/v1/items/a%2Fb", serverReceivedURI, "wire URL must prepend empty-key replacement while preserving encoded slash")
	})
}
