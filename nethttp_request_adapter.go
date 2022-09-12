package nethttplibrary

import (
	"bytes"
	"context"
	"errors"
	"io/ioutil"
	nethttp "net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	abs "github.com/microsoft/kiota-abstractions-go"
	absauth "github.com/microsoft/kiota-abstractions-go/authentication"
	absser "github.com/microsoft/kiota-abstractions-go/serialization"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// NetHttpRequestAdapter implements the RequestAdapter interface using net/http
type NetHttpRequestAdapter struct {
	// serializationWriterFactory is the factory used to create serialization writers
	serializationWriterFactory absser.SerializationWriterFactory
	// parseNodeFactory is the factory used to create parse nodes
	parseNodeFactory absser.ParseNodeFactory
	// httpClient is the client used to send requests
	httpClient *nethttp.Client
	// authenticationProvider is the provider used to authenticate requests
	authenticationProvider absauth.AuthenticationProvider
	// The base url for every request.
	baseUrl string
	// The observation name for the request adapter.
	observabilityName string
}

// NewNetHttpRequestAdapter creates a new NetHttpRequestAdapter with the given parameters
func NewNetHttpRequestAdapter(authenticationProvider absauth.AuthenticationProvider) (*NetHttpRequestAdapter, error) {
	return NewNetHttpRequestAdapterWithParseNodeFactory(authenticationProvider, nil)
}

// NewNetHttpRequestAdapterWithParseNodeFactory creates a new NetHttpRequestAdapter with the given parameters
func NewNetHttpRequestAdapterWithParseNodeFactory(authenticationProvider absauth.AuthenticationProvider, parseNodeFactory absser.ParseNodeFactory) (*NetHttpRequestAdapter, error) {
	return NewNetHttpRequestAdapterWithParseNodeFactoryAndSerializationWriterFactory(authenticationProvider, parseNodeFactory, nil)
}

// NewNetHttpRequestAdapterWithParseNodeFactoryAndSerializationWriterFactory creates a new NetHttpRequestAdapter with the given parameters
func NewNetHttpRequestAdapterWithParseNodeFactoryAndSerializationWriterFactory(authenticationProvider absauth.AuthenticationProvider, parseNodeFactory absser.ParseNodeFactory, serializationWriterFactory absser.SerializationWriterFactory) (*NetHttpRequestAdapter, error) {
	return NewNetHttpRequestAdapterWithParseNodeFactoryAndSerializationWriterFactoryAndHttpClient(authenticationProvider, parseNodeFactory, serializationWriterFactory, nil)
}

// NewNetHttpRequestAdapterWithParseNodeFactoryAndSerializationWriterFactoryAndHttpClient creates a new NetHttpRequestAdapter with the given parameters
func NewNetHttpRequestAdapterWithParseNodeFactoryAndSerializationWriterFactoryAndHttpClient(authenticationProvider absauth.AuthenticationProvider, parseNodeFactory absser.ParseNodeFactory, serializationWriterFactory absser.SerializationWriterFactory, httpClient *nethttp.Client) (*NetHttpRequestAdapter, error) {
	return NewNetHttpRequestAdapterWithParseNodeFactoryAndSerializationWriterFactoryAndHttpClientAndObservabilityName(authenticationProvider, parseNodeFactory, serializationWriterFactory, httpClient, "")
}

var DefaultObservationName = "kiota-http-go"

// NewNetHttpRequestAdapterWithParseNodeFactoryAndSerializationWriterFactoryAndHttpClientAndObservabilityName creates a new NetHttpRequestAdapter with the given parameters
func NewNetHttpRequestAdapterWithParseNodeFactoryAndSerializationWriterFactoryAndHttpClientAndObservabilityName(authenticationProvider absauth.AuthenticationProvider, parseNodeFactory absser.ParseNodeFactory, serializationWriterFactory absser.SerializationWriterFactory, httpClient *nethttp.Client, observabilityName string) (*NetHttpRequestAdapter, error) {
	if observabilityName == "" {
		observabilityName = DefaultObservationName
	}
	if authenticationProvider == nil {
		return nil, errors.New("authenticationProvider cannot be nil")
	}
	result := &NetHttpRequestAdapter{
		serializationWriterFactory: serializationWriterFactory,
		parseNodeFactory:           parseNodeFactory,
		httpClient:                 httpClient,
		authenticationProvider:     authenticationProvider,
		baseUrl:                    "",
		observabilityName:          observabilityName,
	}
	if result.httpClient == nil {
		defaultClient := GetDefaultClient()
		result.httpClient = defaultClient
	}
	if result.serializationWriterFactory == nil {
		result.serializationWriterFactory = absser.DefaultSerializationWriterFactoryInstance
	}
	if result.parseNodeFactory == nil {
		result.parseNodeFactory = absser.DefaultParseNodeFactoryInstance
	}
	return result, nil
}

// GetSerializationWriterFactory returns the serialization writer factory currently in use for the request adapter service.
func (a *NetHttpRequestAdapter) GetSerializationWriterFactory() absser.SerializationWriterFactory {
	return a.serializationWriterFactory
}

// EnableBackingStore enables the backing store proxies for the SerializationWriters and ParseNodes in use.
func (a *NetHttpRequestAdapter) EnableBackingStore() {
	//TODO implement when backing store is available for go
}

// SetBaseUrl sets the base url for every request.
func (a *NetHttpRequestAdapter) SetBaseUrl(baseUrl string) {
	a.baseUrl = baseUrl
}

// GetBaseUrl gets the base url for every request.
func (a *NetHttpRequestAdapter) GetBaseUrl() string {
	return a.baseUrl
}

func (a *NetHttpRequestAdapter) getHttpResponseMessage(ctx context.Context, requestInfo *abs.RequestInformation, claims string, spanForAttributes trace.Span) (*nethttp.Response, error) {
	ctx, span := otel.GetTracerProvider().Tracer(a.observabilityName).Start(ctx, "getHttpResponseMessage")
	defer span.End()
	if ctx == nil {
		ctx = context.Background()
	}
	a.setBaseUrlForRequestInformation(requestInfo)
	additionalContext := make(map[string]interface{})
	if claims != "" {
		additionalContext[claimsKey] = claims
	}
	err := a.authenticationProvider.AuthenticateRequest(ctx, requestInfo, additionalContext)
	if err != nil {
		return nil, err
	}
	request, err := a.getRequestFromRequestInformation(ctx, requestInfo, spanForAttributes)
	if err != nil {
		return nil, err
	}
	response, err := (*a.httpClient).Do(request)
	if err != nil {
		return nil, err
	}
	if response != nil {
		contentLenHeader := response.Header.Get("Content-Length")
		if contentLenHeader != "" {
			contentLen, _ := strconv.Atoi(contentLenHeader)
			spanForAttributes.SetAttributes(attribute.Int("http.response_content_length", contentLen))
		}
		spanForAttributes.SetAttributes(
			attribute.Int("http.status_code", response.StatusCode),
			attribute.String("http.flavor", response.Proto),
		)
	}
	return a.retryCAEResponseIfRequired(ctx, response, requestInfo, claims, spanForAttributes)
}

const claimsKey = "claims"

var reBearer = regexp.MustCompile(`(?i)^Bearer\s`)
var reClaims = regexp.MustCompile(`\"([^\"]*)\"`)
var AuthenticateChallengedEventKey = "authenticate_challenge_received"

func (a *NetHttpRequestAdapter) retryCAEResponseIfRequired(ctx context.Context, response *nethttp.Response, requestInfo *abs.RequestInformation, claims string, spanForAttributes trace.Span) (*nethttp.Response, error) {
	ctx, span := otel.GetTracerProvider().Tracer(a.observabilityName).Start(ctx, "retryCAEResponseIfRequired")
	defer span.End()
	if response.StatusCode == 401 &&
		claims == "" { //avoid infinite loop, we only retry once
		authenticateHeaderVal := response.Header.Get("WWW-Authenticate")
		if authenticateHeaderVal != "" && reBearer.Match([]byte(authenticateHeaderVal)) {
			span.AddEvent(AuthenticateChallengedEventKey)
			spanForAttributes.SetAttributes(attribute.Int("http.retry_count", 1))
			responseClaims := ""
			parametersRaw := string(reBearer.ReplaceAll([]byte(authenticateHeaderVal), []byte("")))
			parameters := strings.Split(parametersRaw, ",")
			for _, parameter := range parameters {
				if strings.HasPrefix(strings.Trim(parameter, " "), claimsKey) {
					responseClaims = reClaims.FindStringSubmatch(parameter)[1]
					break
				}
			}
			if responseClaims != "" {
				defer a.purge(response)
				return a.getHttpResponseMessage(ctx, requestInfo, responseClaims, spanForAttributes)
			}
		}
	}
	return response, nil
}

func (a *NetHttpRequestAdapter) getResponsePrimaryContentType(response *nethttp.Response) string {
	if response.Header == nil {
		return ""
	}
	rawType := response.Header.Get("Content-Type")
	splat := strings.Split(rawType, ";")
	return strings.ToLower(splat[0])
}

func (a *NetHttpRequestAdapter) setBaseUrlForRequestInformation(requestInfo *abs.RequestInformation) {
	requestInfo.PathParameters["baseurl"] = a.GetBaseUrl()
}

const requestTimeOutInSeconds = 100

func (a *NetHttpRequestAdapter) prepareContext(ctx context.Context, requestInfo *abs.RequestInformation) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	// set deadline if not set in receiving context
	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		ctx, _ = context.WithTimeout(ctx, time.Second*requestTimeOutInSeconds)
	}

	for _, value := range requestInfo.GetRequestOptions() {
		ctx = context.WithValue(ctx, value.GetKey(), value)
	}
	ctx = context.WithValue(ctx, observabilityOptionsKeyValue, &ObservabilityOptions{
		observabilityName: a.observabilityName,
	})
	return ctx
}
func (a *NetHttpRequestAdapter) getRequestFromRequestInformation(ctx context.Context, requestInfo *abs.RequestInformation, spanForAttributes trace.Span) (*nethttp.Request, error) {
	ctx, span := otel.GetTracerProvider().Tracer(a.observabilityName).Start(ctx, "getRequestFromRequestInformation")
	defer span.End()
	spanForAttributes.SetAttributes(attribute.String("http.method", requestInfo.Method.String()))
	uri, err := requestInfo.GetUri()
	if err != nil {
		return nil, err
	}
	spanForAttributes.SetAttributes(
		attribute.String("http.uri", uri.String()),
		attribute.String("http.scheme", uri.Scheme),
		attribute.String("http.host", uri.Host),
		attribute.Int("http.request_content_length", len(requestInfo.Content)),
	)

	request, err := nethttp.NewRequestWithContext(ctx, requestInfo.Method.String(), uri.String(), nil)

	if err != nil {
		return nil, err
	}
	if len(requestInfo.Content) > 0 {
		reader := bytes.NewReader(requestInfo.Content)
		request.Body = ioutil.NopCloser(reader)
	}
	if request.Header == nil {
		request.Header = make(nethttp.Header)
	}
	if requestInfo.Headers != nil {
		for key, value := range requestInfo.Headers {
			request.Header.Set(key, value)
		}
		if requestInfo.Headers["Content-Type"] != "" { //TODO the map is case sensitive and should be normalized
			spanForAttributes.SetAttributes(
				attribute.String("http.request_content_type", requestInfo.Headers["Content-Type"]),
			)
		}
	}

	return request, nil
}

var EventResponseHandlerInvokedKey = "response_handler_invoked"
var queryParametersCleanupRegex = regexp.MustCompile(`\{\?[^\}]+}`)

func (a *NetHttpRequestAdapter) startTracingSpan(ctx context.Context, requestInfo *abs.RequestInformation, methodName string) (context.Context, trace.Span) {
	telemetryPathValue := queryParametersCleanupRegex.ReplaceAll([]byte(requestInfo.UrlTemplate), []byte(""))
	ctx, span := otel.GetTracerProvider().Tracer(a.observabilityName).Start(ctx, methodName+" - "+string(telemetryPathValue))
	return ctx, span
}

// SendAsync executes the HTTP request specified by the given RequestInformation and returns the deserialized response model.
func (a *NetHttpRequestAdapter) SendAsync(ctx context.Context, requestInfo *abs.RequestInformation, constructor absser.ParsableFactory, errorMappings abs.ErrorMappings) (absser.Parsable, error) {
	if requestInfo == nil {
		return nil, errors.New("requestInfo cannot be nil")
	}
	ctx = a.prepareContext(ctx, requestInfo)
	ctx, span := a.startTracingSpan(ctx, requestInfo, "SendAsync")
	defer span.End()
	span.SetAttributes(attribute.String("http.uri_template", requestInfo.UrlTemplate))
	response, err := a.getHttpResponseMessage(ctx, requestInfo, "", span)
	if err != nil {
		return nil, err
	}

	responseHandler := getResponseHandler(ctx)
	if responseHandler != nil {
		span.AddEvent(EventResponseHandlerInvokedKey)
		result, err := responseHandler(response, errorMappings)
		if err != nil {
			return nil, err
		}
		return result.(absser.Parsable), nil
	} else if response != nil {
		defer a.purge(response)
		err = a.throwFailedResponses(ctx, response, errorMappings, span)
		if err != nil {
			return nil, err
		}
		if a.shouldReturnNil(response) {
			return nil, err
		}
		parseNode, _, err := a.getRootParseNode(ctx, response, span)
		if err != nil {
			return nil, err
		}
		if parseNode == nil {
			return nil, nil
		}
		_, deserializeSpan := otel.GetTracerProvider().Tracer(a.observabilityName).Start(ctx, "GetObjectValue")
		defer deserializeSpan.End()
		result, err := parseNode.GetObjectValue(constructor)
		return result, err
	} else {
		return nil, errors.New("response is nil")
	}
}

// SendEnumAsync executes the HTTP request specified by the given RequestInformation and returns the deserialized response model.
func (a *NetHttpRequestAdapter) SendEnumAsync(ctx context.Context, requestInfo *abs.RequestInformation, parser absser.EnumFactory, errorMappings abs.ErrorMappings) (interface{}, error) {
	if requestInfo == nil {
		return nil, errors.New("requestInfo cannot be nil")
	}
	ctx = a.prepareContext(ctx, requestInfo)
	ctx, span := a.startTracingSpan(ctx, requestInfo, "SendEnumAsync")
	defer span.End()
	response, err := a.getHttpResponseMessage(ctx, requestInfo, "", span)
	if err != nil {
		return nil, err
	}

	responseHandler := getResponseHandler(ctx)
	if responseHandler != nil {
		span.AddEvent(EventResponseHandlerInvokedKey)
		result, err := responseHandler(response, errorMappings)
		if err != nil {
			return nil, err
		}
		return result.(absser.Parsable), nil
	} else if response != nil {
		defer a.purge(response)
		err = a.throwFailedResponses(ctx, response, errorMappings, span)
		if err != nil {
			return nil, err
		}
		if a.shouldReturnNil(response) {
			return nil, err
		}
		parseNode, _, err := a.getRootParseNode(ctx, response, span)
		if err != nil {
			return nil, err
		}
		if parseNode == nil {
			return nil, nil
		}
		_, deserializeSpan := otel.GetTracerProvider().Tracer(a.observabilityName).Start(ctx, "GetEnumValue")
		defer deserializeSpan.End()
		result, err := parseNode.GetEnumValue(parser)
		return result, err
	} else {
		return nil, errors.New("response is nil")
	}
}

// SendCollectionAsync executes the HTTP request specified by the given RequestInformation and returns the deserialized response model collection.
func (a *NetHttpRequestAdapter) SendCollectionAsync(ctx context.Context, requestInfo *abs.RequestInformation, constructor absser.ParsableFactory, errorMappings abs.ErrorMappings) ([]absser.Parsable, error) {
	if requestInfo == nil {
		return nil, errors.New("requestInfo cannot be nil")
	}
	ctx = a.prepareContext(ctx, requestInfo)
	ctx, span := a.startTracingSpan(ctx, requestInfo, "SendCollectionAsync")
	defer span.End()
	response, err := a.getHttpResponseMessage(ctx, requestInfo, "", span)
	if err != nil {
		return nil, err
	}

	responseHandler := getResponseHandler(ctx)
	if responseHandler != nil {
		span.AddEvent(EventResponseHandlerInvokedKey)
		result, err := responseHandler(response, errorMappings)
		if err != nil {
			return nil, err
		}
		return result.([]absser.Parsable), nil
	} else if response != nil {
		defer a.purge(response)
		err = a.throwFailedResponses(ctx, response, errorMappings, span)
		if err != nil {
			return nil, err
		}
		if a.shouldReturnNil(response) {
			return nil, err
		}
		parseNode, _, err := a.getRootParseNode(ctx, response, span)
		if err != nil {
			return nil, err
		}
		if parseNode == nil {
			return nil, nil
		}
		_, deserializeSpan := otel.GetTracerProvider().Tracer(a.observabilityName).Start(ctx, "GetCollectionOfObjectValues")
		defer deserializeSpan.End()
		result, err := parseNode.GetCollectionOfObjectValues(constructor)
		return result, err
	} else {
		return nil, errors.New("response is nil")
	}
}

// SendEnumCollectionAsync executes the HTTP request specified by the given RequestInformation and returns the deserialized response model collection.
func (a *NetHttpRequestAdapter) SendEnumCollectionAsync(ctx context.Context, requestInfo *abs.RequestInformation, parser absser.EnumFactory, errorMappings abs.ErrorMappings) ([]interface{}, error) {
	if requestInfo == nil {
		return nil, errors.New("requestInfo cannot be nil")
	}
	ctx = a.prepareContext(ctx, requestInfo)
	ctx, span := a.startTracingSpan(ctx, requestInfo, "SendEnumCollectionAsync")
	defer span.End()
	response, err := a.getHttpResponseMessage(ctx, requestInfo, "", span)
	if err != nil {
		return nil, err
	}

	responseHandler := getResponseHandler(ctx)
	if responseHandler != nil {
		span.AddEvent(EventResponseHandlerInvokedKey)
		result, err := responseHandler(response, errorMappings)
		if err != nil {
			return nil, err
		}
		return result.([]interface{}), nil
	} else if response != nil {
		defer a.purge(response)
		err = a.throwFailedResponses(ctx, response, errorMappings, span)
		if err != nil {
			return nil, err
		}
		if a.shouldReturnNil(response) {
			return nil, err
		}
		parseNode, _, err := a.getRootParseNode(ctx, response, span)
		if err != nil {
			return nil, err
		}
		if parseNode == nil {
			return nil, nil
		}
		_, deserializeSpan := otel.GetTracerProvider().Tracer(a.observabilityName).Start(ctx, "GetCollectionOfEnumValues")
		defer deserializeSpan.End()
		result, err := parseNode.GetCollectionOfEnumValues(parser)
		return result, err
	} else {
		return nil, errors.New("response is nil")
	}
}

func getResponseHandler(ctx context.Context) abs.ResponseHandler {
	var handlerOption = ctx.Value(abs.ResponseHandlerOptionKey)
	if handlerOption != nil {
		return handlerOption.(abs.RequestHandlerOption).GetResponseHandler()
	}
	return nil
}

// SendPrimitiveAsync executes the HTTP request specified by the given RequestInformation and returns the deserialized primitive response model.
func (a *NetHttpRequestAdapter) SendPrimitiveAsync(ctx context.Context, requestInfo *abs.RequestInformation, typeName string, errorMappings abs.ErrorMappings) (interface{}, error) {
	if requestInfo == nil {
		return nil, errors.New("requestInfo cannot be nil")
	}
	ctx = a.prepareContext(ctx, requestInfo)
	ctx, span := a.startTracingSpan(ctx, requestInfo, "SendPrimitiveAsync")
	defer span.End()
	response, err := a.getHttpResponseMessage(ctx, requestInfo, "", span)
	if err != nil {
		return nil, err
	}

	responseHandler := getResponseHandler(ctx)
	if responseHandler != nil {
		span.AddEvent(EventResponseHandlerInvokedKey)
		result, err := responseHandler(response, errorMappings)
		if err != nil {
			return nil, err
		}
		return result.(absser.Parsable), nil
	} else if response != nil {
		defer a.purge(response)
		err = a.throwFailedResponses(ctx, response, errorMappings, span)
		if err != nil {
			return nil, err
		}
		if a.shouldReturnNil(response) {
			return nil, err
		}
		if typeName == "[]byte" {
			res, err := ioutil.ReadAll(response.Body)
			if err != nil {
				return nil, err
			} else if len(res) == 0 {
				return nil, nil
			}
			return res, nil
		}
		parseNode, _, err := a.getRootParseNode(ctx, response, span)
		if err != nil {
			return nil, err
		}
		if parseNode == nil {
			return nil, nil
		}
		_, deserializeSpan := otel.GetTracerProvider().Tracer(a.observabilityName).Start(ctx, "Get"+typeName+"Value")
		defer deserializeSpan.End()
		switch typeName {
		case "string":
			return parseNode.GetStringValue()
		case "float32":
			return parseNode.GetFloat32Value()
		case "float64":
			return parseNode.GetFloat64Value()
		case "int32":
			return parseNode.GetInt32Value()
		case "int64":
			return parseNode.GetInt64Value()
		case "bool":
			return parseNode.GetBoolValue()
		case "Time":
			return parseNode.GetTimeValue()
		case "UUID":
			return parseNode.GetUUIDValue()
		default:
			return nil, errors.New("unsupported type")
		}
	} else {
		return nil, errors.New("response is nil")
	}
}

// SendPrimitiveCollectionAsync executes the HTTP request specified by the given RequestInformation and returns the deserialized primitive response model collection.
func (a *NetHttpRequestAdapter) SendPrimitiveCollectionAsync(ctx context.Context, requestInfo *abs.RequestInformation, typeName string, errorMappings abs.ErrorMappings) ([]interface{}, error) {
	if requestInfo == nil {
		return nil, errors.New("requestInfo cannot be nil")
	}
	ctx = a.prepareContext(ctx, requestInfo)
	ctx, span := a.startTracingSpan(ctx, requestInfo, "SendPrimitiveCollectionAsync")
	defer span.End()
	response, err := a.getHttpResponseMessage(ctx, requestInfo, "", span)
	if err != nil {
		return nil, err
	}

	responseHandler := getResponseHandler(ctx)
	if responseHandler != nil {
		span.AddEvent(EventResponseHandlerInvokedKey)
		result, err := responseHandler(response, errorMappings)
		if err != nil {
			return nil, err
		}
		return result.([]interface{}), nil
	} else if response != nil {
		defer a.purge(response)
		err = a.throwFailedResponses(ctx, response, errorMappings, span)
		if err != nil {
			return nil, err
		}
		if a.shouldReturnNil(response) {
			return nil, err
		}
		parseNode, _, err := a.getRootParseNode(ctx, response, span)
		if err != nil {
			return nil, err
		}
		if parseNode == nil {
			return nil, nil
		}
		_, deserializeSpan := otel.GetTracerProvider().Tracer(a.observabilityName).Start(ctx, "GetCollectionOfPrimitiveValues")
		defer deserializeSpan.End()
		return parseNode.GetCollectionOfPrimitiveValues(typeName)
	} else {
		return nil, errors.New("response is nil")
	}
}

// SendNoContentAsync executes the HTTP request specified by the given RequestInformation with no return content.
func (a *NetHttpRequestAdapter) SendNoContentAsync(ctx context.Context, requestInfo *abs.RequestInformation, errorMappings abs.ErrorMappings) error {
	if requestInfo == nil {
		return errors.New("requestInfo cannot be nil")
	}
	ctx = a.prepareContext(ctx, requestInfo)
	ctx, span := a.startTracingSpan(ctx, requestInfo, "SendNoContentAsync")
	defer span.End()
	response, err := a.getHttpResponseMessage(ctx, requestInfo, "", span)
	if err != nil {
		return err
	}

	responseHandler := getResponseHandler(ctx)
	if responseHandler != nil {
		span.AddEvent(EventResponseHandlerInvokedKey)
		_, err := responseHandler(response, errorMappings)
		return err
	} else if response != nil {
		defer a.purge(response)
		err = a.throwFailedResponses(ctx, response, errorMappings, span)
		if err != nil {
			return err
		}
		return nil
	} else {
		return errors.New("response is nil")
	}
}

func (a *NetHttpRequestAdapter) getRootParseNode(ctx context.Context, response *nethttp.Response, spanForAttributes trace.Span) (absser.ParseNode, context.Context, error) {
	ctx, span := otel.GetTracerProvider().Tracer(a.observabilityName).Start(ctx, "getRootParseNode")
	defer span.End()
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, ctx, err
	}
	contentType := a.getResponsePrimaryContentType(response)
	if contentType == "" {
		return nil, ctx, nil
	}
	spanForAttributes.SetAttributes(attribute.String("http.response_content_type", contentType))
	rootNode, err := a.parseNodeFactory.GetRootParseNode(contentType, body)
	return rootNode, ctx, err
}
func (a *NetHttpRequestAdapter) purge(response *nethttp.Response) error {
	_, _ = ioutil.ReadAll(response.Body) //we don't care about errors comming from reading the body, just trying to purge anything that maybe left
	err := response.Body.Close()
	if err != nil {
		return err
	}
	return nil
}
func (a *NetHttpRequestAdapter) shouldReturnNil(response *nethttp.Response) bool {
	return response.StatusCode == 204
}

var ErrorMappingFoundAttributeName = "error_mapping_found"
var ErrorBodyFoundAttributeName = "error_body_found"

func (a *NetHttpRequestAdapter) throwFailedResponses(ctx context.Context, response *nethttp.Response, errorMappings abs.ErrorMappings, spanForAttributes trace.Span) error {
	ctx, span := otel.GetTracerProvider().Tracer(a.observabilityName).Start(ctx, "throwFailedResponses")
	defer span.End()
	if response.StatusCode < 400 {
		return nil
	}
	span.SetStatus(codes.Error, "received_error_response")

	statusAsString := strconv.Itoa(response.StatusCode)
	var errorCtor absser.ParsableFactory = nil
	if len(errorMappings) != 0 {
		if errorMappings[statusAsString] != nil {
			errorCtor = errorMappings[statusAsString]
		} else if response.StatusCode >= 400 && response.StatusCode < 500 && errorMappings["4XX"] != nil {
			errorCtor = errorMappings["4XX"]
		} else if response.StatusCode >= 500 && response.StatusCode < 600 && errorMappings["5XX"] != nil {
			errorCtor = errorMappings["5XX"]
		}
	}

	if errorCtor == nil {
		spanForAttributes.SetAttributes(attribute.Bool(ErrorMappingFoundAttributeName, false))
		return &abs.ApiError{
			Message: "The server returned an unexpected status code and no error factory is registered for this code: " + statusAsString,
		}
	}
	spanForAttributes.SetAttributes(attribute.Bool(ErrorMappingFoundAttributeName, true))

	rootNode, _, err := a.getRootParseNode(ctx, response, spanForAttributes)
	if err != nil {
		return err
	}
	if rootNode == nil {
		spanForAttributes.SetAttributes(attribute.Bool(ErrorMappingFoundAttributeName, false))
		return &abs.ApiError{
			Message: "The server returned an unexpected status code with no response body: " + statusAsString,
		}
	}
	spanForAttributes.SetAttributes(attribute.Bool(ErrorMappingFoundAttributeName, true))

	_, deserializeSpan := otel.GetTracerProvider().Tracer(a.observabilityName).Start(ctx, "GetObjectValue")
	defer deserializeSpan.End()
	errValue, err := rootNode.GetObjectValue(errorCtor)
	if err != nil {
		return err
	} else if errValue == nil {
		return &abs.ApiError{
			Message: "The server returned an unexpected status code but the error could not be deserialized: " + statusAsString,
		}
	}

	return errValue.(error)
}
