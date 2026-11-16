package nethttplibrary

import (
	"errors"
	abstractions "github.com/microsoft/kiota-abstractions-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
	"net/http"
	"net/url"
	"strings"
)

var urlReplaceOptionKey = abstractions.RequestOptionKey{Key: "UrlReplaceOptionKey"}

// UrlReplaceHandler is a middleware handler that replaces url segments in the uri path.
type UrlReplaceHandler struct {
	options UrlReplaceOptions
}

// NewUrlReplaceOptions creates a new UrlReplaceOptions with default options
func NewUrlReplaceOptions() *UrlReplaceOptions {
	return &UrlReplaceOptions{
		Enabled:          true,
		ReplacementPairs: nil,
	}
}

// NewUrlReplaceHandler creates a new UrlReplaceHandler with the specified settings
func NewUrlReplaceHandler(enabled bool, replacementPairs map[string]string) *UrlReplaceHandler {
	return &UrlReplaceHandler{UrlReplaceOptions{Enabled: enabled, ReplacementPairs: replacementPairs}}
}

// NewUrlReplaceHandlerWithOptions creates a new UrlReplaceHandler with the given options
func NewUrlReplaceHandlerWithOptions(options UrlReplaceOptions) *UrlReplaceHandler {
	return &UrlReplaceHandler{options: options}
}

// UrlReplaceOptions is a configuration object for the UrlReplaceHandler middleware
type UrlReplaceOptions struct {
	Enabled          bool
	ReplacementPairs map[string]string
}

// GetKey returns UrlReplaceOptions unique name in context object
func (u *UrlReplaceOptions) GetKey() abstractions.RequestOptionKey {
	return urlReplaceOptionKey
}

// GetReplacementPairs reads ReplacementPairs settings from UrlReplaceOptions
func (u *UrlReplaceOptions) GetReplacementPairs() map[string]string {
	if u == nil {
		return nil
	}
	return u.ReplacementPairs
}

// IsEnabled reads Enabled setting from UrlReplaceOptions
func (u *UrlReplaceOptions) IsEnabled() bool {
	if u == nil {
		return false
	}
	return u.Enabled
}

type urlReplaceOptionsInt interface {
	abstractions.RequestOption
	IsEnabled() bool
	GetReplacementPairs() map[string]string
}

// Intercept is invoked by the middleware pipeline to either move the request/response
// to the next middleware in the pipeline
func (c *UrlReplaceHandler) Intercept(pipeline Pipeline, middlewareIndex int, req *http.Request) (*http.Response, error) {
	if req == nil {
		return nil, errors.New("req cannot be nil")
	}
	if req.URL == nil {
		return nil, errors.New("req.URL cannot be nil")
	}

	reqOption, ok := req.Context().Value(urlReplaceOptionKey).(urlReplaceOptionsInt)
	if !ok || isNilInterface(reqOption) {
		reqOption = &c.options
	}

	obsOptions := GetObservabilityOptionsFromRequest(req)
	ctx := req.Context()
	var span trace.Span
	if obsOptions != nil && !isNilInterface(obsOptions) {
		ctx, span = otel.GetTracerProvider().Tracer(obsOptions.GetTracerInstrumentationName()).Start(ctx, "UrlReplaceHandler_Intercept")
		span.SetAttributes(attribute.Bool("com.microsoft.kiota.handler.url_replacer.enable", true))
		defer span.End()
		req = req.WithContext(ctx)
	}

	if !reqOption.IsEnabled() || len(reqOption.GetReplacementPairs()) == 0 {
		response, err := pipeline.Next(req, middlewareIndex)
		if err != nil && span != nil {
			span.RecordError(err)
		}
		return response, err
	}

	if req.URL.RawPath == "" {
		req.URL.Path = ReplacePathTokens(req.URL.Path, reqOption.GetReplacementPairs())
	} else {
		req.URL.Path, req.URL.RawPath = replacePathAndRawPath(req.URL.Path, req.URL.RawPath, reqOption.GetReplacementPairs())
	}

	if span != nil && obsOptions != nil && !isNilInterface(obsOptions) && obsOptions.GetIncludeEUIIAttributes() {
		reqURI := req.RequestURI
		if reqURI == "" && req.URL != nil {
			reqURI = req.URL.String()
		}
		span.SetAttributes(attribute.String("http.request_url", reqURI))
	}

	response, err := pipeline.Next(req, middlewareIndex)
	if err != nil && span != nil {
		span.RecordError(err)
	}
	return response, err
}

// ReplacePathTokens invokes token replacement logic on the given url path
func ReplacePathTokens(path string, replacementPairs map[string]string) string {
	for key, value := range replacementPairs {
		path = strings.Replace(path, key, value, 1)
	}
	return path
}

func replacePathAndRawPath(path, rawPath string, replacementPairs map[string]string) (string, string) {
	for key, value := range replacementPairs {
		idx := strings.Index(path, key)
		if idx == -1 {
			continue
		}
		indices, ok := mapPathToRawIndices(path, rawPath)
		if !ok {
			path = strings.Replace(path, key, value, 1)
			continue
		}
		rawStart := indices[idx]
		rawEnd := indices[idx+len(key)]
		path = path[:idx] + value + path[idx+len(key):]
		rawValue := (&url.URL{Path: value}).EscapedPath()
		rawPath = rawPath[:rawStart] + rawValue + rawPath[rawEnd:]
	}
	return path, rawPath
}

func mapPathToRawIndices(path, rawPath string) ([]int, bool) {
	indices := make([]int, len(path)+1)
	r, p := 0, 0
	for p < len(path) && r < len(rawPath) {
		indices[p] = r
		if rawPath[r] == '%' && r+2 < len(rawPath) && isHex(rawPath[r+1]) && isHex(rawPath[r+2]) {
			val := (unhex(rawPath[r+1]) << 4) | unhex(rawPath[r+2])
			if path[p] != val {
				return nil, false
			}
			r += 3
			p++
		} else {
			if path[p] != rawPath[r] {
				return nil, false
			}
			r++
			p++
		}
	}
	if p != len(path) || r != len(rawPath) {
		return nil, false
	}
	indices[len(path)] = len(rawPath)
	return indices, true
}

func isHex(c byte) bool {
	return ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F')
}

func unhex(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}
