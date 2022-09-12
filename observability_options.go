package nethttplibrary

import (
	nethttp "net/http"

	abs "github.com/microsoft/kiota-abstractions-go"
)

// ObservabilityOptions defines the options for handlers
type ObservabilityOptions struct {
	observabilityName string
}

// GetObservabilityName returns the observability name to use for the tracer
func (o *ObservabilityOptions) GetObservabilityName() string {
	return o.observabilityName
}

// ObservabilityOptionsInt defines the options contract for handlers
type ObservabilityOptionsInt interface {
	abs.RequestOption
	GetObservabilityName() string
}

func (*ObservabilityOptions) GetKey() abs.RequestOptionKey {
	return observabilityOptionsKeyValue
}

var observabilityOptionsKeyValue = abs.RequestOptionKey{
	Key: "ObservabilityOptions",
}

// GetObservabilityOptionsFromRequest returns the observability options from the request context
func GetObservabilityOptionsFromRequest(req *nethttp.Request) ObservabilityOptionsInt {
	if options, ok := req.Context().Value(observabilityOptionsKeyValue).(ObservabilityOptionsInt); ok {
		return options
	}
	return nil
}
