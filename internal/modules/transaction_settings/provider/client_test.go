package provider

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"
)

// roundTripFunc adapts a function to http.RoundTripper, to simulate transport outcomes without a network.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

// failingBody is a response body whose read fails.
type failingBody struct{}

func (failingBody) Read([]byte) (int, error) { return 0, errors.New("connection reset") }

func (failingBody) Close() error { return nil }

func TestCallSendsTokenTenantQueryAndPayload(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /internal/thing", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		if r.URL.Query().Get("product") != "fixed_asset" || r.Header.Get("Content-Type") != "application/json" {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		envelope(w, map[string]string{"ok": "yes"})
	})
	raw, err := testClient(serve(t, mux)).call(context.Background(), http.MethodPost, "/internal/thing",
		testClientID, url.Values{"product": {"fixed_asset"}}, map[string]string{"a": "b"})
	require.NoError(t, err)
	require.JSONEq(t, `{"message":"ok","data":{"ok":"yes"}}`, string(raw))
}

func TestCallWithoutPayloadSendsNoBody(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /internal/thing", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		if r.Header.Get("Content-Type") != "" || r.URL.RawQuery != "" {
			http.Error(w, "unexpected", http.StatusBadRequest)
			return
		}
		envelope(w, []int{1})
	})
	_, err := testClient(serve(t, mux)).call(context.Background(), http.MethodGet, "/internal/thing", testClientID, nil, nil)
	require.NoError(t, err)
}

func TestCallReportsNonSuccessStatus(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /internal/thing", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	})
	_, err := testClient(serve(t, mux)).call(context.Background(), http.MethodGet, "/internal/thing", testClientID, nil, nil)
	var status *statusError
	require.ErrorAs(t, err, &status)
	require.Equal(t, http.StatusNotFound, status.status)
	require.ErrorContains(t, err, "HTTP 404")
}

func TestCallReportsTransportFailure(t *testing.T) {
	c := testClient("http://owner.invalid")
	c.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("dial refused")
	})
	_, err := c.call(context.Background(), http.MethodGet, "/x", testClientID, nil, nil)
	require.ErrorContains(t, err, "dial refused")
}

func TestCallReportsBodyReadFailure(t *testing.T) {
	c := testClient("http://owner.invalid")
	c.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: failingBody{}, Header: make(http.Header)}, nil
	})
	_, err := c.call(context.Background(), http.MethodGet, "/x", testClientID, nil, nil)
	require.ErrorContains(t, err, "connection reset")
}

func TestCallRejectsUnencodablePayload(t *testing.T) {
	_, err := testClient("http://owner.invalid").call(context.Background(), http.MethodPost, "/x", testClientID, nil, make(chan int))
	require.ErrorContains(t, err, "encode")
}

func TestCallRejectsMalformedURL(t *testing.T) {
	_, err := testClient("http://\x7f").call(context.Background(), http.MethodGet, "/x", testClientID, nil, nil)
	require.ErrorContains(t, err, "build")
}

func TestCallFailsWhenServerIsGone(t *testing.T) {
	_, err := testClient(closedURL(t)).call(context.Background(), http.MethodGet, "/x", testClientID, nil, nil)
	require.ErrorContains(t, err, "call owner /x")
}

func TestNewInternalClientTrimsTrailingSlash(t *testing.T) {
	require.Equal(t, "http://owner", newInternalClient("owner", "http://owner/", testToken).baseURL)
}

func TestDecodeDataOutcomes(t *testing.T) {
	var out map[string]string
	require.ErrorIs(t, decodeData([]byte(`{"message":"ok"}`), &out), errMissingData)
	require.NoError(t, decodeData([]byte(`{"data":{"a":"b"}}`), &out))
	require.Equal(t, "b", out["a"])
	require.NoError(t, decodeData([]byte(`{"data":null}`), &out))
	require.ErrorContains(t, decodeData([]byte(`not json`), &out), "decode envelope")
	require.ErrorContains(t, decodeData([]byte(`{"data":[1]}`), &out), "decode data")
}

func TestChunkIDsSplitsAtTheLookupCap(t *testing.T) {
	require.Empty(t, chunkIDs(nil))
	require.Len(t, chunkIDs([]int64{1}), 1)
	require.Len(t, chunkIDs(make([]int64, maxLookupIDs)), 1)
	chunks := chunkIDs(make([]int64, maxLookupIDs+1))
	require.Len(t, chunks, 2)
	require.Len(t, chunks[1], 1)
}

func TestIDHelpers(t *testing.T) {
	id, err := parseID("42")
	require.NoError(t, err)
	require.Equal(t, int64(42), id)
	_, err = parseID("x")
	require.ErrorIs(t, err, errContract)

	none, err := parseIDPtr(nil)
	require.NoError(t, err)
	require.Nil(t, none)
	_, err = parseIDPtr(ptr("y"))
	require.ErrorIs(t, err, errContract)
	got, err := parseIDPtr(ptr("7"))
	require.NoError(t, err)
	require.Equal(t, int64(7), *got)

	require.Equal(t, []string{"1", "2"}, idStrings([]int64{1, 2}))
	require.Empty(t, derefString(nil))
	require.Equal(t, "s", derefString(ptr("s")))
	require.False(t, derefBool(nil))
	require.True(t, derefBool(ptr(true)))

	byID, err := indexByID([]string{"3", "4"}, func(s string) string { return s })
	require.NoError(t, err)
	require.Equal(t, "4", byID[4])
	_, err = indexByID([]string{"z"}, func(s string) string { return s })
	require.ErrorIs(t, err, errContract)
	require.Equal(t, reflect.Struct, reflect.TypeOf(internalClient{}).Kind())
}
