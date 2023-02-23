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

	handler := NewUrlReplaceHandler()
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
