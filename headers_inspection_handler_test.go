package nethttplibrary

import (
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	abs "github.com/microsoft/kiota-abstractions-go"
	assert "github.com/stretchr/testify/assert"
)

type stubPipeline struct {
	response *nethttp.Response
	err      error
}

func (s *stubPipeline) Next(req *nethttp.Request, middlewareIndex int) (*nethttp.Response, error) {
	return s.response, s.err
}

func TestItCreateANewHeadersInspectionHandler(t *testing.T) {
	handler := NewHeadersInspectionHandler()
	assert.NotNil(t, handler)
	_, ok := any(handler).(Middleware)
	assert.True(t, ok, "handler does not implement Middleware")
}
func TestHeadersInspectionOptionsImplementTheOptionInterface(t *testing.T) {
	options := NewHeadersInspectionOptions()
	assert.NotNil(t, options)
	_, ok := any(options).(abs.RequestOption)
	assert.True(t, ok, "options does not implement optionsType")
}

func TestItGetsRequestHeaders(t *testing.T) {
	options := NewHeadersInspectionOptions()
	options.InspectRequestHeaders = true
	assert.Empty(t, options.GetRequestHeaders().ListKeys())
	handler := NewHeadersInspectionHandlerWithOptions(*options)
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.Header().Add("test", "test")
		res.WriteHeader(200)
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()
	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	req.Header.Add("test", "test")
	if err != nil {
		t.Error(err)
	}
	_, err = handler.Intercept(newNoopPipeline(), 0, req)
	if err != nil {
		t.Error(err)
	}

	assert.NotEmpty(t, options.GetRequestHeaders().ListKeys())
	assert.Equal(t, "test", options.GetRequestHeaders().Get("test")[0])
	assert.Empty(t, options.GetResponseHeaders().ListKeys())
}

func TestItGetsResponseHeaders(t *testing.T) {
	options := NewHeadersInspectionOptions()
	options.InspectResponseHeaders = true
	assert.Empty(t, options.GetRequestHeaders().ListKeys())
	handler := NewHeadersInspectionHandlerWithOptions(*options)
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.Header().Add("test", "test")
		res.WriteHeader(200)
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()
	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	req.Header.Add("test", "test")
	if err != nil {
		t.Error(err)
	}
	_, err = handler.Intercept(newNoopPipeline(), 0, req)
	if err != nil {
		t.Error(err)
	}

	assert.NotEmpty(t, options.GetResponseHeaders().ListKeys())
	assert.Equal(t, "test", options.GetResponseHeaders().Get("test")[0])
	assert.Empty(t, options.GetRequestHeaders().ListKeys())
}

func TestHeadersInspectionWithEmptyHeaderValuesDoesNotPanic(t *testing.T) {
	options := NewHeadersInspectionOptions()
	options.InspectRequestHeaders = true
	options.InspectResponseHeaders = true
	handler := NewHeadersInspectionHandlerWithOptions(*options)

	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com", nil)
	assert.NoError(t, err)
	req.Header["empty-req-header"] = []string{}
	req.Header.Add("valid-req-header", "valid")

	mockResp := &nethttp.Response{
		StatusCode: 200,
		Header: nethttp.Header{
			"empty-res-header": []string{},
			"valid-res-header": []string{"valid"},
		},
	}
	pipeline := &stubPipeline{response: mockResp}

	assert.NotPanics(t, func() {
		resp, err := handler.Intercept(pipeline, 0, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, mockResp, resp)
	})

	assert.Equal(t, "valid", options.GetRequestHeaders().Get("valid-req-header")[0])
	assert.Equal(t, "valid", options.GetResponseHeaders().Get("valid-res-header")[0])
	assert.Empty(t, options.GetRequestHeaders().Get("empty-req-header"))
	assert.Empty(t, options.GetResponseHeaders().Get("empty-res-header"))
}

func TestHeadersInspectionNilRequestDoesNotPanic(t *testing.T) {
	handler := NewHeadersInspectionHandler()
	assert.NotPanics(t, func() {
		resp, err := handler.Intercept(&stubPipeline{}, 0, nil)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestHeadersInspectionNilHeadersOptionsDoesNotPanic(t *testing.T) {
	options := &HeadersInspectionOptions{
		InspectRequestHeaders:  true,
		InspectResponseHeaders: true,
		RequestHeaders:         nil,
		ResponseHeaders:        nil,
	}
	handler := NewHeadersInspectionHandlerWithOptions(*options)
	mockResp := &nethttp.Response{
		StatusCode: 200,
		Header: nethttp.Header{
			"res-header": []string{"res-val"},
		},
	}
	pipeline := &stubPipeline{response: mockResp}

	req, err := nethttp.NewRequest(nethttp.MethodGet, "https://example.com", nil)
	assert.NoError(t, err)
	req.Header.Add("req-header", "req-val")

	assert.NotPanics(t, func() {
		resp, err := handler.Intercept(pipeline, 0, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})
}
