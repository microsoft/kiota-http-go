package nethttplibrary

import (
	nethttp "net/http"
	"strconv"
	"strings"

	abs "github.com/microsoft/kiota-abstractions-go"
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
	reqOption, ok := req.Context().Value(parametersNameDecodingKeyValue).(parametersNameDecodingOptionsInt)
	if !ok {
		reqOption = &handler.options
	}
	if reqOption.GetEnable() &&
		len(reqOption.GetParametersToDecode()) != 0 &&
		strings.Contains(req.URL.RawQuery, "%") {
		for _, parameter := range reqOption.GetParametersToDecode() {
			valueToReplace := "%" + strconv.FormatInt(int64(parameter), 16)
			replacementValue := string(parameter)
			req.URL.RawQuery = strings.ReplaceAll(strings.ReplaceAll(req.URL.RawQuery, strings.ToUpper(valueToReplace), replacementValue), strings.ToLower(valueToReplace), replacementValue)
		}
	}
	return pipeline.Next(req, middlewareIndex)
}
