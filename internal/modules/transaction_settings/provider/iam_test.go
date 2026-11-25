package provider

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	iamhttp "gitea.tagsamurai.local/BETS-V2/service-contracts/services/iam/http"
)

func TestIAMApproverPresenceAnswersPerTransaction(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST "+pathApproverPresence, func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		var req iamhttp.ApproverPresenceRequest
		if err := decodeBody(r, &req); err != nil || len(req.Transactions) != 2 {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		envelope(w, iamhttp.ApproverPresenceResponse{"borrow": true, "assign": false})
	})
	iam := iamClient{client: testClient(serve(t, mux))}
	presence, err := iam.approverPresence(context.Background(), testClientID, []string{"borrow", "assign"})
	require.NoError(t, err)
	require.Equal(t, iamhttp.ApproverPresenceResponse{"borrow": true, "assign": false}, presence)
}

func TestIAMApproverPresenceSurfacesFailures(t *testing.T) {
	statusMux := http.NewServeMux()
	statusMux.HandleFunc("POST "+pathApproverPresence, func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "down", http.StatusInternalServerError)
	})
	_, err := iamClient{client: testClient(serve(t, statusMux))}.approverPresence(context.Background(), testClientID, nil)
	require.ErrorContains(t, err, "HTTP 500")

	noDataMux := http.NewServeMux()
	noDataMux.HandleFunc("POST "+pathApproverPresence, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"message":"ok"}`))
	})
	_, err = iamClient{client: testClient(serve(t, noDataMux))}.approverPresence(context.Background(), testClientID, nil)
	require.ErrorIs(t, err, errMissingData)
}

func TestIAMManagerContactReadsTheGroupContact(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /internal/fixed-asset-groups/{id}/manager-contact", func(w http.ResponseWriter, r *http.Request) {
		if !authorized(w, r) {
			return
		}
		if r.PathValue("id") == "404" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		envelope(w, iamhttp.ManagerContactResponse{Name: "Rina Wulandari", Role: "Group Manager"})
	})
	iam := iamClient{client: testClient(serve(t, mux))}
	contact, err := iam.managerContact(context.Background(), testClientID, 9)
	require.NoError(t, err)
	require.Equal(t, iamhttp.ManagerContactResponse{Name: "Rina Wulandari", Role: "Group Manager"}, contact)

	_, err = iam.managerContact(context.Background(), testClientID, 404)
	require.ErrorContains(t, err, "HTTP 404")

	noData := http.NewServeMux()
	noData.HandleFunc("GET /internal/fixed-asset-groups/{id}/manager-contact", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"message":"ok"}`))
	})
	_, err = iamClient{client: testClient(serve(t, noData))}.managerContact(context.Background(), testClientID, 9)
	require.ErrorIs(t, err, errMissingData)
}
