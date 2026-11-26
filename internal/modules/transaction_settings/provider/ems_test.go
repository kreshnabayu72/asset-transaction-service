package provider

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	emshttp "gitea.tagsamurai.local/BETS-V2/service-contracts/services/enterprise_management/http"
)

func TestEMSReadsDecodeTheirData(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+pathSubscriptions, func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		envelope(w, emshttp.SubscriptionsData{Subscriptions: []emshttp.SubscriptionItem{{ProductSlug: "fixed_asset", Status: "active"}}})
	})
	mux.HandleFunc("GET "+pathEffectiveCapacities, func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		if r.URL.Query().Get("product") != productFixedAsset {
			http.Error(w, "product required", http.StatusUnprocessableEntity)
			return
		}
		envelope(w, emshttp.EffectiveCapacitiesData{AddonLicenses: []emshttp.AddonLicense{{LicenseType: "audit", Total: 2}}})
	})
	ems := emsClient{client: testClient(serve(t, mux))}

	subs, err := ems.subscriptions(context.Background(), testClientID)
	require.NoError(t, err)
	require.Equal(t, "active", subs[0].Status)

	caps, err := ems.effectiveCapacities(context.Background(), testClientID)
	require.NoError(t, err)
	require.Equal(t, "audit", caps.AddonLicenses[0].LicenseType)
}

func TestEMSReadsSurfaceFailures(t *testing.T) {
	statusMux := http.NewServeMux()
	noDataMux := http.NewServeMux()
	for _, path := range []string{pathSubscriptions, pathEffectiveCapacities} {
		statusMux.HandleFunc("GET "+path, func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "down", http.StatusInternalServerError)
		})
		noDataMux.HandleFunc("GET "+path, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(`{"message":"ok"}`))
		})
	}
	down := emsClient{client: testClient(serve(t, statusMux))}
	_, err := down.subscriptions(context.Background(), testClientID)
	require.ErrorContains(t, err, "HTTP 500")
	_, err = down.effectiveCapacities(context.Background(), testClientID)
	require.ErrorContains(t, err, "HTTP 500")

	empty := emsClient{client: testClient(serve(t, noDataMux))}
	_, err = empty.subscriptions(context.Background(), testClientID)
	require.ErrorIs(t, err, errMissingData)
	_, err = empty.effectiveCapacities(context.Background(), testClientID)
	require.ErrorIs(t, err, errMissingData)
}
