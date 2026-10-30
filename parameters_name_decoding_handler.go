package nethttplibrary

import (
	"errors"
	nethttp "net/http"
	"strconv"
	"strings"

	abs "github.com/microsoft/kiota-abstractions-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// ParametersNameDecodingOptions defines the options for the ParametersNameDecodingHandler
type ParametersNameDecodingOptions struct {
	// Enable defines if the parameters name decoding should be enabled
	Enable bool
	// ParametersToDecode defines the characters that should be decoded
	ParametersToDecode []byte
}

// ParametersNameDecodingHandler decodes special characters in the request query parameters that had to be encoded due to RFC 6570 restrictions names before executing the request.
type ParametersNameDecodingHandler struct {
	options ParametersNameDecodingOptions
}

// NewParametersNameDecodingHandler creates a new ParametersNameDecodingHandler with default options
func NewParametersNameDecodingHandler() *ParametersNameDecodingHandler {
	return NewParametersNameDecodingHandlerWithOptions(ParametersNameDecodingOptions{
		Enable:             true,
		ParametersToDecode: []byte{'-', '.', '~', '$'},
	})
}

// NewParametersNameDecodingHandlerWithOptions creates a new ParametersNameDecodingHandler with the given options
func NewParametersNameDecodingHandlerWithOptions(options ParametersNameDecodingOptions) *ParametersNameDecodingHandler {
	return &ParametersNameDecodingHandler{options: options}
}

type parametersNameDecodingOptionsInt interface {
	abs.RequestOption
	GetEnable() bool
	GetParametersToDecode() []byte
}

var parametersNameDecodingKeyValue = abs.RequestOptionKey{
	Key: "ParametersNameDecodingHandler",
}

// GetKey returns the key value to be used when the option is added to the request context
func (options *ParametersNameDecodingOptions) GetKey() abs.RequestOptionKey {
	return parametersNameDecodingKeyValue
}

// GetEnable returns the enable value from the option
func (options *ParametersNameDecodingOptions) GetEnable() bool {
	return options.Enable
}

// GetParametersToDecode returns the parametersToDecode value from the option
func (options *ParametersNameDecodingOptions) GetParametersToDecode() []byte {
	return options.ParametersToDecode
}

// Intercept implements the RequestInterceptor interface and decodes the parameters name
func (handler *ParametersNameDecodingHandler) Intercept(pipeline Pipeline, middlewareIndex int, req *nethttp.Request) (*nethttp.Response, error) {
	if req == nil {
		return nil, errors.New("req cannot be nil")
	}
	if req.URL == nil {
		return pipeline.Next(req, middlewareIndex)
	}
	reqOption, ok := req.Context().Value(parametersNameDecodingKeyValue).(parametersNameDecodingOptionsInt)
	if !ok || isNil(reqOption) {
		reqOption = &handler.options
	}
	obsOptions := GetObservabilityOptionsFromRequest(req)
	ctx := req.Context()
	if obsOptions != nil {
		ctx, span := otel.GetTracerProvider().Tracer(obsOptions.GetTracerInstrumentationName()).Start(ctx, "ParametersNameDecodingHandler_Intercept")
		span.SetAttributes(attribute.Bool("com.microsoft.kiota.handler.parameters_name_decoding.enable", reqOption.GetEnable()))
		req = req.WithContext(ctx)
		defer span.End()
	}
	if reqOption.GetEnable() &&
		len(reqOption.GetParametersToDecode()) != 0 &&
		strings.Contains(req.URL.RawQuery, "%") {
		req.URL.RawQuery = decodeQueryParameters(req.URL.RawQuery, reqOption.GetParametersToDecode())
	}
	return pipeline.Next(req, middlewareIndex)
}

func decodeQueryParameters(originalRawQuery string, parametersToDecode []byte) string {
	if originalRawQuery == "" || len(parametersToDecode) == 0 || !strings.Contains(originalRawQuery, "%") {
		return originalRawQuery
	}
	parts := strings.Split(originalRawQuery, "&")
	for i, part := range parts {
		if part == "" {
			continue
		}
		eqIdx := strings.Index(part, "=")
		if eqIdx == -1 {
			parts[i] = decodeUriEncodedString(part, parametersToDecode)
		} else {
			name := part[:eqIdx]
			value := part[eqIdx+1:]
			parts[i] = decodeUriEncodedString(name, parametersToDecode) + "=" + value
		}
	}
	return strings.Join(parts, "&")
}

func decodeUriEncodedString(originalValue string, parametersToDecode []byte) string {
	resultValue := originalValue
	for _, parameter := range parametersToDecode {
		valueToReplace := "%" + strconv.FormatInt(int64(parameter), 16)
		replacementValue := string(parameter)
		resultValue = strings.ReplaceAll(strings.ReplaceAll(resultValue, strings.ToUpper(valueToReplace), replacementValue), strings.ToLower(valueToReplace), replacementValue)
	}
	return resultValue
}
