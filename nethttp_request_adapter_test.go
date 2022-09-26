package nethttplibrary

import (
	"context"
	nethttp "net/http"
	httptest "net/http/httptest"
	"net/url"
	"testing"

	abs "github.com/microsoft/kiota-abstractions-go"
	absauth "github.com/microsoft/kiota-abstractions-go/authentication"
	"github.com/microsoft/kiota-http-go/internal"

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

	err2 := adapter.SendNoContentAsync(context.TODO(), request, nil)
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

func TestItDoesntFailOnEmptyContentType(t *testing.T) {
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.WriteHeader(201)
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

	res, err := adapter.SendAsync(context.Background(), request, nil, nil)
	assert.Nil(t, err)
	assert.Nil(t, res)
}

func TestItReturnsUsableStreamOnStream(t *testing.T) {
	statusCodes := []int{200, 201, 202, 203, 206}

	for i := 0; i < len(statusCodes); i++ {

		testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
			res.WriteHeader(statusCodes[i])
			res.Write([]byte("test"))
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

		res, err2 := adapter.SendPrimitiveAsync(context.TODO(), request, "[]byte", nil)
		assert.Nil(t, err2)
		assert.NotNil(t, res)
		assert.Equal(t, 4, len(res.([]byte)))
	}
}

func TestItReturnsNilOnStream(t *testing.T) {
	statusCodes := []int{200, 201, 202, 203, 204}

	for i := 0; i < len(statusCodes); i++ {

		testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
			res.WriteHeader(statusCodes[i])
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

		res, err2 := adapter.SendPrimitiveAsync(context.TODO(), request, "[]byte", nil)
		assert.Nil(t, err2)
		assert.Nil(t, res)
	}
}

func TestSendNoContentDoesntFailOnOtherCodes(t *testing.T) {
	statusCodes := []int{200, 201, 202, 203, 204, 206}

	for i := 0; i < len(statusCodes); i++ {

		testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
			res.WriteHeader(statusCodes[i])
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

		err2 := adapter.SendNoContentAsync(context.TODO(), request, nil)
		assert.Nil(t, err2)
	}
}

func TestSendReturnNilOnNoContent(t *testing.T) {
	statusCodes := []int{200, 201, 202, 203, 204, 205}

	for i := 0; i < len(statusCodes); i++ {

		testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
			res.WriteHeader(statusCodes[i])
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

		res, err2 := adapter.SendAsync(context.TODO(), request, internal.MockEntityFactory, nil)
		assert.Nil(t, err2)
		assert.Nil(t, res)
	}
}

func TestSendReturnsObjectOnContent(t *testing.T) {
	statusCodes := []int{200, 201, 202, 203, 204, 205}

	for i := 0; i < len(statusCodes); i++ {

		testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
			res.WriteHeader(statusCodes[i])
		}))
		defer func() { testServer.Close() }()
		authProvider := &absauth.AnonymousAuthenticationProvider{}
		adapter, err := NewNetHttpRequestAdapterWithParseNodeFactory(authProvider, &internal.MockParseNodeFactory{})
		assert.Nil(t, err)
		assert.NotNil(t, adapter)

		uri, err := url.Parse(testServer.URL)
		assert.Nil(t, err)
		assert.NotNil(t, uri)
		request := abs.NewRequestInformation()
		request.SetUri(*uri)
		request.Method = abs.GET

		res, err2 := adapter.SendAsync(context.TODO(), request, internal.MockEntityFactory, nil)
		assert.Nil(t, err2)
		assert.Nil(t, res)
	}
}

func TestResponseHandlerIsCalledWhenProvided(t *testing.T) {
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.WriteHeader(201)
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

	count := 1
	responseHandler := func(response interface{}, errorMappings abs.ErrorMappings) (interface{}, error) {
		count = 2
		return nil, nil
	}

	handlerOption := abs.NewRequestHandlerOption()
	handlerOption.SetResponseHandler(responseHandler)

	request.AddRequestOptions([]abs.RequestOption{handlerOption})

	err = adapter.SendNoContentAsync(context.Background(), request, nil)
	assert.Nil(t, err)
	assert.Equal(t, 2, count)
}
