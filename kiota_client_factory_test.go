package nethttplibrary

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	abstractions "github.com/microsoft/kiota-abstractions-go"
	"github.com/stretchr/testify/assert"
)

func TestGetDefaultMiddleWareWithMultipleOptions(t *testing.T) {
	retryOptions := RetryHandlerOptions{
		ShouldRetry: func(delay time.Duration, executionCount int, request *nethttp.Request, response *nethttp.Response) bool {
			return false
		},
	}
	redirectHandlerOptions := RedirectHandlerOptions{
		MaxRedirects: defaultMaxRedirects,
		ShouldRedirect: func(req *nethttp.Request, res *nethttp.Response) bool {
			return true
		},
	}
	compressionOptions := NewCompressionOptionsReference(false)
	parametersNameDecodingOptions := ParametersNameDecodingOptions{
		Enable:             true,
		ParametersToDecode: []byte{'-', '.', '~', '$'},
	}
	userAgentHandlerOptions := UserAgentHandlerOptions{
		Enabled:        true,
		ProductName:    "kiota-go",
		ProductVersion: "1.1.0",
	}
	headersInspectionOptions := HeadersInspectionOptions{
		RequestHeaders:  abstractions.NewRequestHeaders(),
		ResponseHeaders: abstractions.NewResponseHeaders(),
	}
	urlReplaceOptions := UrlReplaceOptions{
		Enabled:          true,
		ReplacementPairs: map[string]string{"/users/me-token-to-replace": "/me"},
	}
	options, err := GetDefaultMiddlewaresWithOptions(&retryOptions,
		&redirectHandlerOptions,
		compressionOptions,
		&parametersNameDecodingOptions,
		&userAgentHandlerOptions,
		&headersInspectionOptions,
		&urlReplaceOptions,
	)
	if err != nil {
		t.Error(err)
	}
	if len(options) != 7 {
		t.Errorf("expected 7 middleware, got %v", len(options))
	}

	for _, element := range options {
		switch v := element.(type) {
		case *CompressionHandler:
			assert.Equal(t, v.options.ShouldCompress(), compressionOptions.ShouldCompress())
		case *UrlReplaceHandler:
			assert.Equal(t, v.options.IsEnabled(), urlReplaceOptions.IsEnabled())
			assert.Equal(t, v.options.GetReplacementPairs(), urlReplaceOptions.GetReplacementPairs())
		}
	}
}

func TestGetDefaultMiddleWareWithInvalidOption(t *testing.T) {
	chaosOptions := ChaosHandlerOptions{
		ChaosPercentage: 101,
		ChaosStrategy:   Random,
	}
	_, err := GetDefaultMiddlewaresWithOptions(&chaosOptions)

	assert.Equal(t, err.Error(), "unsupported option type")
}

func TestGetDefaultMiddleWareWithOptions(t *testing.T) {
	compression := NewCompressionOptionsReference(false)
	options, err := GetDefaultMiddlewaresWithOptions(compression)
	verifyMiddlewareWithDisabledCompression(t, options, err)
}

func TestGetDefaultMiddleWareWithOptionsDeprecated(t *testing.T) {
	compression := NewCompressionOptions(false)
	options, err := GetDefaultMiddlewaresWithOptions(compression)
	verifyMiddlewareWithDisabledCompression(t, options, err)
}

func verifyMiddlewareWithDisabledCompression(t *testing.T, options []Middleware, err error) {
	if err != nil {
		t.Error(err)
	}
	if len(options) != 7 {
		t.Errorf("expected 7 middleware, got %v", len(options))
	}
	for _, element := range options {
		switch v := element.(type) {
		case *CompressionHandler:
			assert.Equal(t, v.options.ShouldCompress(), false)
		}
	}
}

func TestGetDefaultMiddlewares(t *testing.T) {
	options := GetDefaultMiddlewares()
	if len(options) != 7 {
		t.Errorf("expected 7 middleware, got %v", len(options))
	}

	for _, element := range options {
		switch v := element.(type) {
		case *CompressionHandler:
			assert.True(t, v.options.ShouldCompress())
		case *UrlReplaceHandler:
			assert.True(t, v.options.IsEnabled())
			assert.Nil(t, v.options.GetReplacementPairs())
		}
	}
}

func TestGetDefaultMiddlewaresDeterministicOrder(t *testing.T) {
	for i := 0; i < 20; i++ {
		middlewares := GetDefaultMiddlewares()
		assert.Equal(t, 7, len(middlewares))

		_, ok0 := middlewares[0].(*RetryHandler)
		assert.True(t, ok0, "expected RetryHandler at index 0")

		_, ok1 := middlewares[1].(*RedirectHandler)
		assert.True(t, ok1, "expected RedirectHandler at index 1")

		_, ok2 := middlewares[2].(*ParametersNameDecodingHandler)
		assert.True(t, ok2, "expected ParametersNameDecodingHandler at index 2")

		_, ok3 := middlewares[3].(*UrlReplaceHandler)
		assert.True(t, ok3, "expected UrlReplaceHandler at index 3")

		_, ok4 := middlewares[4].(*UserAgentHandler)
		assert.True(t, ok4, "expected UserAgentHandler at index 4")

		_, ok5 := middlewares[5].(*HeadersInspectionHandler)
		assert.True(t, ok5, "expected HeadersInspectionHandler at index 5")

		_, ok6 := middlewares[6].(*CompressionHandler)
		assert.True(t, ok6, "expected CompressionHandler at index 6")
	}
}

func TestDefaultPipelinePostRetryWithObservability(t *testing.T) {
	postPayload := []byte("default pipeline post request body with retry and observability")
	attempts := 0

	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		attempts++
		assert.Equal(t, nethttp.MethodPost, req.Method)
		assert.Equal(t, "gzip", req.Header.Get("Content-Encoding"), "expected Content-Encoding: gzip on attempt %d", attempts)

		// Decompress body and verify it matches the original postPayload on all attempts
		gzipReader, err := gzip.NewReader(req.Body)
		assert.NoError(t, err, "attempt %d body should be valid gzip", attempts)
		if err == nil {
			decompressed, readErr := io.ReadAll(gzipReader)
			assert.NoError(t, readErr)
			_ = gzipReader.Close()
			assert.Equal(t, postPayload, decompressed, "attempt %d payload mismatch", attempts)
		}

		if attempts == 1 {
			res.Header().Set("Retry-After", "0")
			res.WriteHeader(nethttp.StatusTooManyRequests)
			return
		}

		res.WriteHeader(nethttp.StatusOK)
		_, _ = res.Write([]byte("ok"))
	}))
	defer testServer.Close()

	client := GetDefaultClient()
	req, err := nethttp.NewRequest(nethttp.MethodPost, testServer.URL, bytes.NewReader(postPayload))
	assert.NoError(t, err)

	obsOptions := &ObservabilityOptions{}
	ctx := context.WithValue(req.Context(), obsOptions.GetKey(), obsOptions)
	req = req.WithContext(ctx)

	resp, err := client.Do(req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	if resp != nil {
		assert.Equal(t, nethttp.StatusOK, resp.StatusCode)
		if resp.Body != nil {
			_ = resp.Body.Close()
		}
	}
	assert.Equal(t, 2, attempts)
}
