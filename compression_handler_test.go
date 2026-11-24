package nethttplibrary

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	nethttp "net/http"
	httptest "net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCompressionHandlerAddsAcceptEncodingHeader(t *testing.T) {
	postBody, _ := json.Marshal(map[string]string{"name": "Test", "email": "Test@Test.com"})
	var acceptEncodingHeader string
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		acceptEncodingHeader = req.Header.Get("Accept-Encoding")
		fmt.Fprint(res, `{}`)
	}))
	defer testServer.Close()

	client := GetDefaultClient(NewCompressionHandler())
	client.Post(testServer.URL, "application/json", bytes.NewBuffer(postBody))

	assert.Equal(t, acceptEncodingHeader, "gzip")
}

func TestCompressionHandlerAddsContentEncodingHeader(t *testing.T) {
	postBody, _ := json.Marshal(map[string]string{"name": "Test", "email": "Test@Test.com"})
	var contentTypeHeader string
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		contentTypeHeader = req.Header.Get("Content-Encoding")
		fmt.Fprint(res, `{}`)
	}))
	defer testServer.Close()

	client := GetDefaultClient(NewCompressionHandler())
	client.Post(testServer.URL, "application/json", bytes.NewBuffer(postBody))

	assert.Equal(t, contentTypeHeader, "gzip")
}

func TestCompressionHandlerCompressesRequestBody(t *testing.T) {
	postBody, _ := json.Marshal(map[string]string{"name": `Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book. It has Contrary to popular belief, Lorem Ipsum is not simply random text. It has roots in a piece of classical Latin literature from 45 BC, making it over 2000 years old. Richard McClintock, a Latin professor at Hampden-Sydney College in Virginia, looked up one of the more obscure Latin words, consectetur, from a Lorem Ipsum passage, and going through the cites of the word in classical literature, discovered the undoubtable source. Lorem Ipsum comes from sections 1.10.32 and 1.10.33 of "de Finibus Bonorum et Malorum" (The Extremes of Good and Evil) by Cicero, written in 45 BC. This book is a treatise on the theory of ethics, very popular during the Renaissance. The first line of Lorem Ipsum, "Lorem ipsum dolor sit amet..", comes from Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book. It has survived not only five centuries, but also the leap into electronic typesetting, remaining essentially unchanged. It was popularised in the 1960s with the release of Letraset sheets containing Lorem Ipsum passages, and more recently with desktop publishing software like Aldus PageMaker including versions of Lorem Ipsum.a line in section 1.10.32.
    `, "email": "Test@Test.com"})
	var compressedBody []byte
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		compressedBody, _ = io.ReadAll(req.Body)
		fmt.Fprint(res, `{}`)
	}))
	defer testServer.Close()

	client := GetDefaultClient(NewCompressionHandler())
	client.Post(testServer.URL, "application/json", bytes.NewBuffer(postBody))

	assert.Greater(t, len(postBody), len(compressedBody))

}

func TestCompressionHandlerCompressesRequestBodyWithRetry(t *testing.T) {
	postBody, _ := json.Marshal(map[string]string{"name": `Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book. It has Contrary to popular belief, Lorem Ipsum is not simply random text. It has roots in a piece of classical Latin literature from 45 BC, making it over 2000 years old. Richard McClintock, a Latin professor at Hampden-Sydney College in Virginia, looked up one of the more obscure Latin words, consectetur, from a Lorem Ipsum passage, and going through the cites of the word in classical literature, discovered the undoubtable source. Lorem Ipsum comes from sections 1.10.32 and 1.10.33 of "de Finibus Bonorum et Malorum" (The Extremes of Good and Evil) by Cicero, written in 45 BC. This book is a treatise on the theory of ethics, very popular during the Renaissance. The first line of Lorem Ipsum, "Lorem ipsum dolor sit amet..", comes from Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book. It has survived not only five centuries, but also the leap into electronic typesetting, remaining essentially unchanged. It was popularised in the 1960s with the release of Letraset sheets containing Lorem Ipsum passages, and more recently with desktop publishing software like Aldus PageMaker including versions of Lorem Ipsum.a line in section 1.10.32.
    `, "email": "Test@Test.com"})

	var compressedBody []byte
	var requestCount int
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		if requestCount == 0 {
			res.WriteHeader(http.StatusTooManyRequests)
			requestCount++
			return
		}

		compressedBody, _ = io.ReadAll(req.Body)
		fmt.Fprint(res, `{}`)
	}))
	defer testServer.Close()

	client := GetDefaultClient(NewCompressionHandler(), NewRetryHandler())
	_, err := client.Post(testServer.URL, "application/json", bytes.NewBuffer(postBody))

	assert.NotZero(t, len(compressedBody))
	assert.Greater(t, len(postBody), len(compressedBody))
	assert.Equal(t, requestCount, 1)
	assert.NoError(t, err)
}

func TestCompressionHandlerContentRangeRequestBody(t *testing.T) {
	postBody, _ := json.Marshal(map[string]string{"name": `Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book. It has Contrary to popular belief, Lorem Ipsum is not simply random text. It has roots in a piece of classical Latin literature from 45 BC, making it over 2000 years old. Richard McClintock, a Latin professor at Hampden-Sydney College in Virginia, looked up one of the more obscure Latin words, consectetur, from a Lorem Ipsum passage, and going through the cites of the word in classical literature, discovered the undoubtable source. Lorem Ipsum comes from sections 1.10.32 and 1.10.33 of "de Finibus Bonorum et Malorum" (The Extremes of Good and Evil) by Cicero, written in 45 BC. This book is a treatise on the theory of ethics, very popular during the Renaissance. The first line of Lorem Ipsum, "Lorem ipsum dolor sit amet..", comes from Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book. It has survived not only five centuries, but also the leap into electronic typesetting, remaining essentially unchanged. It was popularised in the 1960s with the release of Letraset sheets containing Lorem Ipsum passages, and more recently with desktop publishing software like Aldus PageMaker including versions of Lorem Ipsum.a line in section 1.10.32.
    `, "email": "Test@Test.com"})
	var compressedBody []byte
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		compressedBody, _ = io.ReadAll(req.Body)
		fmt.Fprint(res, `{}`)
	}))
	defer testServer.Close()

	client := GetDefaultClient(NewCompressionHandler())
	req, _ := nethttp.NewRequest("PUT", testServer.URL, bytes.NewBuffer(postBody))
	req.Header.Add("Content-Range", "bytes 0-3/4")
	client.Do(req)

	assert.Equal(t, len(postBody), len(compressedBody))
}

func TestCompressionHandlerContentEncodingRequestBody(t *testing.T) {
	postBody, _ := json.Marshal(map[string]string{"name": `Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book. It has Contrary to popular belief, Lorem Ipsum is not simply random text. It has roots in a piece of classical Latin literature from 45 BC, making it over 2000 years old. Richard McClintock, a Latin professor at Hampden-Sydney College in Virginia, looked up one of the more obscure Latin words, consectetur, from a Lorem Ipsum passage, and going through the cites of the word in classical literature, discovered the undoubtable source. Lorem Ipsum comes from sections 1.10.32 and 1.10.33 of "de Finibus Bonorum et Malorum" (The Extremes of Good and Evil) by Cicero, written in 45 BC. This book is a treatise on the theory of ethics, very popular during the Renaissance. The first line of Lorem Ipsum, "Lorem ipsum dolor sit amet..", comes from Lorem Ipsum is simply dummy text of the printing and typesetting industry. Lorem Ipsum has been the industry's standard dummy text ever since the 1500s, when an unknown printer took a galley of type and scrambled it to make a type specimen book. It has survived not only five centuries, but also the leap into electronic typesetting, remaining essentially unchanged. It was popularised in the 1960s with the release of Letraset sheets containing Lorem Ipsum passages, and more recently with desktop publishing software like Aldus PageMaker including versions of Lorem Ipsum.a line in section 1.10.32.
    `, "email": "Test@Test.com"})
	var compressedBody []byte
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		compressedBody, _ = io.ReadAll(req.Body)
		fmt.Fprint(res, `{}`)
	}))
	defer testServer.Close()

	client := GetDefaultClient(NewCompressionHandler())
	req, _ := nethttp.NewRequest("PUT", testServer.URL, bytes.NewBuffer(postBody))
	req.Header.Add("Content-Encoding", "gzip")
	client.Do(req)

	assert.Equal(t, len(postBody), len(compressedBody))
}

func TestCompressionHandlerRetriesRequest(t *testing.T) {
	postBody, _ := json.Marshal(map[string]string{"name": "Test", "email": "Test@Test.com"})
	status := 415
	reqCount := 0

	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		defer req.Body.Close()
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(status)
		status = 200
		reqCount += 1
		fmt.Fprint(res, `{}`)
	}))
	defer testServer.Close()

	client := getDefaultClientWithoutMiddleware()
	client.Transport = NewCustomTransport(NewCompressionHandler())
	client.Post(testServer.URL, "application/json", bytes.NewBuffer(postBody))

	assert.Equal(t, reqCount, 2)
}

func TestCompressionHandlerWorksWithEmptyBody(t *testing.T) {
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		result, _ := json.Marshal(map[string]string{"name": "Test", "email": "Test@Test.com"})

		res.Header().Set("Content-Type", "application/json")
		res.Header().Set("Content-Encoding", "gzip")
		fmt.Fprint(res, result)
	}))
	defer testServer.Close()

	client := getDefaultClientWithoutMiddleware()
	client.Transport = NewCustomTransport(NewCompressionHandler())

	fmt.Print(testServer.URL)
	resp, _ := client.Get(testServer.URL)

	assert.NotNil(t, resp)
}

func TestResetTransport(t *testing.T) {
	client := getDefaultClientWithoutMiddleware()
	client.Transport = &nethttp.Transport{}
}
func TestCompressionHandlerNilRequest(t *testing.T) {
	handler := NewCompressionHandler()
	resp, err := handler.Intercept(nil, 0, nil)
	assert.Error(t, err)
	assert.Nil(t, resp)
	assert.Contains(t, err.Error(), "req cannot be nil")
}

func TestCompressionHandlerTypedNilOptionInContext(t *testing.T) {
	testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
		res.WriteHeader(200)
	}))
	defer testServer.Close()

	handler := NewCompressionHandler()
	pipeline := newMiddlewarePipeline([]Middleware{handler}, nethttp.DefaultTransport)

	req, err := nethttp.NewRequest("GET", testServer.URL, nil)
	assert.NoError(t, err)
	var nilOpt *CompressionOptions
	ctx := context.WithValue(req.Context(), compressKey, nilOpt)
	req = req.WithContext(ctx)

	assert.NotPanics(t, func() {
		resp, err := handler.Intercept(pipeline, 0, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
	})
}

func TestCompressionHandlerCloses415ResponseBodyOnRetry(t *testing.T) {
	handler := NewCompressionHandler()
	bodyTracker := &trackingReadCloser{Reader: strings.NewReader("unsupported media type")}
	resp415 := &nethttp.Response{
		StatusCode: 415,
		Header:     nethttp.Header{},
		Body:       bodyTracker,
	}
	resp200 := &nethttp.Response{
		StatusCode: nethttp.StatusOK,
		Header:     nethttp.Header{},
	}
	req, err := nethttp.NewRequest(nethttp.MethodPost, "https://example.com/upload", strings.NewReader("hello world"))
	assert.NoError(t, err)

	callCount := 0
	pipeline := &dynamicPipeline{
		next: func(r *nethttp.Request, index int) (*nethttp.Response, error) {
			callCount++
			if callCount == 1 {
				return resp415, nil
			}
			return resp200, nil
		},
	}
	resp, err := handler.Intercept(pipeline, 0, req)
	assert.NoError(t, err)
	assert.Equal(t, nethttp.StatusOK, resp.StatusCode)
	assert.True(t, bodyTracker.closed, "415 response body should be closed on retry")
}

func TestCompressionHandlerPreservesSeekableBodyOn415Retry(t *testing.T) {
	handler := NewCompressionHandler()
	payload := "payload data that needs to be seekable"
	req, err := nethttp.NewRequest(nethttp.MethodPost, "https://example.com/test", strings.NewReader(payload))
	require.NoError(t, err)
	req.Header.Set("Content-Length", strconv.Itoa(len(payload)))
	originalGetBody := req.GetBody
	replays := 0
	req.GetBody = func() (io.ReadCloser, error) {
		replays++
		return originalGetBody()
	}

	callCount := 0
	pipeline := &dynamicPipeline{
		next: func(r *nethttp.Request, index int) (*nethttp.Response, error) {
			callCount++
			if callCount == 1 {
				assert.Equal(t, strconv.FormatInt(r.ContentLength, 10), r.Header.Get("Content-Length"))
				return &nethttp.Response{
					StatusCode: 415,
					Header:     nethttp.Header{},
					Body:       io.NopCloser(strings.NewReader("")),
				}, nil
			}
			assert.Empty(t, r.Header.Get("Content-Encoding"))
			assert.Equal(t, int64(len(payload)), r.ContentLength)
			assert.Equal(t, strconv.Itoa(len(payload)), r.Header.Get("Content-Length"))
			replay, replayErr := r.GetBody()
			require.NoError(t, replayErr)
			defer replay.Close()
			replayedBody, readErr := io.ReadAll(replay)
			require.NoError(t, readErr)
			assert.Equal(t, payload, string(replayedBody))
			assert.Equal(t, 1, replays, "the fallback must restore the original GetBody")

			// On the retried request, verify body implements io.Seeker and can be rewound
			seeker, ok := r.Body.(io.Seeker)
			assert.True(t, ok, "retried uncompressed request body must implement io.Seeker")
			if ok {
				readBody, readErr := io.ReadAll(r.Body)
				assert.NoError(t, readErr)
				assert.Equal(t, payload, string(readBody))

				// Rewind and read again to ensure seeking works
				_, seekErr := seeker.Seek(0, io.SeekStart)
				assert.NoError(t, seekErr)
				readBody2, readErr2 := io.ReadAll(r.Body)
				assert.NoError(t, readErr2)
				assert.Equal(t, payload, string(readBody2))
			}
			return &nethttp.Response{
				StatusCode: nethttp.StatusOK,
				Header:     nethttp.Header{},
			}, nil
		},
	}

	resp, err := handler.Intercept(pipeline, 0, req)
	require.NoError(t, err)
	assert.Equal(t, nethttp.StatusOK, resp.StatusCode)
	assert.Equal(t, 2, callCount)
}

func TestDefaultMiddlewareReplaysCompressedBodyThroughRedirectsAndRetry(t *testing.T) {
	const payload = "payload sent through redirects and retry"
	type hop struct {
		path, method, encoding, retryAttempt string
		length                               int64
		body                                 []byte
	}
	hops := make(chan hop, 8)
	server := httptest.NewServer(nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		select {
		case hops <- hop{r.URL.Path, r.Method, r.Header.Get("Content-Encoding"), r.Header.Get("Retry-Attempt"), r.ContentLength, body}:
		default:
			t.Error("unexpected extra request")
		}

		switch r.URL.Path {
		case "/start":
			w.Header().Set("Location", "/next")
			w.WriteHeader(nethttp.StatusTemporaryRedirect)
		case "/next":
			w.Header().Set("Location", "/target")
			w.WriteHeader(nethttp.StatusPermanentRedirect)
		case "/target":
			if r.Header.Get("Retry-Attempt") == "" {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(nethttp.StatusServiceUnavailable)
			}
		}
	}))
	defer server.Close()

	// Like requests from the adapter, this request has no original GetBody.
	req, err := nethttp.NewRequest(nethttp.MethodPost, server.URL+"/start", NopCloser(strings.NewReader(payload)))
	require.NoError(t, err)
	req.ContentLength = int64(len(payload))
	resp, err := GetDefaultClient().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, nethttp.StatusOK, resp.StatusCode)
	require.Len(t, hops, 4)

	var firstBody []byte
	for i, path := range []string{"/start", "/next", "/target", "/target"} {
		got := <-hops
		assert.Equal(t, path, got.path)
		assert.Equal(t, nethttp.MethodPost, got.method)
		assert.Equal(t, "gzip", got.encoding)
		assert.Equal(t, int64(len(got.body)), got.length)
		if i == 0 {
			firstBody = got.body
			reader, err := gzip.NewReader(bytes.NewReader(firstBody))
			require.NoError(t, err)
			defer reader.Close()
			decoded, err := io.ReadAll(reader)
			require.NoError(t, err)
			assert.Equal(t, payload, string(decoded))
		}
		if i == 3 {
			assert.Equal(t, "1", got.retryAttempt)
		}
		assert.Equal(t, firstBody, got.body)
	}
}
