package nethttplibrary

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	nethttp "net/http"
	"strconv"
	"strings"
	"time"

	abs "github.com/microsoft/kiota-abstractions-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// RetryHandler handles transient HTTP responses and retries the request given the retry options
type RetryHandler struct {
	// default options to use when evaluating the response
	options RetryHandlerOptions
}

// NewRetryHandler creates a new RetryHandler with default options
func NewRetryHandler() *RetryHandler {
	return NewRetryHandlerWithOptions(RetryHandlerOptions{
		ShouldRetry: func(delay time.Duration, executionCount int, request *nethttp.Request, response *nethttp.Response) bool {
			return true
		},
	})
}

// NewRetryHandlerWithOptions creates a new RetryHandler with the given options
func NewRetryHandlerWithOptions(options RetryHandlerOptions) *RetryHandler {
	return &RetryHandler{options: options}
}

const defaultMaxRetries = 3
const absoluteMaxRetries = 10
const defaultDelaySeconds = 3
const absoluteMaxDelaySeconds = 180

// RetryHandlerOptions to apply when evaluating the response for retrial
type RetryHandlerOptions struct {
	// Callback to determine if the request should be retried
	ShouldRetry func(delay time.Duration, executionCount int, request *nethttp.Request, response *nethttp.Response) bool
	// The maximum number of times a request can be retried
	MaxRetries int
	// The delay in seconds between retries
	DelaySeconds int
	// Whether POST and PATCH requests are retried only after a response showing the server didn't process them. This defaults to false.
	//
	// POST and PATCH aren't idempotent, so resending one the server already processed can repeat its side effect, such as sending an email twice.
	// A 504 can arrive after the server processed the request.
	// When this is true, POST and PATCH requests are retried only after a 429 or 503 response, and a 504 is returned to the caller.
	// This relies on the common service convention, followed by Microsoft Graph, that a 429 or 503 response means the server didn't process the request.
	// HTTP doesn't guarantee this, so a service that returns 429 or 503 after acting on a request can still see it repeated.
	// PUT and DELETE also change state, but they're idempotent, so they aren't affected, and neither are other methods.
	RetryUnprocessedMutationsOnly bool
}

type retryHandlerOptionsInt interface {
	abs.RequestOption
	GetShouldRetry() func(delay time.Duration, executionCount int, request *nethttp.Request, response *nethttp.Response) bool
	GetDelaySeconds() int
	GetMaxRetries() int
}

// retryUnprocessedMutationsOnlyOption is checked separately from retryHandlerOptionsInt,
// so that option types written for the existing interface keep satisfying it.
type retryUnprocessedMutationsOnlyOption interface {
	GetRetryUnprocessedMutationsOnly() bool
}

var retryKeyValue = abs.RequestOptionKey{
	Key: "RetryHandler",
}

// GetKey returns the key value to be used when the option is added to the request context
func (options *RetryHandlerOptions) GetKey() abs.RequestOptionKey {
	return retryKeyValue
}

// GetShouldRetry returns the should retry callback function which evaluates the response for retrial
func (options *RetryHandlerOptions) GetShouldRetry() func(delay time.Duration, executionCount int, request *nethttp.Request, response *nethttp.Response) bool {
	if options == nil || options.ShouldRetry == nil {
		return func(delay time.Duration, executionCount int, request *nethttp.Request, response *nethttp.Response) bool {
			return true
		}
	}
	return options.ShouldRetry
}

// GetDelaySeconds returns the delays in seconds between retries
func (options *RetryHandlerOptions) GetDelaySeconds() int {
	if options == nil || options.DelaySeconds < 1 {
		return defaultDelaySeconds
	} else if options.DelaySeconds > absoluteMaxDelaySeconds {
		return absoluteMaxDelaySeconds
	} else {
		return options.DelaySeconds
	}
}

// GetRetryUnprocessedMutationsOnly returns whether POST and PATCH requests are retried only after a 429 or 503 response
func (options *RetryHandlerOptions) GetRetryUnprocessedMutationsOnly() bool {
	if options == nil {
		return false
	}
	return options.RetryUnprocessedMutationsOnly
}

// GetMaxRetries returns the maximum number of times a request can be retried
func (options *RetryHandlerOptions) GetMaxRetries() int {
	if options == nil || options.MaxRetries < 1 {
		return defaultMaxRetries
	} else if options.MaxRetries > absoluteMaxRetries {
		return absoluteMaxRetries
	} else {
		return options.MaxRetries
	}
}

const retryAttemptHeader = "Retry-Attempt"
const retryAfterHeader = "Retry-After"

const tooManyRequests = 429
const serviceUnavailable = 503
const gatewayTimeout = 504

// Intercept implements the interface and evaluates whether to retry a failed request.
func (middleware RetryHandler) Intercept(pipeline Pipeline, middlewareIndex int, req *nethttp.Request) (*nethttp.Response, error) {
	if req == nil {
		return nil, errors.New("req cannot be nil")
	}
	obsOptions := GetObservabilityOptionsFromRequest(req)
	ctx := req.Context()
	var span trace.Span
	var observabilityName string
	if obsOptions != nil {
		observabilityName = obsOptions.GetTracerInstrumentationName()
		ctx, span = otel.GetTracerProvider().Tracer(observabilityName).Start(ctx, "RetryHandler_Intercept")
		span.SetAttributes(attribute.Bool("com.microsoft.kiota.handler.retry.enable", true))
		defer span.End()
		req = req.WithContext(ctx)
	}
	response, err := pipeline.Next(req, middlewareIndex)
	if err != nil {
		if span != nil {
			span.RecordError(err)
		}
		return response, err
	}
	reqOption, ok := req.Context().Value(retryKeyValue).(retryHandlerOptionsInt)
	if !ok {
		reqOption = &middleware.options
	}
	response, err = middleware.retryRequest(ctx, pipeline, middlewareIndex, reqOption, req, response, 0, 0, observabilityName)
	if err != nil && span != nil {
		span.RecordError(err)
	}
	return response, err
}

func (middleware RetryHandler) retryRequest(ctx context.Context, pipeline Pipeline, middlewareIndex int, options retryHandlerOptionsInt, req *nethttp.Request, resp *nethttp.Response, executionCount int, cumulativeDelay time.Duration, observabilityName string) (*nethttp.Response, error) {
	if resp == nil {
		return nil, errors.New("response cannot be nil")
	}
	if middleware.isRetriableErrorCode(resp.StatusCode) &&
		middleware.isRetriableRequest(req) &&
		middleware.isRetriableForMethod(options, req, resp) &&
		executionCount < options.GetMaxRetries() &&
		cumulativeDelay < time.Duration(absoluteMaxDelaySeconds)*time.Second &&
		options.GetShouldRetry()(cumulativeDelay, executionCount, req, resp) {
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		executionCount++
		delay := middleware.getRetryDelay(req, resp, options, executionCount)
		cumulativeDelay += delay
		req.Header.Set(retryAttemptHeader, strconv.Itoa(executionCount))
		if req.Body != nil {
			s, ok := req.Body.(io.Seeker)
			if ok {
				s.Seek(0, io.SeekStart)
			}
		}
		var attemptSpan trace.Span
		if observabilityName != "" {
			var attemptCtx context.Context
			attemptCtx, attemptSpan = otel.GetTracerProvider().Tracer(observabilityName).Start(ctx, "RetryHandler_Intercept - attempt "+fmt.Sprint(executionCount))
			attemptSpan.SetAttributes(attribute.Int("http.request.resend_count", executionCount),

				httpResponseStatusCodeAttribute.Int(resp.StatusCode),
				attribute.Float64("http.request.resend_delay", delay.Seconds()),
			)
			defer attemptSpan.End()
			req = req.WithContext(attemptCtx)
		}
		t := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			// Return without retrying if the context was cancelled.
			if attemptSpan != nil {
				attemptSpan.RecordError(ctx.Err())
			}
			return nil, ctx.Err()

			// Leaving this case empty causes it to exit the switch-block.
		case <-t.C:
		}
		response, err := pipeline.Next(req, middlewareIndex)
		if err != nil {
			if attemptSpan != nil {
				attemptSpan.RecordError(err)
			}
			return response, err
		}
		return middleware.retryRequest(ctx, pipeline, middlewareIndex, options, req, response, executionCount, cumulativeDelay, observabilityName)
	}
	return resp, nil
}

func (middleware RetryHandler) isRetriableErrorCode(code int) bool {
	return code == tooManyRequests || code == serviceUnavailable || code == gatewayTimeout
}
func (middleware RetryHandler) isRetriableRequest(req *nethttp.Request) bool {
	isBodiedMethod := req.Method == "POST" || req.Method == "PUT" || req.Method == "PATCH" || req.Method == "QUERY"
	if isBodiedMethod && req.Body != nil {
		return req.ContentLength != -1
	}
	return true
}

// isRetriableForMethod reports whether the response allows resending the request, given its method.
// The retry resends req itself, even when a redirect changed the method along the way, so req.Method is the method to check,
// and a POST or PATCH that was redirected to another method isn't retried at all.
func (middleware RetryHandler) isRetriableForMethod(options retryHandlerOptionsInt, req *nethttp.Request, resp *nethttp.Response) bool {
	option, ok := options.(retryUnprocessedMutationsOnlyOption)
	if !ok || isNilInterface(option) || !option.GetRetryUnprocessedMutationsOnly() || !isNonIdempotentMethod(req.Method) {
		return true
	}
	// A redirect that changed the method (a 301, 302 or 303 turning a POST into a GET) means the
	// server already accepted the original request. The status then describes the redirected
	// request, not the original one that a retry would resend.
	if resp.Request != nil && !strings.EqualFold(resp.Request.Method, req.Method) {
		return false
	}
	// By common service convention, a 429 or 503 means the server didn't process the request,
	// so resending it is treated as safe. A 504 may arrive after the server already acted on it.
	return resp.StatusCode == tooManyRequests || resp.StatusCode == serviceUnavailable
}

func isNonIdempotentMethod(method string) bool {
	return strings.EqualFold(method, nethttp.MethodPost) || strings.EqualFold(method, nethttp.MethodPatch)
}

func (middleware RetryHandler) getRetryDelay(req *nethttp.Request, resp *nethttp.Response, options retryHandlerOptionsInt, executionCount int) time.Duration {
	maxDelay := time.Duration(absoluteMaxDelaySeconds) * time.Second
	var delay time.Duration
	retryAfter := resp.Header.Get(retryAfterHeader)
	if retryAfter != "" {
		retryAfterDelay, err := strconv.ParseFloat(retryAfter, 64)
		if err == nil {
			if !math.IsNaN(retryAfterDelay) && !math.IsInf(retryAfterDelay, 0) && retryAfterDelay >= 0 {
				if retryAfterDelay > float64(absoluteMaxDelaySeconds) {
					retryAfterDelay = float64(absoluteMaxDelaySeconds)
				}
				delay = time.Duration(retryAfterDelay * float64(time.Second))
			} else {
				delay = exponentialBackOffDelay(options.GetDelaySeconds(), executionCount)
			}
		} else {
			// parse the header if it's a date (supports RFC1123, RFC850, and ANSIC per RFC 9110)
			t, err := nethttp.ParseTime(retryAfter)
			if err != nil {
				t, err = time.Parse(time.RFC1123, retryAfter)
			}
			if err == nil {
				now := time.Now()
				if t.Before(now) {
					delay = 0
				} else if t.After(now.Add(maxDelay)) {
					delay = maxDelay
				} else {
					delay = t.Sub(now)
				}
			} else {
				delay = exponentialBackOffDelay(options.GetDelaySeconds(), executionCount)
			}
		}
	} else {
		delay = exponentialBackOffDelay(options.GetDelaySeconds(), executionCount)
	}
	if delay > maxDelay {
		delay = maxDelay
	}
	return delay
}

func exponentialBackOffDelay(delaySeconds, executionCount int) time.Duration {
	exp := executionCount - 1 // executionCount is 1 for first retry
	delay := time.Duration(delaySeconds*(1<<exp)) * time.Second
	jitter := time.Duration(rand.Float64()*1000) * time.Millisecond
	return delay + jitter
}
