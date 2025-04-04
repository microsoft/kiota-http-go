package nethttplibrary

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
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
	reqOption, ok := req.Context().Value(compressKey).(compression)
	if !ok {
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
	unCompressedContentLength := req.ContentLength
	if err != nil {
		if span != nil {
			span.RecordError(err)
		}
		return nil, err
	}

	compressedBody, size, err := compressReqBody(unCompressedBody)
	if err != nil {
		if span != nil {
			span.RecordError(err)
		}
		return nil, err
	}

	req.Header.Set("Content-Encoding", "gzip")
	req.Body = compressedBody
	req.ContentLength = int64(size)

	if span != nil {
		span.SetAttributes(httpRequestBodySizeAttribute.Int(int(req.ContentLength)))
	}

	// Sending request with compressed body
	resp, err := pipeline.Next(req, middlewareIndex)
	if err != nil {
		return nil, err
	}

	// If response has status 415 retry request with uncompressed body
	if resp.StatusCode == 415 {
		delete(req.Header, "Content-Encoding")
		req.Body = io.NopCloser(bytes.NewBuffer(unCompressedBody))
		req.ContentLength = unCompressedContentLength

		if span != nil {
			span.SetAttributes(httpRequestBodySizeAttribute.Int(int(req.ContentLength)),
				httpResponseStatusCodeAttribute.Int(415))
		}

		return pipeline.Next(req, middlewareIndex)
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

func compressReqBody(reqBody []byte) (io.ReadSeekCloser, int, error) {
	var buffer bytes.Buffer
	gzipWriter := gzip.NewWriter(&buffer)
	if _, err := gzipWriter.Write(reqBody); err != nil {
		return nil, 0, err
	}

	if err := gzipWriter.Close(); err != nil {
		return nil, 0, err
	}

	reader := bytes.NewReader(buffer.Bytes())
	return NopCloser(reader), buffer.Len(), nil
}
