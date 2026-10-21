package nethttplibrary

import (
	assert "github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type TestMiddleware struct{}

func (middleware TestMiddleware) Intercept(pipeline Pipeline, middlewareIndex int, req *http.Request) (*http.Response, error) {
	req.Header.Add("test", "test-header")

	return pipeline.Next(req, middlewareIndex)
}

func TestCanInterceptRequests(t *testing.T) {
	transport := NewCustomTransport(&TestMiddleware{})
	client := &http.Client{Transport: transport}
	resp, _ := client.Get("https://example.com")

	expect := "test-header"
	got := resp.Request.Header.Get("test")

	if expect != got {
		t.Errorf("Expected: %v, but received: %v", expect, got)
	}
}

func TestCanInterceptMultipleRequests(t *testing.T) {
	transport := NewCustomTransport(&TestMiddleware{})
	client := &http.Client{Transport: transport}
	resp, _ := client.Get("https://example.com")

	expect := "test-header"
	got := resp.Request.Header.Get("test")

	if expect != got {
		t.Errorf("Expected: %v, but received: %v", expect, got)
	}

	resp2, _ := client.Get("https://example.com")

	got2 := resp2.Request.Header.Get("test")

	if expect != got2 {
		t.Errorf("Expected: %v, but received: %v", expect, got2)
	}
}

func TestItReturnsADefaultTransport(t *testing.T) {
	transport := GetDefaultTransport()
	assert.NotNil(t, transport)
	defaultTransport, ok := transport.(*http.Transport)
	assert.True(t, ok)
	assert.True(t, defaultTransport.ForceAttemptHTTP2)
}

func TestItAcceptsACustomizedTransport(t *testing.T) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ForceAttemptHTTP2 = false
	customTransport := NewCustomTransportWithParentTransport(transport)
	assert.NotNil(t, customTransport)
	result, ok := customTransport.middlewarePipeline.transport.(*http.Transport)
	assert.True(t, ok)
	assert.False(t, result.ForceAttemptHTTP2)
}

func TestItGetsADefaultTransportIfNoneIsProvided(t *testing.T) {
	customTransport := NewCustomTransport()
	assert.NotNil(t, customTransport.middlewarePipeline.transport)
}

type eofTrailerBody struct {
	data     []byte
	trailer  http.Header
	key      string
	value    string
	readDone bool
}

func (b *eofTrailerBody) Read(p []byte) (int, error) {
	if !b.readDone {
		b.readDone = true
		n := copy(p, b.data)
		return n, nil
	}
	b.trailer.Set(b.key, b.value)
	return 0, io.EOF
}

func (b *eofTrailerBody) Close() error {
	return nil
}

func TestRoundTripPreservesTrailersPopulatedAtEOF(t *testing.T) {
	var receivedTrailer string
	testServer := httptest.NewServer(http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		_, err := io.ReadAll(req.Body)
		assert.NoError(t, err)
		receivedTrailer = req.Trailer.Get("X-Checksum")
		res.WriteHeader(http.StatusOK)
	}))
	defer testServer.Close()

	originalTrailer := make(http.Header)
	originalTrailer.Add("X-Checksum", "")

	body := &eofTrailerBody{
		data:    []byte("streaming chunked payload"),
		trailer: originalTrailer,
		key:     "X-Checksum",
		value:   "sha256-eof-checksum-value",
	}

	req, err := http.NewRequest(http.MethodPost, testServer.URL, body)
	assert.NoError(t, err)
	req.ContentLength = -1
	req.Trailer = originalTrailer

	transport := NewCustomTransport(&TestMiddleware{})
	client := &http.Client{Transport: transport}

	resp, err := client.Do(req)
	assert.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "sha256-eof-checksum-value", receivedTrailer)
}
