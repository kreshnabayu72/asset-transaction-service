package provider

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	emshttp "gitea.tagsamurai.local/BETS-V2/service-contracts/services/enterprise_management/http"
	iamhttp "gitea.tagsamurai.local/BETS-V2/service-contracts/services/iam/http"
	marhttp "gitea.tagsamurai.local/BETS-V2/service-contracts/services/master_asset_registry/http"
)

const (
	testToken    = "token-1"
	testClientID = "client-1"
)

// serve starts a fake owner service for mux and closes it with the test.
func serve(t *testing.T, mux *http.ServeMux) string {
	t.Helper()
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server.URL
}

// closedURL is the base URL of a server that is already gone, so every call fails to connect.
func closedURL(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.NotFoundHandler())
	server.Close()
	return server.URL
}

// authorized rejects a request whose service token or tenant header is not the test pair.
func authorized(w http.ResponseWriter, r *http.Request) bool {
	if r.Header.Get("X-Service-Token") != testToken || r.Header.Get("X-Client-ID") != testClientID {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return false
	}
	return true
}

// envelope writes a helper.Success style body with data as its data member.
func envelope(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"message": "ok", "data": data})
}

// decodeBody decodes a JSON request body into out.
func decodeBody(r *http.Request, out any) error {
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, out)
}

// testClient builds an internal client for the test token against baseURL.
func testClient(baseURL string) internalClient {
	return newInternalClient("owner", baseURL, testToken)
}

// ptr returns a pointer to v.
func ptr[T any](v T) *T {
	return &v
}

// owners is a fake of the three owner services with canned answers and failure switches. Canned lists are
// returned whole; the directories pick the ids they asked for.
type owners struct {
	assets      []marhttp.AssetResolveItem
	assetNames  []marhttp.AssetNameLookupItem
	categories  []marhttp.CategoryLookupItem
	groups      []emshttp.GroupItem
	contact     iamhttp.ManagerContactResponse
	failMAR     bool
	failIAM     bool
	groupCalls  atomic.Int32
	lastContact atomic.Int64
}

// clients starts the fake owners and returns the three internal clients pointing at them.
func (o *owners) clients(t *testing.T) (marClient, emsClient, iamClient) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+pathCategoryLookup, func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		if o.failMAR {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		envelope(w, marhttp.CategoryLookupData{Results: o.categories})
	})
	mux.HandleFunc("POST "+pathAssetResolve, func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		if o.failMAR {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		envelope(w, marhttp.AssetResolveData{Results: o.assets})
	})
	mux.HandleFunc("POST "+pathAssetNameLookup, func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		if o.failMAR {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		envelope(w, marhttp.AssetNameLookupData{Results: o.assetNames})
	})
	mux.HandleFunc("GET /internal/fixed-asset-groups", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		o.groupCalls.Add(1)
		if o.failIAM {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		envelope(w, o.groups)
	})
	mux.HandleFunc("GET /internal/fixed-asset-groups/{id}/manager-contact", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		if o.failIAM {
			http.Error(w, "down", http.StatusInternalServerError)
			return
		}
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			http.Error(w, "bad id", http.StatusBadRequest)
			return
		}
		o.lastContact.Store(id)
		envelope(w, o.contact)
	})
	base := serve(t, mux)
	return marClient{client: testClient(base)}, emsClient{client: testClient(base)}, iamClient{client: testClient(base)}
}
