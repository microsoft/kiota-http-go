package nethttplibrary

import (
	"io"
	nethttp "net/http"
)

// Middleware interface for cross cutting concerns with HTTP requests and responses.
type Middleware interface {
	// Intercept intercepts the request and returns the response. The implementer MUST call pipeline.Next()
	Intercept(Pipeline, int, *nethttp.Request) (*nethttp.Response, error)
}

const maxDrainBytes int64 = 4096

// drainAndCloseResponseBody cleans up intermediate response bodies using a bounded cleanup policy.
// It drains small, bounded response bodies (ContentLength <= 4KB) to facilitate connection reuse,
// and immediately closes large or unknown-length bodies without reading to prevent hangs or timeouts.
func drainAndCloseResponseBody(resp *nethttp.Response) {
	if resp == nil || resp.Body == nil {
		return
	}
	if resp.ContentLength >= 0 && resp.ContentLength <= maxDrainBytes {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxDrainBytes))
	}
	_ = resp.Body.Close()
}

type requestBodyTrackerKey struct{}

type requestBodyTracker struct {
	body          io.ReadCloser
	contentLength int64
	getBody       func() (io.ReadCloser, error)
	modified      bool
}
