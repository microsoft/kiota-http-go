package nethttplibrary

import (
	nethttp "net/http"

	abs "github.com/microsoft/kiota-abstractions-go"
)

// ObservabilityOptions holds the tracing, metrics and logging configuration for the request adapter
type ObservabilityOptions struct {
	// Whether to include attributes which could contains EUII information like URLs
	IncludeEUIIAttibutes bool
}

// GetTracerInstrumentationName returns the observability name to use for the tracer
func (o *ObservabilityOptions) GetTracerInstrumentationName() string {
	return "github.com/microsoft/kiota-http-go"
}

// GetIncludeEUIIAttibutes returns whether to include attributes which could contains EUII information
func (o *ObservabilityOptions) GetIncludeEUIIAttibutes() bool {
	return o.IncludeEUIIAttibutes
}

// SetIncludeEUIIAttibutes set whether to include attributes which could contains EUII information
func (o *ObservabilityOptions) SetIncludeEUIIAttibutes(value bool) {
	o.IncludeEUIIAttibutes = value
}

// ObservabilityOptionsInt defines the options contract for handlers
type ObservabilityOptionsInt interface {
	abs.RequestOption
	GetTracerInstrumentationName() string
	GetIncludeEUIIAttibutes() bool
	SetIncludeEUIIAttibutes(value bool)
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
