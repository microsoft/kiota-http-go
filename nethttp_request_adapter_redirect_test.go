package nethttplibrary

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	abs "github.com/microsoft/kiota-abstractions-go"
	absauth "github.com/microsoft/kiota-abstractions-go/authentication"
	"github.com/microsoft/kiota-http-go/internal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmptyRedirectResponsesWithoutLocation(t *testing.T) {
	for _, status := range []int{http.StatusFound, http.StatusNotModified} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// A 304 has no body; a 302 without Location must not be followed.
				w.WriteHeader(status)
			}))
			defer server.Close()

			adapter, err := NewNetHttpRequestAdapter(&absauth.AnonymousAuthenticationProvider{})
			require.NoError(t, err)
			uri, err := url.Parse(server.URL)
			require.NoError(t, err)
			request := abs.NewRequestInformation()
			request.SetUri(*uri)
			request.Method = abs.GET
			ctx := context.Background()

			for _, test := range []struct {
				name string
				send func() (any, error)
			}{
				{"model", func() (any, error) {
					return adapter.Send(ctx, request, internal.MockEntityFactory, nil)
				}},
				{"model collection", func() (any, error) {
					return adapter.SendCollection(ctx, request, internal.MockEntityFactory, nil)
				}},
				{"enum", func() (any, error) {
					return adapter.SendEnum(ctx, request, nil, nil)
				}},
				{"enum collection", func() (any, error) {
					return adapter.SendEnumCollection(ctx, request, nil, nil)
				}},
				{"primitive", func() (any, error) {
					return adapter.SendPrimitive(ctx, request, "string", nil)
				}},
				{"primitive collection", func() (any, error) {
					return adapter.SendPrimitiveCollection(ctx, request, "string", nil)
				}},
				{"bytes", func() (any, error) {
					return adapter.SendPrimitive(ctx, request, "[]byte", nil)
				}},
				{"no content", func() (any, error) {
					return nil, adapter.SendNoContent(ctx, request, nil)
				}},
			} {
				t.Run(test.name, func(t *testing.T) {
					result, err := test.send()
					require.NoError(t, err)
					assert.Nil(t, result)
				})
			}
		})
	}
}
