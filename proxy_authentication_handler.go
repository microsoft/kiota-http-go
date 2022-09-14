package nethttplibrary

import (
	"encoding/base64"
	abstractions "github.com/microsoft/kiota-abstractions-go"
	"net/http"
)

// ProxyAuthenticationHandler represents a proxyAuthentication middleware
type ProxyAuthenticationHandler struct {
	options ProxyAuthenticationOptions
}

// ProxyAuthenticationOptions is a configuration object for the ProxyAuthenticationHandler middleware
type ProxyAuthenticationOptions struct {
	userName             *string
	password             *string
	authenticationHeader *string
}

type proxyAuthentication interface {
	abstractions.RequestOption
	GetAuthenticationHeader() *string
	HasAuthentication() bool
}

var proxyAuthenticationKey = abstractions.RequestOptionKey{Key: "ProxyAuthenticationHandler"}

// NewProxyAuthenticationHandlerWithOptions creates an instance of the ProxyAuthentication middleware with
// specified configurations.
func NewProxyAuthenticationHandlerWithOptions(option ProxyAuthenticationOptions) *ProxyAuthenticationHandler {
	return &ProxyAuthenticationHandler{options: option}
}

// NewProxyAuthenticationOptions creates a configuration object for the ProxyAuthenticationHandler
func NewProxyAuthenticationOptions(userName *string, password *string) ProxyAuthenticationOptions {
	return ProxyAuthenticationOptions{userName: userName, password: password}
}

// GetKey returns CompressionOptions unique name in context object
func (o *ProxyAuthenticationOptions) GetKey() abstractions.RequestOptionKey {
	return proxyAuthenticationKey
}

// GetAuthenticationHeader return base64 encrypted authentication username
func (o *ProxyAuthenticationOptions) GetAuthenticationHeader() *string {
	if o.authenticationHeader == nil && o.HasAuthentication() {
		header := *o.userName + ":" + *o.password
		o.authenticationHeader = &header
	}
	return o.authenticationHeader
}

// HasAuthentication return authentication password
func (o *ProxyAuthenticationOptions) HasAuthentication() bool {
	return o.userName != nil && o.password != nil
}

// Intercept is invoked by the middleware pipeline to either move the request/response
// to the next middleware in the pipeline
func (p *ProxyAuthenticationHandler) Intercept(pipeline Pipeline, middlewareIndex int, req *http.Request) (*http.Response, error) {
	reqOption, ok := req.Context().Value(proxyAuthenticationKey).(proxyAuthentication)
	if !ok {
		reqOption = &p.options
	}

	if !reqOption.HasAuthentication() {
		return pipeline.Next(req, middlewareIndex)
	}

	auth := *reqOption.GetAuthenticationHeader()
	basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))

	req.Header.Add("Proxy-Authorization", basicAuth)
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	// Sending request with authentication headers body
	return pipeline.Next(req, middlewareIndex)
}
