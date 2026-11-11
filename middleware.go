package nethttplibrary

import (
	nethttp "net/http"
	"reflect"
)

// Middleware interface for cross cutting concerns with HTTP requests and responses.
type Middleware interface {
	// Intercept intercepts the request and returns the response. The implementer MUST call pipeline.Next()
	Intercept(Pipeline, int, *nethttp.Request) (*nethttp.Response, error)
}

func isNilInterface(v any) bool {
	if v == nil {
		return true
	}
	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.Chan, reflect.Func, reflect.Map, reflect.Pointer, reflect.UnsafePointer, reflect.Interface, reflect.Slice:
		return val.IsNil()
	default:
		return false
	}
}
