package nethttplibrary

import "go.opentelemetry.io/otel/attribute"

// HTTP Request attributes
const (
	HttpRequestBodySizeAttribute    = attribute.Key("http.request.body.size")
	HttpRequestResendCountAttribute = attribute.Key("http.request.resend_count")
	HttpRequestMethodAttribute      = attribute.Key("http.request.method")
)

// HTTP Response attributes
const (
	HttpResponseBodySizeAttribute          = attribute.Key("http.response.body.size")
	HttpResponseHeaderContentTypeAttribute = attribute.Key("http.response.header.content_type")
	HttpResponseStatusCodeAttribute        = attribute.Key("http.response.status_code")
)

// Network attributes
const (
	NetworkProtocolNameAttribute = attribute.Key("network.protocol.name")
)

// Server attributes
const (
	ServerAddressAttribute = attribute.Key("server.address")
)

// URL attributes
const (
	UrlFullAttribute      = attribute.Key("url.full")
	UrlSchemeAttribute    = attribute.Key("url.scheme")
	UrlUriSchemeAttribute = attribute.Key("url.uri_scheme")
)
