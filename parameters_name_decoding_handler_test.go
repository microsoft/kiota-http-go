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
		{"?q=1%2B2", "/?q=1%2B2"},                       //Values are not decoded
		{"?q=M%26A", "/?q=M%26A"},                       //Values are not decoded
		{"?q%2D1=M%26A", "/?q-1=M%26A"},                 //Values are not decoded but params are
		{"?q%2D1&q=M%26A=M%26A", "/?q-1&q=M%26A=M%26A"}, //Values are not decoded but params are
		{"?%24select=diplayName&api%2Dversion=1%2B2", "/?$select=diplayName&api-version=1%2B2"}, //Values are not decoded but params are
		{"?%24select=diplayName&api%2Dversion=M%26A", "/?$select=diplayName&api-version=M%26A"}, //Values are not decoded but params are
		{"?%24select=diplayName&api%7Eversion=M%26A", "/?$select=diplayName&api~version=M%26A"}, //Values are not decoded but params are
		{"?%24select=diplayName&api%2Eversion=M%26A", "/?$select=diplayName&api.version=M%26A"}, //Values are not decoded but params are
		{"?%24select=diplayName&api%2Eversion=M%26A", "/?$select=diplayName&api.version=M%26A"}, //Values are not decoded but params are
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
func TestItPreservesEncodedCharactersInQueryParameterValues(t *testing.T) {
	testData := [][]string{
		{"?query=%24select&%24select=displayName", "/?query=%24select&$select=displayName"},
		{"?filter=name%20eq%20%27%24test%27&api%2Dversion=2", "/?filter=name%20eq%20%27%24test%27&api-version=2"},
		{"?flag&%24top=5", "/?flag&$top=5"},
		{"?empty=&%24count=true", "/?empty=&$count=true"},
	}
	result := ""
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		result = req.URL.String()
		res.WriteHeader(200)
		res.Write([]byte("body"))
	}))
	defer testServer.Close()
	for _, data := range testData {
		handler := NewParametersNameDecodingHandler()
		input := testServer.URL + data[0]
		expected := data[1]
		req, err := nethttp.NewRequest(nethttp.MethodGet, input, nil)
		assert.NoError(t, err)
		resp, err := handler.Intercept(newNoopPipeline(), 0, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, expected, result)
	}
}

func TestParametersNameDecodingHandlerNilRequest(t *testing.T) {
	handler := NewParametersNameDecodingHandler()
	assert.NotPanics(t, func() {
		resp, err := handler.Intercept(newNoopPipeline(), 0, nil)
		assert.Error(t, err)
		assert.Nil(t, resp)
	})
}
