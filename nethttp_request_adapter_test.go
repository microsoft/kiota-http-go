package nethttplibrary

import (
	"context"
	"fmt"
	"github.com/microsoft/kiota-abstractions-go/serialization"
	nethttp "net/http"
	httptest "net/http/httptest"
	"net/url"
	"testing"

	abs "github.com/microsoft/kiota-abstractions-go"
	absauth "github.com/microsoft/kiota-abstractions-go/authentication"
	absstore "github.com/microsoft/kiota-abstractions-go/store"
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

	err2 := adapter.SendNoContent(context.TODO(), request, nil)
	assert.Nil(t, err2)
	assert.Equal(t, 2, methodCallCount)
}

func TestItThrowsApiError(t *testing.T) {
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.Header().Set("client-request-id", "example-guid")
		res.WriteHeader(500)
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

	err2 := adapter.SendNoContent(context.TODO(), request, nil)
	assert.NotNil(t, err2)
	apiError, ok := err2.(*abs.ApiError)
	if !ok {
		t.Fail()
	}
	assert.Equal(t, 500, apiError.ResponseStatusCode)
	assert.Equal(t, "example-guid", apiError.ResponseHeaders.Get("client-request-id")[0])
}

func TestGenericError(t *testing.T) {
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.WriteHeader(500)
		res.Write([]byte("test"))
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

	result := 0
	errorMapping := abs.ErrorMappings{
		"XXX": func(parseNode serialization.ParseNode) (serialization.Parsable, error) {
			result++
			return nil, &abs.ApiError{
				Message: "test XXX message",
			}
		},
	}

	_, err2 := adapter.SendPrimitive(context.TODO(), request, "[]byte", errorMapping)
	assert.NotNil(t, err2)
	assert.Equal(t, 1, result)
	assert.Equal(t, "test XXX message", err2.Error())
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

	res, err := adapter.Send(context.Background(), request, nil, nil)
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

		res, err2 := adapter.SendPrimitive(context.TODO(), request, "[]byte", nil)
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

		res, err2 := adapter.SendPrimitive(context.TODO(), request, "[]byte", nil)
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

		err2 := adapter.SendNoContent(context.TODO(), request, nil)
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

		res, err2 := adapter.Send(context.TODO(), request, internal.MockEntityFactory, nil)
		assert.Nil(t, err2)
		assert.Nil(t, res)
	}
}

func TestSendReturnErrOnNoContent(t *testing.T) {
	// Subset of status codes this applies to since there's many of them. This
	// could be switched to ranges if full coverage is desired.
	statusCodes := []int{nethttp.StatusBadRequest, nethttp.StatusInternalServerError}

	for _, code := range statusCodes {
		testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
			res.WriteHeader(code)
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

		res, err2 := adapter.Send(context.TODO(), request, internal.MockEntityFactory, nil)
		assert.Error(t, err2)
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

		res, err2 := adapter.Send(context.TODO(), request, internal.MockEntityFactory, nil)
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

	err = adapter.SendNoContent(context.Background(), request, nil)
	assert.Nil(t, err)
	assert.Equal(t, 2, count)
}

func TestNonFinalResponseHandlerIsCalled(t *testing.T) {
	statusCodes := []int{200, 201, 202, 203, 204, 205}

	for i := 0; i < len(statusCodes); i++ {

		testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
			res.WriteHeader(statusCodes[i])
			fmt.Fprint(res, `{}`)
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

		count := 1
		responseHandler := func(response interface{}, errorMappings abs.ErrorMappings) (interface{}, error) {
			count = 2
			return nil, nil
		}
		processHandler := NewProcessHandler(responseHandler, false)
		request.AddRequestOptions([]abs.RequestOption{processHandler})

		res, err2 := adapter.Send(context.TODO(), request, internal.MockEntityFactory, nil)
		assert.Nil(t, err2)
		assert.Nil(t, res)
		assert.Equal(t, 2, count)
	}
}

func TestNetHttpRequestAdapter_EnableBackingStore(t *testing.T) {
	authProvider := &absauth.AnonymousAuthenticationProvider{}
	adapter, err := NewNetHttpRequestAdapter(authProvider)
	assert.NoError(t, err)

	var store = func() absstore.BackingStore {
		return nil
	}

	assert.NotEqual(t, absstore.BackingStoreFactoryInstance(), store())
	adapter.EnableBackingStore(store)
	assert.Equal(t, absstore.BackingStoreFactoryInstance(), store())
}
