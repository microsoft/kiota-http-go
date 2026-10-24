package nethttplibrary

import (
	"errors"
	"github.com/stretchr/testify/assert"
	nethttp "net/http"
	"testing"
)

type SpyPipeline struct {
	client          *nethttp.Client
	receivedRequest *nethttp.Request
}

func (pipeline *SpyPipeline) Next(req *nethttp.Request, middlewareIndex int) (*nethttp.Response, error) {
	pipeline.receivedRequest = req
	return nil, errors.New("Spy executor only")
}
func newSpyPipeline() *SpyPipeline {
	return &SpyPipeline{
		client: getDefaultClientWithoutMiddleware(),
	}
}
func (pipeline *SpyPipeline) GetReceivedRequest() *nethttp.Request {
	return pipeline.receivedRequest
}

func TestURLReplacementHandler(t *testing.T) {

	handler := NewUrlReplaceHandler(true, map[string]string{"/users/me-token-to-replace": "/me"})
	if handler == nil {
		t.Error("handler is nil")
	}
	url := "https://msgraph.com/users/me-token-to-replace/contactFolders"
	req, err := nethttp.NewRequest(nethttp.MethodGet, url, nil)
	if err != nil {
		t.Error(err)
	}

	pipeline := newSpyPipeline()
	_, _ = handler.Intercept(pipeline, 0, req)

	assert.Equal(t, pipeline.GetReceivedRequest().URL.Path, "/me/contactFolders")
}

func TestNewUrlReplaceOptions(t *testing.T) {
	options := NewUrlReplaceOptions()
	assert.NotNil(t, options)
	assert.True(t, options.IsEnabled())
	assert.Nil(t, options.GetReplacementPairs())
	assert.Equal(t, options.GetKey(), urlReplaceOptionKey)
}

func TestNewUrlReplaceHandlerWithOptions(t *testing.T) {
	options := UrlReplaceOptions{
		Enabled:          true,
		ReplacementPairs: map[string]string{"foo": "bar"},
	}
	handler := NewUrlReplaceHandlerWithOptions(options)
	assert.NotNil(t, handler)
	assert.True(t, handler.options.IsEnabled())
	assert.Equal(t, handler.options.GetReplacementPairs(), map[string]string{"foo": "bar"})
}



func TestUrlReplaceHandlerNilRequest(t *testing.T) {
	handler := NewUrlReplaceHandler(true, map[string]string{"foo": "bar"})
	assert.NotPanics(t, func() {
		resp, err := handler.Intercept(newNoopPipeline(), 0, nil)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}

func TestUrlReplaceHandlerNilURL(t *testing.T) {
	handler := NewUrlReplaceHandler(true, map[string]string{"foo": "bar"})
	req := &nethttp.Request{}
	assert.NotPanics(t, func() {
		resp, err := handler.Intercept(newNoopPipeline(), 0, req)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}