package nethttplibrary

import (
	"context"
	"errors"
	"fmt"
	nethttp "net/http"
	"net/url"
	"strings"

	abs "github.com/microsoft/kiota-abstractions-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ProxyResolverFunc determines if a given URI will use a proxy.
// Returns the proxy URL if the destination uses a proxy, or nil if no proxy
// is used or the proxy is bypassed for this destination.
type ProxyResolverFunc func(uri *url.URL) (*url.URL, error)

// ScrubSensitiveHeadersFunc is a callback that determines which headers to
// remove during redirects. It receives the redirect request being prepared,
// the original URI, the new redirect URI, and a proxy resolver function.
type ScrubSensitiveHeadersFunc func(
	request *nethttp.Request,
	originalURI *url.URL,
	newURI *url.URL,
	proxyResolver ProxyResolverFunc,
)

// RedirectHandler handles redirect responses and follows them according to the options specified.
type RedirectHandler struct {
	// options to use when evaluating whether to redirect or not
	options RedirectHandlerOptions
}

// NewRedirectHandler creates a new redirect handler with the default options.
func NewRedirectHandler() *RedirectHandler {
	return NewRedirectHandlerWithOptions(RedirectHandlerOptions{
		MaxRedirects: defaultMaxRedirects,
		ShouldRedirect: func(req *nethttp.Request, res *nethttp.Response) bool {
			return true
		},
	})
}

// NewRedirectHandlerWithOptions creates a new redirect handler with the specified options.
func NewRedirectHandlerWithOptions(options RedirectHandlerOptions) *RedirectHandler {
	return &RedirectHandler{options: options}
}

// DefaultScrubSensitiveHeaders removes sensitive headers when redirecting across security boundaries.
//
// Removes Authorization and Cookie headers when:
//   - Host changes (e.g., example.com -> api.example.com)
//   - Scheme changes (e.g., https:// -> http://)
//   - Port changes (e.g., :80 -> :8080)
//
// Removes Proxy-Authorization header when:
//   - No proxy is configured (proxyResolver is nil)
//   - Proxy is bypassed for the new destination (proxyResolver returns nil or error)
func DefaultScrubSensitiveHeaders(
	request *nethttp.Request,
	originalURI *url.URL,
	newURI *url.URL,
	proxyResolver ProxyResolverFunc,
) {
	if request == nil || originalURI == nil || newURI == nil {
		return
	}

	// Remove Authorization and Cookie headers if host, scheme, or port changes
	isDifferentHostOrSchemeOrPort := !strings.EqualFold(newURI.Hostname(), originalURI.Hostname()) ||
		!strings.EqualFold(newURI.Scheme, originalURI.Scheme) ||
		newURI.Port() != originalURI.Port()

	if isDifferentHostOrSchemeOrPort {
		request.Header.Del("Authorization")
		request.Header.Del("Cookie")
	}

	// Remove Proxy-Authorization if no proxy is configured or the destination bypasses proxy
	isProxyInactive := proxyResolver == nil
	if !isProxyInactive {
		proxyURL, err := proxyResolver(newURI)
		// Treat errors conservatively - if we can't determine proxy status, remove the header
		isProxyInactive = (err != nil || proxyURL == nil)
	}

	if isProxyInactive {
		request.Header.Del("Proxy-Authorization")
	}
}

// getProxyResolverFromPipeline attempts to extract a ProxyResolverFunc from the pipeline.
// Returns nil if no proxy is configured, pipeline doesn't provide transport access,
// or transport is not *http.Transport.
func getProxyResolverFromPipeline(pipeline Pipeline) ProxyResolverFunc {
	// Try to get transport from pipeline via type assertion
	type transportAccessor interface {
		GetTransport() nethttp.RoundTripper
	}

	accessor, ok := pipeline.(transportAccessor)
	if !ok {
		return nil
	}

	transport := accessor.GetTransport()
	if transport == nil {
		return nil
	}

	// Extract Proxy function from http.Transport
	httpTransport, ok := transport.(*nethttp.Transport)
	if !ok || httpTransport.Proxy == nil {
		return nil
	}

	// Wrap the Proxy function to match ProxyResolverFunc signature
	return func(uri *url.URL) (*url.URL, error) {
		// Create a minimal request for the Proxy function
		req := &nethttp.Request{
			URL: uri,
		}
		return httpTransport.Proxy(req)
	}
}

// RedirectHandlerOptions to use when evaluating whether to redirect or not.
type RedirectHandlerOptions struct {
	// A callback that determines whether to redirect or not.
	ShouldRedirect func(req *nethttp.Request, res *nethttp.Response) bool
	// The maximum number of redirects to follow.
	MaxRedirects int
	// A callback that determines which headers to scrub during redirects.
	// Defaults to DefaultScrubSensitiveHeaders if nil.
	ScrubSensitiveHeaders ScrubSensitiveHeadersFunc
}

var redirectKeyValue = abs.RequestOptionKey{
	Key: "RedirectHandler",
}

type redirectHandlerOptionsInt interface {
	abs.RequestOption
	GetShouldRedirect() func(req *nethttp.Request, res *nethttp.Response) bool
	GetMaxRedirect() int
}

// GetKey returns the key value to be used when the option is added to the request context
func (options *RedirectHandlerOptions) GetKey() abs.RequestOptionKey {
	return redirectKeyValue
}

// GetShouldRedirect returns the redirection evaluation function.
func (options *RedirectHandlerOptions) GetShouldRedirect() func(req *nethttp.Request, res *nethttp.Response) bool {
	return options.ShouldRedirect
}

// GetMaxRedirect returns the maximum number of redirects to follow.
func (options *RedirectHandlerOptions) GetMaxRedirect() int {
	if options == nil || options.MaxRedirects < 1 {
		return defaultMaxRedirects
	} else if options.MaxRedirects > absoluteMaxRedirects {
		return absoluteMaxRedirects
	} else {
		return options.MaxRedirects
	}
}

const defaultMaxRedirects = 5
const absoluteMaxRedirects = 20
const movedPermanently = 301
const found = 302
const seeOther = 303
const temporaryRedirect = 307
const permanentRedirect = 308
const locationHeader = "Location"

// Intercept implements the interface and evaluates whether to follow a redirect response.
func (middleware RedirectHandler) Intercept(pipeline Pipeline, middlewareIndex int, req *nethttp.Request) (*nethttp.Response, error) {
	obsOptions := GetObservabilityOptionsFromRequest(req)
	ctx := req.Context()
	var span trace.Span
	var observabilityName string
	if obsOptions != nil {
		observabilityName = obsOptions.GetTracerInstrumentationName()
		ctx, span = otel.GetTracerProvider().Tracer(observabilityName).Start(ctx, "RedirectHandler_Intercept")
		span.SetAttributes(attribute.Bool("com.microsoft.kiota.handler.redirect.enable", true))
		defer span.End()
		req = req.WithContext(ctx)
	}
	response, err := pipeline.Next(req, middlewareIndex)
	if err != nil {
		return response, err
	}
	reqOption, ok := req.Context().Value(redirectKeyValue).(redirectHandlerOptionsInt)
	if !ok {
		reqOption = &middleware.options
	}
	return middleware.redirectRequest(ctx, pipeline, middlewareIndex, reqOption, req, response, 0, observabilityName)
}

func (middleware RedirectHandler) redirectRequest(ctx context.Context, pipeline Pipeline, middlewareIndex int, reqOption redirectHandlerOptionsInt, req *nethttp.Request, response *nethttp.Response, redirectCount int, observabilityName string) (*nethttp.Response, error) {
	shouldRedirect := reqOption.GetShouldRedirect() != nil && reqOption.GetShouldRedirect()(req, response) || reqOption.GetShouldRedirect() == nil
	if middleware.isRedirectResponse(response) &&
		redirectCount < reqOption.GetMaxRedirect() &&
		shouldRedirect {
		redirectCount++
		redirectRequest, err := middleware.getRedirectRequest(pipeline, req, response)
		if err != nil {
			return response, err
		}
		if observabilityName != "" {
			ctx, span := otel.GetTracerProvider().Tracer(observabilityName).Start(ctx, "RedirectHandler_Intercept - redirect "+fmt.Sprint(redirectCount))
			span.SetAttributes(attribute.Int("com.microsoft.kiota.handler.redirect.count", redirectCount),
				httpResponseStatusCodeAttribute.Int(response.StatusCode),
			)
			defer span.End()
			redirectRequest = redirectRequest.WithContext(ctx)
		}

		result, err := pipeline.Next(redirectRequest, middlewareIndex)
		if err != nil {
			return result, err
		}
		return middleware.redirectRequest(ctx, pipeline, middlewareIndex, reqOption, redirectRequest, result, redirectCount, observabilityName)
	}
	return response, nil
}

func (middleware RedirectHandler) isRedirectResponse(response *nethttp.Response) bool {
	if response == nil {
		return false
	}
	locationHeader := response.Header.Get(locationHeader)
	if locationHeader == "" {
		return false
	}
	statusCode := response.StatusCode
	return statusCode == movedPermanently || statusCode == found || statusCode == seeOther || statusCode == temporaryRedirect || statusCode == permanentRedirect
}

func (middleware RedirectHandler) getRedirectRequest(
	pipeline Pipeline,
	request *nethttp.Request,
	response *nethttp.Response,
) (*nethttp.Request, error) {
	if request == nil || response == nil {
		return nil, errors.New("request or response is nil")
	}
	locationHeaderValue := response.Header.Get(locationHeader)
	if locationHeaderValue[0] == '/' {
		locationHeaderValue = request.URL.Scheme + "://" + request.URL.Host + locationHeaderValue
	}
	result := request.Clone(request.Context())
	targetUrl, err := url.Parse(locationHeaderValue)
	if err != nil {
		return nil, err
	}
	result.URL = targetUrl
	if result.Host != targetUrl.Host {
		result.Host = targetUrl.Host
	}

	// Handle 303 See Other - change method and remove body-related headers
	if response.StatusCode == seeOther {
		result.Method = nethttp.MethodGet
		result.Header.Del("Content-Type")
		result.Header.Del("Content-Length")
		result.Body = nil
	}

	// Use callback to scrub sensitive headers, defaulting if not provided
	scrubFunc := middleware.options.ScrubSensitiveHeaders
	if scrubFunc == nil {
		scrubFunc = DefaultScrubSensitiveHeaders
	}

	// Get proxy resolver from pipeline
	proxyResolver := getProxyResolverFromPipeline(pipeline)

	// Call the scrub function
	scrubFunc(result, request.URL, targetUrl, proxyResolver)

	return result, nil
}
