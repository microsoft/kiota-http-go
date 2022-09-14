// Package nethttplibrary implements the Kiota abstractions with net/http to execute the requests.
// It also provides a middleware infrastructure with some default middleware handlers like the retry handler and the redirect handler.
package nethttplibrary

import (
	nethttp "net/http"
	"net/url"
	"time"
)

// GetDefaultClientWithProxySettings creates a new default net/http client with a proxy url and default middleware
func GetDefaultClientWithProxySettings(proxyUrlStr string) (*nethttp.Client, error) {
	client := getDefaultClientWithoutMiddleware()

	transport, err := getTransportWithProxy(proxyUrlStr, nil)
	if err != nil {
		return nil, err
	}
	client.Transport = transport
	return client, nil
}

// GetDefaultClientWithAuthenticatedProxySettings creates a new default net/http client with a proxy url and default middleware
func GetDefaultClientWithAuthenticatedProxySettings(proxyUrlStr string, username string, password string) (*nethttp.Client, error) {
	client := getDefaultClientWithoutMiddleware()

	user := url.UserPassword(username, password)
	transport, err := getTransportWithProxy(proxyUrlStr, user)
	if err != nil {
		return nil, err
	}
	client.Transport = transport
	return client, nil
}

func getTransportWithProxy(proxyUrlStr string, user *url.Userinfo) (nethttp.RoundTripper, error) {
	proxyURL, err := url.Parse(proxyUrlStr)
	if err != nil {
		return nil, err
	}

	if user != nil {
		proxyURL.User = user
	}

	transport := &nethttp.Transport{
		Proxy: nethttp.ProxyURL(proxyURL),
	}

	middlewares := GetDefaultMiddlewares()

	return NewCustomTransportWithParentTransport(transport, middlewares...), nil
}

// GetDefaultClient creates a new default net/http client with the options configured for the Kiota request adapter
func GetDefaultClient(middleware ...Middleware) *nethttp.Client {
	client := getDefaultClientWithoutMiddleware()
	client.Transport = NewCustomTransport(middleware...)
	return client
}

// used for internal unit testing
func getDefaultClientWithoutMiddleware() *nethttp.Client {
	// the default client doesn't come with any other settings than making a new one does, and using the default client impacts behavior for non-kiota requests
	return &nethttp.Client{
		CheckRedirect: func(req *nethttp.Request, via []*nethttp.Request) error {
			return nethttp.ErrUseLastResponse
		},
		Timeout: time.Second * 30,
	}
}

// GetDefaultMiddlewares creates a new default set of middlewares for the Kiota request adapter
func GetDefaultMiddlewares() []Middleware {
	return []Middleware{
		NewRetryHandler(),
		NewRedirectHandler(),
		NewCompressionHandler(),
		NewParametersNameDecodingHandler(),
		//TODO add additional middlewares
	}
}
