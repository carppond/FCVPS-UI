package bandwagon

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func withEndpoint(t *testing.T, url string) {
	t.Helper()
	old := apiEndpoint
	apiEndpoint = url
	t.Cleanup(func() { apiEndpoint = old })
}

func TestFetchServiceInfo(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("veid") != "123" || r.URL.Query().Get("api_key") != "secret" {
			http.Error(w, "bad creds", http.StatusForbidden)
			return
		}
		_, _ = w.Write([]byte(`{"error":0,"plan_monthly_data":2199023255552,` +
			`"data_counter":1099511627776,"data_next_reset":1717200000}`))
	}))
	defer srv.Close()
	withEndpoint(t, srv.URL)

	info, err := FetchServiceInfo(context.Background(), srv.Client(), "123", "secret")
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if info.PlanMonthlyData != 2199023255552 || info.DataCounter != 1099511627776 ||
		info.DataNextReset != 1717200000 {
		t.Fatalf("unexpected: %+v", info)
	}
}

func TestFetchServiceInfoAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"error":1,"message":"invalid api_key"}`))
	}))
	defer srv.Close()
	withEndpoint(t, srv.URL)

	if _, err := FetchServiceInfo(context.Background(), srv.Client(), "1", "bad"); err == nil {
		t.Fatal("expected error on api error=1")
	}
}

func TestFetchServiceInfoEmptyCreds(t *testing.T) {
	if _, err := FetchServiceInfo(context.Background(), nil, "", ""); err == nil {
		t.Fatal("expected error for empty creds")
	}
}
