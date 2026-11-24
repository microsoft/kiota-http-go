package nethttplibrary

import (
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	abstractions "github.com/microsoft/kiota-abstractions-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// CompressionHandler represents a compression middleware
type CompressionHandler struct {
	options CompressionOptions
}

// CompressionOptions is a configuration object for the CompressionHandler middleware
type CompressionOptions struct {
	enableCompression bool
}

type compression interface {
	abstractions.RequestOption
	ShouldCompress() bool
}

var compressKey = abstractions.RequestOptionKey{Key: "CompressionHandler"}

type compressedRequestKey struct{}

// NewCompressionHandler creates an instance of a compression middleware
func NewCompressionHandler() *CompressionHandler {
	options := NewCompressionOptionsReference(true)
	return NewCompressionHandlerWithOptions(*options)
}

// NewCompressionHandlerWithOptions creates an instance of the compression middleware with
// specified configurations.
func NewCompressionHandlerWithOptions(option CompressionOptions) *CompressionHandler {
	return &CompressionHandler{options: option}
}

// NewCompressionOptions creates a configuration object for the CompressionHandler
//
// Deprecated: This function is deprecated, and superseded by NewCompressionOptionsReference,
// which returns a pointer instead of plain value.
func NewCompressionOptions(enableCompression bool) CompressionOptions {
	return CompressionOptions{enableCompression: enableCompression}
}

// NewCompressionOptionsReference creates a configuration object for the CompressionHandler.
//
// This function supersedes the NewCompressionOptions function and returns a pointer,
// which is expected by GetDefaultMiddlewaresWithOptions.
func NewCompressionOptionsReference(enableCompression bool) *CompressionOptions {
	options := CompressionOptions{enableCompression: enableCompression}
	return &options
}

// GetKey returns CompressionOptions unique name in context object
func (o CompressionOptions) GetKey() abstractions.RequestOptionKey {
	return compressKey
}

// ShouldCompress reads compression setting form CompressionOptions
func (o CompressionOptions) ShouldCompress() bool {
	return o.enableCompression
}

// Intercept is invoked by the middleware pipeline to either move the request/response
// to the next middleware in the pipeline
func (c *CompressionHandler) Intercept(pipeline Pipeline, middlewareIndex int, req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("req cannot be nil")
	}

	reqOption, ok := req.Context().Value(compressKey).(compression)
	if !ok || isNilInterface(reqOption) {
		reqOption = c.options
	}

	obsOptions := GetObservabilityOptionsFromRequest(req)
	ctx := req.Context()
	var span trace.Span
	if obsOptions != nil {
		ctx, span = otel.GetTracerProvider().Tracer(obsOptions.GetTracerInstrumentationName()).Start(ctx, "CompressionHandler_Intercept")
		span.SetAttributes(attribute.Bool("com.microsoft.kiota.handler.compression.enable", true))
		defer span.End()
		req = req.WithContext(ctx)
	}

	if !reqOption.ShouldCompress() || contentRangeBytesIsPresent(req.Header) || contentEncodingIsPresent(req.Header) || req.Body == nil {
		return pipeline.Next(req, middlewareIndex)
	}
	if span != nil {
		span.SetAttributes(attribute.Bool("http.request_body_compressed", true))
	}

	unCompressedBody, err := io.ReadAll(req.Body)
	if err != nil {
		if span != nil {
			span.RecordError(err)
		}
		return nil, err
	}

	compressedBody, err := compressReqBody(unCompressedBody)
	if err != nil {
		if span != nil {
			span.RecordError(err)
		}
		return nil, err
	}

	originalGetBody := req.GetBody
	originalContentLength := req.ContentLength
	originalLengthHeader, hasLengthHeader := req.Header["Content-Length"]
	getUncompressedBody := func() io.ReadSeekCloser {
		return NopCloser(bytes.NewReader(unCompressedBody))
	}
	if originalGetBody == nil {
		originalGetBody = func() (io.ReadCloser, error) {
			return getUncompressedBody(), nil
		}
	}
	getCompressedBody := func() io.ReadSeekCloser {
		return NopCloser(bytes.NewReader(compressedBody))
	}
	compressionAttempt := new(byte)
	req = req.Clone(context.WithValue(req.Context(), compressedRequestKey{}, compressionAttempt))
	req.Header.Set("Content-Encoding", "gzip")
	req.Body = getCompressedBody()
	req.GetBody = func() (io.ReadCloser, error) {
		return getCompressedBody(), nil
	}
	req.ContentLength = int64(len(compressedBody))
	if hasLengthHeader {
		req.Header.Set("Content-Length", strconv.Itoa(len(compressedBody)))
	}

	if span != nil {
		span.SetAttributes(httpRequestBodySizeAttribute.Int(int(req.ContentLength)))
	}

	// Sending request with compressed body
	resp, err := pipeline.Next(req, middlewareIndex)
	if err != nil {
		return nil, err
	}

	// A 415 can come from a later redirect hop. Only resend the request that actually carried this gzip body.
	if resp != nil && resp.StatusCode == http.StatusUnsupportedMediaType &&
		resp.Request != nil && resp.Request.Context().Value(compressedRequestKey{}) == compressionAttempt &&
		resp.Request.Body != nil && resp.Request.Body != http.NoBody && resp.Request.Header.Get("Content-Encoding") == "gzip" {
		if resp.Body != nil {
			_ = resp.Body.Close()
		}
		fallback := resp.Request.Clone(resp.Request.Context())
		fallback.Header.Del("Content-Encoding")
		fallback.Body = getUncompressedBody()
		fallback.GetBody = originalGetBody
		fallback.ContentLength = originalContentLength
		if hasLengthHeader {
			fallback.Header["Content-Length"] = originalLengthHeader
		} else {
			fallback.Header.Del("Content-Length")
		}

		if span != nil {
			span.SetAttributes(httpRequestBodySizeAttribute.Int(int(fallback.ContentLength)),
				httpResponseStatusCodeAttribute.Int(415))
		}

		return pipeline.Next(fallback, middlewareIndex)
	}

	return resp, nil
}

func contentRangeBytesIsPresent(header http.Header) bool {
	contentRanges, _ := header["Content-Range"]
	for _, contentRange := range contentRanges {
		if strings.Contains(strings.ToLower(contentRange), "bytes") {
			return true
		}
	}
	return false
}

func contentEncodingIsPresent(header http.Header) bool {
	_, ok := header["Content-Encoding"]
	return ok
}

func compressReqBody(reqBody []byte) ([]byte, error) {
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	if _, err := gzipWriter.Write(reqBody); err != nil {
		return nil, err
	}

	if err := gzipWriter.Close(); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}
