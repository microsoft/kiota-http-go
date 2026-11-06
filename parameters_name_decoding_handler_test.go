package nethttplibrary

import (
	"context"
	"errors"
	nethttp "net/http"
	httptest "net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
)

func TestItDecodesQueryParameterNames(t *testing.T) {
	testData := [][]string{
		{"?%24select=diplayName&api%2Dversion=2", "/?$select=diplayName&api-version=2"},
		{"?%24select=diplayName&api%7Eversion=2", "/?$select=diplayName&api~version=2"},
		{"?%24select=diplayName&api%2Eversion=2", "/?$select=diplayName&api.version=2"},
		{"/api-version/?%24select=diplayName&api%2Eversion=2", "/api-version/?$select=diplayName&api.version=2"},
		{"", "/"},
		{"?q=1%2B2", "/?q=1%2B2"},                       //Values are not decoded
		{"?q=M%26A", "/?q=M%26A"},                       //Values are not decoded
		{"?q%2D1=M%26A", "/?q-1=M%26A"},                 //Values are not decoded but params are
		{"?q%2D1&q=M%26A=M%26A", "/?q-1&q=M%26A=M%26A"}, //Values are not decoded but params are
		{"?%24select=diplayName&api%2Dversion=1%2B2", "/?$select=diplayName&api-version=1%2B2"}, //Values are not decoded but params are
		{"?%24select=diplayName&api%2Dversion=M%26A", "/?$select=diplayName&api-version=M%26A"}, //Values are not decoded but params are
		{"?%24select=diplayName&api%7Eversion=M%26A", "/?$select=diplayName&api~version=M%26A"}, //Values are not decoded but params are
		{"?%24select=diplayName&api%2Eversion=M%26A", "/?$select=diplayName&api.version=M%26A"}, //Values are not decoded but params are
		{"?%24select=diplayName&api%2Eversion=M%26A", "/?$select=diplayName&api.version=M%26A"}, //Values are not decoded but params are
	}
	result := ""
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		result = req.URL.String()
		res.WriteHeader(200)
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()
	for _, data := range testData {
		handler := NewParametersNameDecodingHandler()
		input := testServer.URL + data[0]
		expected := data[1]
		req, err := nethttp.NewRequest(nethttp.MethodGet, input, nil)
		if err != nil {
			t.Error(err)
		}
		resp, err := handler.Intercept(newNoopPipeline(), 0, req)
		if err != nil {
			t.Error(err)
		}
		assert.NotNil(t, resp)
		assert.Equal(t, expected, result)
	}
}
func TestItPreservesEncodedCharactersInQueryParameterValues(t *testing.T) {
	testData := [][]string{
		{"?query=%24select&%24select=displayName", "/?query=%24select&$select=displayName"},
		{"?filter=name%20eq%20%27%24test%27&api%2Dversion=2", "/?filter=name%20eq%20%27%24test%27&api-version=2"},
		{"?flag&%24top=5", "/?flag&$top=5"},
		{"?empty=&%24count=true", "/?empty=&$count=true"},
	}
	result := ""
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		result = req.URL.String()
		res.WriteHeader(200)
		res.Write([]byte("body"))
	}))
	defer testServer.Close()
	for _, data := range testData {
		handler := NewParametersNameDecodingHandler()
		input := testServer.URL + data[0]
		expected := data[1]
		req, err := nethttp.NewRequest(nethttp.MethodGet, input, nil)
		assert.NoError(t, err)
		resp, err := handler.Intercept(newNoopPipeline(), 0, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, expected, result)
	}
}

func TestParametersNameDecodingHandlerNilRequest(t *testing.T) {
	handler := NewParametersNameDecodingHandler()
	assert.NotPanics(t, func() {
		resp, err := handler.Intercept(&stubPipeline{}, 0, nil)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestParametersNameDecodingOptionsNilSafety(t *testing.T) {
	var nilOpts *ParametersNameDecodingOptions
	assert.False(t, nilOpts.GetEnable())
	assert.Nil(t, nilOpts.GetParametersToDecode())
}

func TestParametersNameDecodingHandlerTypedNilOptionInContext(t *testing.T) {
	handler := NewParametersNameDecodingHandler()
	req, err := nethttp.NewRequest("GET", "https://example.com/test?%24top=10", nil)
	assert.NoError(t, err)

	var nilOpt *ParametersNameDecodingOptions
	ctx := context.WithValue(req.Context(), parametersNameDecodingKeyValue, nilOpt)
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
		assert.Equal(t, "https://example.com/test?$top=10", pipeline.receivedRequest.URL.String())
	})
}

func TestParametersNameDecodingHandlerDownstreamPipelineErrorBubblesUpWithoutDuplicateRecording(t *testing.T) {
	tracerProvider := newRecordingTracerProvider()
	prevProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(tracerProvider)
	defer otel.SetTracerProvider(prevProvider)

	handler := NewParametersNameDecodingHandler()
	expectedErr := errors.New("downstream network error")
	pipeline := &stubPipeline{err: expectedErr}

	req, err := nethttp.NewRequest("GET", "https://example.com/test", nil)
	require.NoError(t, err)

	obsOptions := &ObservabilityOptions{}
	ctx := context.WithValue(req.Context(), observabilityOptionsKeyValue, obsOptions)
	req = req.WithContext(ctx)

	resp, err := handler.Intercept(pipeline, 0, req)
	assert.ErrorIs(t, err, expectedErr)
	assert.Nil(t, resp)

	tracer, ok := tracerProvider.tracers[obsOptions.GetTracerInstrumentationName()]
	require.True(t, ok)
	require.NotEmpty(t, tracer.spans)

	interceptSpan := tracer.spans[0]
	assert.Equal(t, "ParametersNameDecodingHandler_Intercept", interceptSpan.name)
	assert.True(t, interceptSpan.ended)
	assert.Empty(t, interceptSpan.recordedErrors, "downstream pipeline errors should bubble up without duplicate recording on handler span")
}
