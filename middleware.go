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

func isNil(i any) bool {
	if i == nil {
		return true
	}
	switch reflect.TypeOf(i).Kind() {
	case reflect.Ptr, reflect.Map, reflect.Array, reflect.Chan, reflect.Slice, reflect.Interface:
		return reflect.ValueOf(i).IsNil()
	}
	return false
}
