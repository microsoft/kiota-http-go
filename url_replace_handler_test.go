package nethttplibrary

import (
	"context"
	"errors"
	nethttp "net/http"
	"testing"

	"github.com/stretchr/testify/assert"
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
		"/users/token-to-replace":     "/me",
		"/users/token%2Dto%2Dreplace": "/me",
	})
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com/users/token-to-replace/items", nil)
	assert.NoError(t, err)
	req.URL.RawPath = "/users/token%2Dto%2Dreplace/items"

	pipeline := newSpyPipeline()
	_, _ = handler.Intercept(pipeline, 0, req)

	assert.Equal(t, "/me/items", pipeline.GetReceivedRequest().URL.Path)
	assert.Equal(t, "/me/items", pipeline.GetReceivedRequest().URL.RawPath)
}

func TestUrlReplaceHandlerSpanRecordsURLWhenRequestURIEmpty(t *testing.T) {
	tracerProvider := newRecordingTracerProvider()
	otel.SetTracerProvider(tracerProvider)

	handler := NewUrlReplaceHandler(true, map[string]string{"/users/token-to-replace": "/me"})
	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com/users/token-to-replace", nil)
	assert.NoError(t, err)

	obsOptions := &ObservabilityOptions{}
	ctx := context.WithValue(req.Context(), observabilityOptionsKeyValue, obsOptions)
	req = req.WithContext(ctx)

	pipeline := &stubPipeline{
		response: &nethttp.Response{
			StatusCode: nethttp.StatusOK,
			Header:     nethttp.Header{},
		},
	}
	resp, err := handler.Intercept(pipeline, 0, req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	tracer, ok := tracerProvider.tracers[obsOptions.GetTracerInstrumentationName()]
	assert.True(t, ok)
	assert.NotEmpty(t, tracer.spans)

	var replaceSpan *recordingSpan
	for _, s := range tracer.spans {
		if s.name == "UrlReplaceHandler_Intercept" {
			replaceSpan = s
			break
		}
	}
	assert.NotNil(t, replaceSpan)
	foundURL := false
	for _, attr := range replaceSpan.attributes {
		if attr.Key == "http.request_url" {
			foundURL = true
			assert.Equal(t, "https://example.com/me", attr.Value.AsString())
		}
	}
	assert.True(t, foundURL, "http.request_url attribute should be set with replaced URL")
}
