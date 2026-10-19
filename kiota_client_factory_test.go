package nethttplibrary

import (
	abstractions "github.com/microsoft/kiota-abstractions-go"
	"github.com/stretchr/testify/assert"
	nethttp "net/http"
	"testing"
	"time"
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
