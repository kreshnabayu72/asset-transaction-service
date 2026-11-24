package provider

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	marhttp "gitea.tagsamurai.local/BETS-V2/service-contracts/services/master_asset_registry/http"
)

func TestMARCategoriesChunkRequestsAtTheLookupCap(t *testing.T) {
	var requests atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+pathCategoryLookup, func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		requests.Add(1)
		var req marhttp.CategoryLookupRequest
		if err := decodeBody(r, &req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		results := make([]marhttp.CategoryLookupItem, 0, len(req.CategoryIDs))
		for _, id := range req.CategoryIDs {
			results = append(results, marhttp.CategoryLookupItem{ID: id, Found: true})
		}
		envelope(w, marhttp.CategoryLookupData{Results: results})
	})
	ids := make([]int64, maxLookupIDs+1)
	for i := range ids {
		ids[i] = int64(i + 1)
	}
	items, err := marClient{client: testClient(serve(t, mux))}.categories(context.Background(), testClientID, ids)
	require.NoError(t, err)
	require.Len(t, items, len(ids))
	require.EqualValues(t, 2, requests.Load())
}

func TestMARLookupsSurfaceStatusAndMissingData(t *testing.T) {
	statusMux := http.NewServeMux()
	noDataMux := http.NewServeMux()
	for _, path := range []string{pathCategoryLookup, pathAssetResolve, pathAssetNameLookup} {
		statusMux.HandleFunc("POST "+path, func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "down", http.StatusInternalServerError)
		})
		noDataMux.HandleFunc("POST "+path, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"message":"ok"}`))
		})
	}
	ids := []int64{1}

	down := marClient{client: testClient(serve(t, statusMux))}
	_, err := down.categories(context.Background(), testClientID, ids)
	require.ErrorContains(t, err, "HTTP 500")
	_, err = down.assets(context.Background(), testClientID, ids)
	require.ErrorContains(t, err, "HTTP 500")
	_, err = down.assetNames(context.Background(), testClientID, ids)
	require.ErrorContains(t, err, "HTTP 500")

	empty := marClient{client: testClient(serve(t, noDataMux))}
	_, err = empty.categories(context.Background(), testClientID, ids)
	require.ErrorIs(t, err, errMissingData)
	_, err = empty.assets(context.Background(), testClientID, ids)
	require.ErrorIs(t, err, errMissingData)
	_, err = empty.assetNames(context.Background(), testClientID, ids)
	require.ErrorIs(t, err, errMissingData)
}

func TestMARLookupsSkipTheCallWithoutIDs(t *testing.T) {
	var calls atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("POST /", func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		http.Error(w, "unexpected", http.StatusInternalServerError)
	})
	m := marClient{client: testClient(serve(t, mux))}
	items, err := m.assets(context.Background(), testClientID, nil)
	require.NoError(t, err)
	require.Empty(t, items)
	require.Zero(t, calls.Load())
}
