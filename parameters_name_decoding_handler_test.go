package nethttplibrary

import (
	nethttp "net/http"
	httptest "net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestItDecodesQueryParameterNames(t *testing.T) {
	testData := [][]string{
		{"?%24select=diplayName&api%2Dversion=2", "/?$select=diplayName&api-version=2"},
		{"?%24select=diplayName&api%7Eversion=2", "/?$select=diplayName&api~version=2"},
		{"?%24select=diplayName&api%2Eversion=2", "/?$select=diplayName&api.version=2"},
		{"/api-version/?%24select=diplayName&api%2Eversion=2", "/api-version/?$select=diplayName&api.version=2"},
		{"", "/"},
	}
	result := ""
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		result = req.URL.String()
		res.WriteHeader(200)
		res.Write([]byte("body"))
	}))
	defer func() { testServer.Close() }()
	for _, data := range testData {

		handler := NewParametersNameDecodingHandler()
		input := testServer.URL + data[0]
		expected := data[1]
		req, err := nethttp.NewRequest(nethttp.MethodGet, input, nil)
		if err != nil {
			t.Error(err)
		}
		resp, err := handler.Intercept(newNoopPipeline(), 0, req)
		if err != nil {
			t.Error(err)
		}
		assert.NotNil(t, resp)
		assert.Equal(t, expected, result)
	}
}
