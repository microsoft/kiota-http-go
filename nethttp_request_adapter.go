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
	if authenticationProvider == nil {
		return nil, errors.New("authenticationProvider cannot be nil")
	}
	result := &NetHttpRequestAdapter{
		serializationWriterFactory: serializationWriterFactory,
		parseNodeFactory:           parseNodeFactory,
		httpClient:                 httpClient,
		authenticationProvider:     authenticationProvider,
		baseUrl:                    "",
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

func (a *NetHttpRequestAdapter) getHttpResponseMessage(ctx context.Context, requestInfo *abs.RequestInformation, claims string) (*nethttp.Response, error) {
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
	request, err := a.getRequestFromRequestInformation(ctx, requestInfo)
	if err != nil {
		return nil, err
	}
	response, err := (*a.httpClient).Do(request)
	if err != nil {
		return nil, err
	}
	return a.retryCAEResponseIfRequired(ctx, response, requestInfo, claims)
}

const claimsKey = "claims"

var reBearer = regexp.MustCompile(`(?i)^Bearer\s`)
var reClaims = regexp.MustCompile(`\"([^\"]*)\"`)

func (a *NetHttpRequestAdapter) retryCAEResponseIfRequired(ctx context.Context, response *nethttp.Response, requestInfo *abs.RequestInformation, claims string) (*nethttp.Response, error) {
	if response.StatusCode == 401 &&
		claims == "" { //avoid infinite loop, we only retry once
		authenticateHeaderVal := response.Header.Get("WWW-Authenticate")
		if authenticateHeaderVal != "" && reBearer.Match([]byte(authenticateHeaderVal)) {
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
				return a.getHttpResponseMessage(ctx, requestInfo, responseClaims)
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

func (a *NetHttpRequestAdapter) getRequestFromRequestInformation(ctx context.Context, requestInfo *abs.RequestInformation) (*nethttp.Request, error) {
	uri, err := requestInfo.GetUri()
	if err != nil {
		return nil, err
	}

	// set deadline if not set in receiving context
	if _, deadlineSet := ctx.Deadline(); !deadlineSet {
		ctxTimed, _ := context.WithTimeout(ctx, time.Second*requestTimeOutInSeconds)
		ctx = ctxTimed
	}

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
	}
	for _, value := range requestInfo.GetRequestOptions() {
		request = request.WithContext(context.WithValue(ctx, value.GetKey(), value))
	}
	return request, nil
}

// SendAsync executes the HTTP request specified by the given RequestInformation and returns the deserialized response model.
func (a *NetHttpRequestAdapter) SendAsync(ctx context.Context, requestInfo *abs.RequestInformation, constructor absser.ParsableFactory, errorMappings abs.ErrorMappings) (absser.Parsable, error) {
	if requestInfo == nil {
		return nil, errors.New("requestInfo cannot be nil")
	}
	response, err := a.getHttpResponseMessage(ctx, requestInfo, "")
	if err != nil {
		return nil, err
	}

	responseHandler := getResponseHandler(ctx)
	if responseHandler != nil {
		result, err := responseHandler(response, errorMappings)
		if err != nil {
			return nil, err
		}
		return result.(absser.Parsable), nil
	} else if response != nil {
		defer a.purge(response)
		err = a.throwFailedResponses(response, errorMappings)
		if err != nil {
			return nil, err
		}
		if a.shouldReturnNil(response) {
			return nil, err
		}
		parseNode, err := a.getRootParseNode(response)
		if err != nil {
			return nil, err
		}
		if parseNode == nil {
			return nil, nil
		}
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
	response, err := a.getHttpResponseMessage(ctx, requestInfo, "")
	if err != nil {
		return nil, err
	}

	responseHandler := getResponseHandler(ctx)
	if responseHandler != nil {
		result, err := responseHandler(response, errorMappings)
		if err != nil {
			return nil, err
		}
		return result.(absser.Parsable), nil
	} else if response != nil {
		defer a.purge(response)
		err = a.throwFailedResponses(response, errorMappings)
		if err != nil {
			return nil, err
		}
		if a.shouldReturnNil(response) {
			return nil, err
		}
		parseNode, err := a.getRootParseNode(response)
		if err != nil {
			return nil, err
		}
		if parseNode == nil {
			return nil, nil
		}
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
	response, err := a.getHttpResponseMessage(ctx, requestInfo, "")
	if err != nil {
		return nil, err
	}

	responseHandler := getResponseHandler(ctx)
	if responseHandler != nil {
		result, err := responseHandler(response, errorMappings)
		if err != nil {
			return nil, err
		}
		return result.([]absser.Parsable), nil
	} else if response != nil {
		defer a.purge(response)
		err = a.throwFailedResponses(response, errorMappings)
		if err != nil {
			return nil, err
		}
		if a.shouldReturnNil(response) {
			return nil, err
		}
		parseNode, err := a.getRootParseNode(response)
		if err != nil {
			return nil, err
		}
		if parseNode == nil {
			return nil, nil
		}
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
	response, err := a.getHttpResponseMessage(ctx, requestInfo, "")
	if err != nil {
		return nil, err
	}

	responseHandler := getResponseHandler(ctx)
	if responseHandler != nil {
		result, err := responseHandler(response, errorMappings)
		if err != nil {
			return nil, err
		}
		return result.([]interface{}), nil
	} else if response != nil {
		defer a.purge(response)
		err = a.throwFailedResponses(response, errorMappings)
		if err != nil {
			return nil, err
		}
		if a.shouldReturnNil(response) {
			return nil, err
		}
		parseNode, err := a.getRootParseNode(response)
		if err != nil {
			return nil, err
		}
		if parseNode == nil {
			return nil, nil
		}
		result, err := parseNode.GetCollectionOfEnumValues(parser)
		return result, err
	} else {
		return nil, errors.New("response is nil")
	}
}

func getResponseHandler(ctx context.Context) abs.ResponseHandler {
	var optionKey = ctx.Value(abs.ResponseHandlerOptionKey)
	if optionKey != nil {
		return ctx.Value(abs.ResponseHandlerOptionKey).(abs.RequestHandlerOption).GetResponseHandler()
	}
	return nil
}

// SendPrimitiveAsync executes the HTTP request specified by the given RequestInformation and returns the deserialized primitive response model.
func (a *NetHttpRequestAdapter) SendPrimitiveAsync(ctx context.Context, requestInfo *abs.RequestInformation, typeName string, errorMappings abs.ErrorMappings) (interface{}, error) {
	if requestInfo == nil {
		return nil, errors.New("requestInfo cannot be nil")
	}
	response, err := a.getHttpResponseMessage(ctx, requestInfo, "")
	if err != nil {
		return nil, err
	}

	responseHandler := getResponseHandler(ctx)
	if responseHandler != nil {
		result, err := responseHandler(response, errorMappings)
		if err != nil {
			return nil, err
		}
		return result.(absser.Parsable), nil
	} else if response != nil {
		defer a.purge(response)
		err = a.throwFailedResponses(response, errorMappings)
		if err != nil {
			return nil, err
		}
		if a.shouldReturnNil(response) {
			return nil, err
		}
		if typeName == "[]byte" {
			return ioutil.ReadAll(response.Body)
		}
		parseNode, err := a.getRootParseNode(response)
		if err != nil {
			return nil, err
		}
		if parseNode == nil {
			return nil, nil
		}
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
	response, err := a.getHttpResponseMessage(ctx, requestInfo, "")
	if err != nil {
		return nil, err
	}

	responseHandler := getResponseHandler(ctx)
	if responseHandler != nil {
		result, err := responseHandler(response, errorMappings)
		if err != nil {
			return nil, err
		}
		return result.([]interface{}), nil
	} else if response != nil {
		defer a.purge(response)
		err = a.throwFailedResponses(response, errorMappings)
		if err != nil {
			return nil, err
		}
		if a.shouldReturnNil(response) {
			return nil, err
		}
		parseNode, err := a.getRootParseNode(response)
		if err != nil {
			return nil, err
		}
		if parseNode == nil {
			return nil, nil
		}
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
	response, err := a.getHttpResponseMessage(ctx, requestInfo, "")
	if err != nil {
		return err
	}

	responseHandler := getResponseHandler(ctx)
	if responseHandler != nil {
		_, err := responseHandler(response, errorMappings)
		return err
	} else if response != nil {
		defer a.purge(response)
		err = a.throwFailedResponses(response, errorMappings)
		if err != nil {
			return err
		}
		return nil
	} else {
		return errors.New("response is nil")
	}
}

func (a *NetHttpRequestAdapter) getRootParseNode(response *nethttp.Response) (absser.ParseNode, error) {
	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	contentType := a.getResponsePrimaryContentType(response)
	if contentType == "" {
		return nil, nil
	}
	return a.parseNodeFactory.GetRootParseNode(contentType, body)
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

func (a *NetHttpRequestAdapter) throwFailedResponses(response *nethttp.Response, errorMappings abs.ErrorMappings) error {
	if response.StatusCode < 400 {
		return nil
	}

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
		return &abs.ApiError{
			Message: "The server returned an unexpected status code and no error factory is registered for this code: " + statusAsString,
		}
	}

	rootNode, err := a.getRootParseNode(response)
	if err != nil {
		return err
	}
	if rootNode == nil {
		return &abs.ApiError{
			Message: "The server returned an unexpected status code with no response body: " + statusAsString,
		}
	}

	errValue, err := rootNode.GetObjectValue(errorCtor)
	if err != nil {
		return err
	}

	return errValue.(error)
}
