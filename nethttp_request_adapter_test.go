package nethttplibrary

import (
	nethttp "net/http"
	httptest "net/http/httptest"
	"net/url"
	"testing"

	abs "github.com/microsoft/kiota-abstractions-go"
	absauth "github.com/microsoft/kiota-abstractions-go/authentication"

	"github.com/stretchr/testify/assert"
)

func TestItRetriesOnCAEResponse(t *testing.T) {
	methodCallCount := 0

	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		if methodCallCount > 0 {
			res.WriteHeader(200)
		} else {
			res.Header().Set("WWW-Authenticate", "Bearer realm=\"\", authorization_uri=\"https://login.microsoftonline.com/common/oauth2/authorize\", client_id=\"00000003-0000-0000-c000-000000000000\", error=\"insufficient_claims\", claims=\"eyJhY2Nlc3NfdG9rZW4iOnsibmJmIjp7ImVzc2VudGlhbCI6dHJ1ZSwgInZhbHVlIjoiMTY1MjgxMzUwOCJ9fX0=\"")
			res.WriteHeader(401)
		}
		methodCallCount++
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()
	authProvider := &absauth.AnonymousAuthenticationProvider{}
	adapter, err := NewNetHttpRequestAdapter(authProvider)
	assert.Nil(t, err)
	assert.NotNil(t, adapter)

	uri, err := url.Parse(testServer.URL)
	assert.Nil(t, err)
	assert.NotNil(t, uri)
	request := abs.NewRequestInformation()
	request.SetUri(*uri)
	request.Method = abs.GET

	err2 := adapter.SendNoContentAsync(request, nil, nil)
	assert.Nil(t, err2)
	assert.Equal(t, 2, methodCallCount)
}

func TestImplementationHonoursInterface(t *testing.T) {
	authProvider := &absauth.AnonymousAuthenticationProvider{}
	adapter, err := NewNetHttpRequestAdapter(authProvider)
	assert.Nil(t, err)
	assert.NotNil(t, adapter)

	assert.Implements(t, (*abs.RequestAdapter)(nil), adapter)
}
