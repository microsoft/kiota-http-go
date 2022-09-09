package nethttplibrary

import (
	"github.com/stretchr/testify/assert"
	nethttp "net/http"
	"net/http/httptest"
	"testing"
)

func TestItCreatesANewChaosHandler(t *testing.T) {
	handler := NewChaosHandler()
	if handler == nil {
		t.Error("handler is nil")
	}
}

func TestItCreatesANewChaosHandlerWithInvalidOptions(t *testing.T) {
	_, err := NewChaosHandlerWithOptions(&ChaosHandlerOptions{
		ChaosPercentage: 101,
		ChaosStrategy:   Random,
	})
	if err == nil || err.Error() != "ChaosPercentage must be between 0 and 100" {
		t.Error("Expected initialization ")
	}
}

func TestItCreatesANewChaosHandlerWithOptions(t *testing.T) {
	options := &ChaosHandlerOptions{
		ChaosPercentage: 100,
		ChaosStrategy:   Random,
		StatusCode:      400,
	}
	handler, err := NewChaosHandlerWithOptions(options)
	if err != nil {
		t.Error(err)
	}
	if handler == nil {
		t.Error("handler is nil")
	}

	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.WriteHeader(200)
		_, err := res.Write([]byte("body"))
		if err != nil {
			t.Error(err)
		}
	}))
	req, err := nethttp.NewRequest(nethttp.MethodGet, testServer.URL, nil)
	if err != nil {
		t.Error(err)
	}
	resp, err := handler.Intercept(newNoopPipeline(), 0, req)
	if err != nil {
		t.Error(err)
	}
	assert.NotNil(t, resp)
	assert.Equal(t, 400, resp.StatusCode)
}
