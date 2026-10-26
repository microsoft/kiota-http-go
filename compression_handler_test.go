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

func TestCompressionHandlerUpdatesGetBodyAndCloses415ResponseBody(t *testing.T) {
	postBody := []byte("payload to compress and test")
	req, err := nethttp.NewRequest("POST", "https://example.com/test", bytes.NewReader(postBody))
	assert.NoError(t, err)

	handler := NewCompressionHandler()
	body415 := &trackingReadCloser{Reader: strings.NewReader("unsupported media type"), closed: false}
	resp415 := &nethttp.Response{
		StatusCode: 415,
		Header:     nethttp.Header{},
		Body:       body415,
	}

	body200 := &trackingReadCloser{Reader: strings.NewReader("success"), closed: false}
	resp200 := &nethttp.Response{
		StatusCode: 200,
		Header:     nethttp.Header{},
		Body:       body200,
	}

	callCount := 0
	pipeline := &testFuncPipeline{
		next: func(r *nethttp.Request, middlewareIndex int) (*nethttp.Response, error) {
			callCount++
			if callCount == 1 {
				assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))
				assert.NotNil(t, r.GetBody)
				compressedBodyData, readBodyErr := io.ReadAll(r.Body)
				assert.NoError(t, readBodyErr)
				assert.NotEqual(t, postBody, compressedBodyData)

				replayedCompressed, getErr := r.GetBody()
				assert.NoError(t, getErr)
				defer replayedCompressed.Close()
				data, readErr := io.ReadAll(replayedCompressed)
				assert.NoError(t, readErr)
				assert.Equal(t, compressedBodyData, data)

				gzReader, gzErr := gzip.NewReader(bytes.NewReader(data))
				assert.NoError(t, gzErr)
				defer gzReader.Close()
				decompressed, decompErr := io.ReadAll(gzReader)
				assert.NoError(t, decompErr)
				assert.Equal(t, postBody, decompressed)
				return resp415, nil
			}
			assert.Empty(t, r.Header.Get("Content-Encoding"))
			assert.NotNil(t, r.GetBody)
			replayedUncompressed, getErr := r.GetBody()
			assert.NoError(t, getErr)
			defer replayedUncompressed.Close()
			data, readErr := io.ReadAll(replayedUncompressed)
			assert.NoError(t, readErr)
			assert.Equal(t, string(postBody), string(data))
			return resp200, nil
		},
	}

	resp, err := handler.Intercept(pipeline, 0, req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, 2, callCount)
	assert.True(t, body415.closed, "415 response body must be closed on retry")
	assert.False(t, body200.closed, "final 200 response body must remain open")
}

func TestCompressionHandlerHandlesNilRequest(t *testing.T) {
	var nilHandler *CompressionHandler
	resp, err := nilHandler.Intercept(newNoopPipeline(), 0, nil)
	assert.Error(t, err)
	assert.Nil(t, resp)
}

func TestCompressionHandlerClosesUnknownLengthAndLarge415ResponseBodyWithoutReading(t *testing.T) {
	postBody := []byte("payload to compress and test")
	handler := NewCompressionHandler()

	// Case 1: Unknown length 415 response body (ContentLength = -1)
	bodyUnknown := &trackingReadCloserWithReadCount{Reader: strings.NewReader("streaming 415"), closed: false}
	resp415Unknown := &nethttp.Response{
		StatusCode:    415,
		Header:        nethttp.Header{},
		Body:          bodyUnknown,
		ContentLength: -1,
	}
	resp200 := &nethttp.Response{
		StatusCode: 200,
		Header:     nethttp.Header{},
		Body:       &trackingReadCloser{Reader: strings.NewReader("success"), closed: false},
	}

	req1, err := nethttp.NewRequest("POST", "https://example.com/test", bytes.NewReader(postBody))
	assert.NoError(t, err)

	callCount := 0
	pipeline1 := &testFuncPipeline{
		next: func(r *nethttp.Request, middlewareIndex int) (*nethttp.Response, error) {
			callCount++
			if callCount == 1 {
				return resp415Unknown, nil
			}
			return resp200, nil
		},
	}

	resp, err := handler.Intercept(pipeline1, 0, req1)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.True(t, bodyUnknown.closed, "415 unknown-length body must be closed")
	assert.Equal(t, 0, bodyUnknown.readCount, "415 unknown-length body must not be read to prevent hangs")

	// Case 2: Large 415 response body exceeding maxDrainBytes
	bodyLarge := &trackingReadCloserWithReadCount{Reader: strings.NewReader("large 415 body"), closed: false}
	resp415Large := &nethttp.Response{
		StatusCode:    415,
		Header:        nethttp.Header{},
		Body:          bodyLarge,
		ContentLength: 10 * 1024 * 1024,
	}

	req2, err := nethttp.NewRequest("POST", "https://example.com/test", bytes.NewReader(postBody))
	assert.NoError(t, err)

	callCount = 0
	pipeline2 := &testFuncPipeline{
		next: func(r *nethttp.Request, middlewareIndex int) (*nethttp.Response, error) {
			callCount++
			if callCount == 1 {
				return resp415Large, nil
			}
			return resp200, nil
		},
	}

	resp, err = handler.Intercept(pipeline2, 0, req2)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.True(t, bodyLarge.closed, "415 large body must be closed")
	assert.Equal(t, 0, bodyLarge.readCount, "415 large body must not be read to prevent hangs")
}

func TestRedirectWithCompressionAndObservability(t *testing.T) {
	for _, statusCode := range []int{307, 308} {
		t.Run(strconv.Itoa(statusCode), func(t *testing.T) {
			postBody := []byte("this is a test payload that will be compressed and redirected")
			firstReqReceived := false
			redirectReqReceived := false

			testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
				if req.URL.Path == "/redirected" {
					redirectReqReceived = true
					assert.Equal(t, nethttp.MethodPost, req.Method)
					assert.Equal(t, "gzip", req.Header.Get("Content-Encoding"))

					rawBody, err := io.ReadAll(req.Body)
					assert.NoError(t, err)
					assert.NotEqual(t, postBody, rawBody, "redirected body must be compressed, not plain uncompressed bytes")
					assert.Equal(t, int64(len(rawBody)), req.ContentLength)

					gzReader, err := gzip.NewReader(bytes.NewReader(rawBody))
					assert.NoError(t, err)
					defer gzReader.Close()
					decompressed, err := io.ReadAll(gzReader)
					assert.NoError(t, err)
					assert.Equal(t, postBody, decompressed)

					res.WriteHeader(nethttp.StatusOK)
					res.Write([]byte("ok"))
					return
				}

				firstReqReceived = true
				assert.Equal(t, nethttp.MethodPost, req.Method)
				assert.Equal(t, "gzip", req.Header.Get("Content-Encoding"))
				res.Header().Set("Location", "/redirected")
				res.WriteHeader(statusCode)
			}))
			defer testServer.Close()

			redirectHandler := NewRedirectHandler()
			retryHandler := NewRetryHandler()
			headersInspectionHandler := NewHeadersInspectionHandler()
			compressionHandler := NewCompressionHandler()
			client := GetDefaultClient(redirectHandler, retryHandler, headersInspectionHandler, compressionHandler)

			obsOptions := &ObservabilityOptions{}
			ctx := context.WithValue(context.Background(), observabilityOptionsKeyValue, obsOptions)
			req, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodPost, testServer.URL, bytes.NewBuffer(postBody))
			assert.NoError(t, err)

			resp, err := client.Do(req)
			assert.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, nethttp.StatusOK, resp.StatusCode)
			assert.True(t, firstReqReceived, "first request should have been received")
			assert.True(t, redirectReqReceived, "redirected request should have been received")
		})
	}
}

func TestCompressionHandler415FallbackUpdatesOriginalRequestConsistent(t *testing.T) {
	postBody := []byte("payload to test 415 fallback consistency")
	obsOptions := &ObservabilityOptions{}
	ctx := context.WithValue(context.Background(), observabilityOptionsKeyValue, obsOptions)
	req, err := nethttp.NewRequestWithContext(ctx, nethttp.MethodPost, "https://example.com/test", bytes.NewReader(postBody))
	assert.NoError(t, err)

	handler := NewCompressionHandler()
	body415 := &trackingReadCloser{Reader: strings.NewReader("unsupported media type"), closed: false}
	resp415 := &nethttp.Response{
		StatusCode: 415,
		Header:     nethttp.Header{},
		Body:       body415,
	}

	body200 := &trackingReadCloser{Reader: strings.NewReader("success"), closed: false}
	resp200 := &nethttp.Response{
		StatusCode: 200,
		Header:     nethttp.Header{},
		Body:       body200,
	}

	callCount := 0
	pipeline := &testFuncPipeline{
		next: func(r *nethttp.Request, middlewareIndex int) (*nethttp.Response, error) {
			callCount++
			if callCount == 1 {
				assert.Equal(t, "gzip", r.Header.Get("Content-Encoding"))
				return resp415, nil
			}
			assert.Empty(t, r.Header.Get("Content-Encoding"))
			return resp200, nil
		},
	}

	resp, err := handler.Intercept(pipeline, 0, req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Assert that req (origReq) was updated consistently by 415 fallback
	assert.Empty(t, req.Header.Get("Content-Encoding"))
	assert.Equal(t, int64(len(postBody)), req.ContentLength)
	assert.NotNil(t, req.GetBody)
	replayed, err := req.GetBody()
	assert.NoError(t, err)
	defer replayed.Close()
	data, err := io.ReadAll(replayed)
	assert.NoError(t, err)
	assert.Equal(t, postBody, data)
}

func TestNativeClientRedirectWithCompression(t *testing.T) {
	for _, statusCode := range []int{307, 308} {
		t.Run(strconv.Itoa(statusCode), func(t *testing.T) {
			postBody := []byte("payload for testing native http client redirect with compression")
			firstReqReceived := false
			redirectReqReceived := false

			testServer := httptest.NewServer(nethttp.HandlerFunc(func(res nethttp.ResponseWriter, req *nethttp.Request) {
				if req.URL.Path == "/redirected" {
					redirectReqReceived = true
					assert.Equal(t, nethttp.MethodPost, req.Method)
					assert.Equal(t, "gzip", req.Header.Get("Content-Encoding"))

					rawBody, err := io.ReadAll(req.Body)
					assert.NoError(t, err)
					assert.NotEqual(t, postBody, rawBody, "redirected body must be compressed")

					gzReader, err := gzip.NewReader(bytes.NewReader(rawBody))
					assert.NoError(t, err)
					defer gzReader.Close()
					decompressed, err := io.ReadAll(gzReader)
					assert.NoError(t, err)
					assert.Equal(t, postBody, decompressed, "one decompression must yield original payload without double compression")

					res.WriteHeader(nethttp.StatusOK)
					res.Write([]byte("ok"))
					return
				}

				firstReqReceived = true
				assert.Equal(t, nethttp.MethodPost, req.Method)
				assert.Equal(t, "gzip", req.Header.Get("Content-Encoding"))
				res.Header().Set("Location", "/redirected")
				res.WriteHeader(statusCode)
			}))
			defer testServer.Close()

			transport := NewCustomTransport(NewCompressionHandler())
			client := &nethttp.Client{Transport: transport}

			req, err := nethttp.NewRequest(nethttp.MethodPost, testServer.URL, bytes.NewReader(postBody))
			assert.NoError(t, err)

			resp, err := client.Do(req)
			assert.NoError(t, err)
			assert.NotNil(t, resp)
			defer resp.Body.Close()

			assert.Equal(t, nethttp.StatusOK, resp.StatusCode)
			assert.True(t, firstReqReceived, "initial request must reach server")
			assert.True(t, redirectReqReceived, "redirected request must reach server")
		})
	}
}
