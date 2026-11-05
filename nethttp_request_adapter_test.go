package nethttplibrary

import (
	"context"
	"github.com/microsoft/kiota-abstractions-go/serialization"
	"go.opentelemetry.io/otel"
	nethttp "net/http"
	httptest "net/http/httptest"
	"net/url"
	"testing"

	abs "github.com/microsoft/kiota-abstractions-go"
	absauth "github.com/microsoft/kiota-abstractions-go/authentication"
	absstore "github.com/microsoft/kiota-abstractions-go/store"
	"github.com/microsoft/kiota-http-go/internal"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendWithResponseContentTypeWhitespace(t *testing.T) {
	for _, contentType := range []string{
		"application/json",
		"application/json;charset=utf-8",
		"Application/JSON; charset=utf-8",
		"application/json ; charset=utf-8",
		"application/json\t; charset=utf-8",
		"Application/JSON \t; charset=utf-8",
		"application/problem+json \t; charset=utf-8",
	} {
		t.Run(contentType, func(t *testing.T) {
			server := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
				res.Header().Set("Content-Type", contentType)
				_, _ = res.Write([]byte(`{}`))
			}))
			defer server.Close()

			registry := serialization.NewParseNodeFactoryRegistry()
			registry.ContentTypeAssociatedFactories["application/json"] = &internal.MockParseNodeFactory{}
			adapter, err := NewNetHttpRequestAdapterWithParseNodeFactory(&absauth.AnonymousAuthenticationProvider{}, registry)
			require.NoError(t, err)
			uri, err := url.Parse(server.URL)
			require.NoError(t, err)
			request := abs.NewRequestInformation()
			request.SetUri(*uri)
			request.Method = abs.GET

			result, err := adapter.Send(context.Background(), request, nil, nil)
			require.NoError(t, err)
			assert.NotNil(t, result)
		})
	}
}

func TestResponsePrimaryContentTypeWithEmptyHeaders(t *testing.T) {
	adapter := &NetHttpRequestAdapter{}
	assert.Empty(t, adapter.getResponsePrimaryContentType(&nethttp.Response{}))
	assert.Empty(t, adapter.getResponsePrimaryContentType(&nethttp.Response{Header: nethttp.Header{}}))
}

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

func TestNetHttpRequestAdapterAccessors(t *testing.T) {
	authProvider := &absauth.AnonymousAuthenticationProvider{}
	adapter, err := NewNetHttpRequestAdapter(authProvider)
	assert.NoError(t, err)
	assert.NotNil(t, adapter)

	// Verify getters return expected instances
	assert.NotNil(t, adapter.GetParseNodeFactory())
	assert.NotNil(t, adapter.GetSerializationWriterFactory())
	assert.Equal(t, "", adapter.GetBaseUrl())

	adapter.SetBaseUrl("https://graph.microsoft.com/v1.0")
	assert.Equal(t, "https://graph.microsoft.com/v1.0", adapter.GetBaseUrl())
}

func TestNetHttpRequestAdapterNilReceiverSafety(t *testing.T) {
	var nilAdapter *NetHttpRequestAdapter

	// Accessors on nil adapter must be safe and not panic
	assert.Nil(t, nilAdapter.GetParseNodeFactory())
	assert.Nil(t, nilAdapter.GetSerializationWriterFactory())
	assert.Equal(t, "", nilAdapter.GetBaseUrl())

	// Mutators on nil adapter should be safe no-ops
	nilAdapter.SetBaseUrl("https://example.com")
	nilAdapter.EnableBackingStore(nil)
}

func TestNetHttpRequestAdapterCustomFactoriesAndBackingStore(t *testing.T) {
	authProvider := &absauth.AnonymousAuthenticationProvider{}
	customClient := &nethttp.Client{}
	customParseNodeFactory := serialization.DefaultParseNodeFactoryInstance
	customWriterFactory := serialization.DefaultSerializationWriterFactoryInstance

	adapter, err := NewNetHttpRequestAdapterWithParseNodeFactoryAndSerializationWriterFactoryAndHttpClient(
		authProvider,
		customParseNodeFactory,
		customWriterFactory,
		customClient,
	)
	assert.NoError(t, err)
	assert.NotNil(t, adapter)

	assert.Equal(t, customParseNodeFactory, adapter.GetParseNodeFactory())
	assert.Equal(t, customWriterFactory, adapter.GetSerializationWriterFactory())

	// Enable backing store and verify GetParseNodeFactory returns wrapped factory
	adapter.EnableBackingStore(nil)
	assert.NotNil(t, adapter.GetParseNodeFactory())
	assert.NotNil(t, adapter.GetSerializationWriterFactory())
}

func TestNetHttpRequestAdapter_SpanAttributes_ServerAddressAndUrlScheme(t *testing.T) {
	tracerProvider := newRecordingTracerProvider()
	prevProvider := otel.GetTracerProvider()
	otel.SetTracerProvider(tracerProvider)
	defer otel.SetTracerProvider(prevProvider)

	authProvider := &absauth.AnonymousAuthenticationProvider{}
	adapter, err := NewNetHttpRequestAdapter(authProvider)
	require.NoError(t, err)

	uri, err := url.Parse("https://graph.microsoft.com/v1.0/users/test-user")
	require.NoError(t, err)
	request := abs.NewRequestInformation()
	request.SetUri(*uri)
	request.Method = abs.GET

	nativeReq, err := adapter.ConvertToNativeRequest(context.Background(), request)
	require.NoError(t, err)
	assert.NotNil(t, nativeReq)

	tracer, ok := tracerProvider.tracers[adapter.observabilityOptions.GetTracerInstrumentationName()]
	require.True(t, ok, "tracer should be created")
	require.NotEmpty(t, tracer.spans, "spans should be recorded")

	var targetSpan *recordingSpan
	for _, s := range tracer.spans {
		if s.name == "getRequestFromRequestInformation" {
			targetSpan = s
			break
		}
	}
	require.NotNil(t, targetSpan, "getRequestFromRequestInformation span should exist")

	var serverAddress, urlScheme string
	for _, attr := range targetSpan.attributes {
		if attr.Key == "server.address" {
			serverAddress = attr.Value.AsString()
		}
		if attr.Key == "url.scheme" {
			urlScheme = attr.Value.AsString()
		}
	}

	assert.Equal(t, "graph.microsoft.com", serverAddress, "server.address must be the target host")
	assert.Equal(t, "https", urlScheme, "url.scheme must be the URI scheme")
}


func TestNetHttpRequestAdapter_SpanAttributes_VariedEndpoints(t *testing.T) {
	testCases := []struct {
		name            string
		rawURL          string
		expectedHost    string
		expectedScheme  string
	}{
		{
			name:           "custom port http",
			rawURL:         "http://localhost:8080/api/items",
			expectedHost:   "localhost",
			expectedScheme: "http",
		},
		{
			name:           "custom port https",
			rawURL:         "https://api.contoso.com:8443/v1/resource",
			expectedHost:   "api.contoso.com",
			expectedScheme: "https",
		},
		{
			name:           "ipv4 address endpoint",
			rawURL:         "http://127.0.0.1:5000/healthz",
			expectedHost:   "127.0.0.1",
			expectedScheme: "http",
		},
		{
			name:           "subdomain https",
			rawURL:         "https://api.subdomain.example.com",
			expectedHost:   "api.subdomain.example.com",
			expectedScheme: "https",
		},
		{
			name:           "ipv6 endpoint with custom port",
			rawURL:         "http://[::1]:8080/healthz",
			expectedHost:   "::1",
			expectedScheme: "http",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tracerProvider := newRecordingTracerProvider()
			prevProvider := otel.GetTracerProvider()
			otel.SetTracerProvider(tracerProvider)
			defer otel.SetTracerProvider(prevProvider)

			authProvider := &absauth.AnonymousAuthenticationProvider{}
			adapter, err := NewNetHttpRequestAdapter(authProvider)
			require.NoError(t, err)

			uri, err := url.Parse(tc.rawURL)
			require.NoError(t, err)
			request := abs.NewRequestInformation()
			request.SetUri(*uri)
			request.Method = abs.GET

			_, err = adapter.ConvertToNativeRequest(context.Background(), request)
			require.NoError(t, err)

			tracer, ok := tracerProvider.tracers[adapter.observabilityOptions.GetTracerInstrumentationName()]
			require.True(t, ok)
			require.NotEmpty(t, tracer.spans)

			var targetSpan *recordingSpan
			for _, s := range tracer.spans {
				if s.name == "getRequestFromRequestInformation" {
					targetSpan = s
					break
				}
			}
			require.NotNil(t, targetSpan)

			var serverAddress, urlScheme string
			for _, attr := range targetSpan.attributes {
				if attr.Key == "server.address" {
					serverAddress = attr.Value.AsString()
				}
				if attr.Key == "url.scheme" {
					urlScheme = attr.Value.AsString()
				}
			}

			assert.Equal(t, tc.expectedHost, serverAddress)
			assert.Equal(t, tc.expectedScheme, urlScheme)
		})
	}
}

func TestNetHttpRequestAdapter_SetBaseUrlForRequestInformation_NilSafe(t *testing.T) {
	adapter := &NetHttpRequestAdapter{
		baseUrl: "https://example.com",
	}

	// 1. requestInfo is nil should be a safe no-op
	assert.NotPanics(t, func() {
		adapter.setBaseUrlForRequestInformation(nil)
	})

	// 2. requestInfo with nil PathParameters should be initialized without panicking
	reqInfo := &abs.RequestInformation{}
	assert.Nil(t, reqInfo.PathParameters)
	assert.NotPanics(t, func() {
		adapter.setBaseUrlForRequestInformation(reqInfo)
	})
	assert.NotNil(t, reqInfo.PathParameters)
	assert.Equal(t, "https://example.com", reqInfo.PathParameters["baseurl"])

	// 3. nil adapter should also not panic and set empty baseurl
	var nilAdapter *NetHttpRequestAdapter
	reqInfo2 := &abs.RequestInformation{}
	assert.NotPanics(t, func() {
		nilAdapter.setBaseUrlForRequestInformation(reqInfo2)
	})
	assert.Equal(t, "", reqInfo2.PathParameters["baseurl"])
}

func TestSendEnum_ResponseHandler_ReturnsRawResult(t *testing.T) {
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.WriteHeader(200)
	}))
	defer testServer.Close()

	authProvider := &absauth.AnonymousAuthenticationProvider{}
	adapter, err := NewNetHttpRequestAdapter(authProvider)
	assert.NoError(t, err)

	uri, err := url.Parse(testServer.URL)
	assert.NoError(t, err)
	request := abs.NewRequestInformation()
	request.SetUri(*uri)
	request.Method = abs.GET

	responseHandler := func(response interface{}, errorMappings abs.ErrorMappings) (interface{}, error) {
		return "CustomEnumValue", nil
	}
	handlerOption := abs.NewRequestHandlerOption()
	handlerOption.SetResponseHandler(responseHandler)
	request.AddRequestOptions([]abs.RequestOption{handlerOption})

	result, err := adapter.SendEnum(context.Background(), request, nil, nil)
	assert.NoError(t, err)
	assert.Equal(t, "CustomEnumValue", result)
}

func TestSendPrimitive_ResponseHandler_ReturnsRawResult(t *testing.T) {
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.WriteHeader(200)
	}))
	defer testServer.Close()

	authProvider := &absauth.AnonymousAuthenticationProvider{}
	adapter, err := NewNetHttpRequestAdapter(authProvider)
	assert.NoError(t, err)

	uri, err := url.Parse(testServer.URL)
	assert.NoError(t, err)
	request := abs.NewRequestInformation()
	request.SetUri(*uri)
	request.Method = abs.GET

	responseHandler := func(response interface{}, errorMappings abs.ErrorMappings) (interface{}, error) {
		return 42, nil
	}
	handlerOption := abs.NewRequestHandlerOption()
	handlerOption.SetResponseHandler(responseHandler)
	request.AddRequestOptions([]abs.RequestOption{handlerOption})

	result, err := adapter.SendPrimitive(context.Background(), request, "int", nil)
	assert.NoError(t, err)
	assert.Equal(t, 42, result)
}

func TestConvertToNativeRequest_NilGuardsAndBaseUrl(t *testing.T) {
	ctx := context.Background()

	// 1. nil adapter returns error
	var nilAdapter *NetHttpRequestAdapter
	reqInfo := abs.NewRequestInformation()
	req, err := nilAdapter.ConvertToNativeRequest(ctx, reqInfo)
	assert.Error(t, err)
	assert.Nil(t, req)
	assert.Contains(t, err.Error(), "adapter cannot be nil")

	// 2. nil requestInfo returns error
	adapter := &NetHttpRequestAdapter{
		authenticationProvider: &absauth.AnonymousAuthenticationProvider{},
	}
	req, err = adapter.ConvertToNativeRequest(ctx, nil)
	assert.Error(t, err)
	assert.Nil(t, req)
	assert.Contains(t, err.Error(), "requestInfo cannot be nil")

	// 3. nil authenticationProvider returns error
	adapterNoAuth := &NetHttpRequestAdapter{}
	req, err = adapterNoAuth.ConvertToNativeRequest(ctx, reqInfo)
	assert.Error(t, err)
	assert.Nil(t, req)
	assert.Contains(t, err.Error(), "authenticationProvider cannot be nil")

	// 4. sets base url on requestInfo
	adapterWithBase := &NetHttpRequestAdapter{
		authenticationProvider: &absauth.AnonymousAuthenticationProvider{},
		baseUrl:                "https://example.com/v1.0",
	}
	reqWithTemplate := abs.NewRequestInformation()
	reqWithTemplate.UrlTemplate = "{+baseurl}/me"
	reqWithTemplate.Method = abs.GET
	nativeReq, err := adapterWithBase.ConvertToNativeRequest(ctx, reqWithTemplate)
	assert.NoError(t, err)
	assert.NotNil(t, nativeReq)
	assert.Equal(t, "https://example.com/v1.0", reqWithTemplate.PathParameters["baseurl"])
}

func TestNetHttpRequestAdapter_PurgeAndResponseNilGuards(t *testing.T) {
	adapter := &NetHttpRequestAdapter{}

	// 1. purge with nil response or nil body does not panic
	assert.NotPanics(t, func() {
		err := adapter.purge(nil)
		assert.NoError(t, err)
	})
	assert.NotPanics(t, func() {
		err := adapter.purge(&nethttp.Response{Body: nil})
		assert.NoError(t, err)
	})

	// 2. shouldReturnNil with nil response returns false
	assert.False(t, adapter.shouldReturnNil(nil))
	assert.True(t, adapter.shouldReturnNil(&nethttp.Response{StatusCode: 204}))
	assert.False(t, adapter.shouldReturnNil(&nethttp.Response{StatusCode: 200}))

	// 3. getResponsePrimaryContentType with nil response returns ""
	assert.Equal(t, "", adapter.getResponsePrimaryContentType(nil))
	assert.Equal(t, "", adapter.getResponsePrimaryContentType(&nethttp.Response{Header: nil}))

	// 4. getRootParseNode with nil response or nil body returns nil without panic
	node, _, err := adapter.getRootParseNode(context.Background(), nil, nil)
	assert.NoError(t, err)
	assert.Nil(t, node)

	node, _, err = adapter.getRootParseNode(context.Background(), &nethttp.Response{Body: nil}, nil)
	assert.NoError(t, err)
	assert.Nil(t, node)
}
