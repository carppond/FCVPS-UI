// Package bandwagon fetches per-service traffic figures from the BandwagonHost
// (64clouds) API, so an agent's monthly used/limit can be auto-populated from
// the provider instead of being measured/entered by hand.
//
// API: GET https://api.64clouds.com/v1/getServiceInfo?veid=<veid>&api_key=<key>
// Returns plan_monthly_data (allowance bytes), data_counter (used bytes), and
// data_next_reset (unix seconds). The api_key is a per-service secret.
package bandwagon

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// apiEndpoint is a var (not const) so tests can point it at an httptest server.
var apiEndpoint = "https://api.64clouds.com/v1/getServiceInfo"

// ServiceInfo is the subset of the getServiceInfo response we consume.
type ServiceInfo struct {
	PlanMonthlyData int64 // monthly allowance, bytes
	DataCounter     int64 // used this billing period, bytes
	DataNextReset   int64 // unix seconds of the next counter reset
}

type rawResponse struct {
	Error           int    `json:"error"`
	Message         string `json:"message"`
	PlanMonthlyData int64  `json:"plan_monthly_data"`
	DataCounter     int64  `json:"data_counter"`
	DataNextReset   int64  `json:"data_next_reset"`
}

// FetchServiceInfo queries the BandwagonHost API for a single service. The
// client should enforce a timeout; a safehttp client is recommended so the
// fixed-host request still cannot be redirected to a private address.
func FetchServiceInfo(ctx context.Context, client *http.Client, veid, apiKey string) (*ServiceInfo, error) {
	if veid == "" || apiKey == "" {
		return nil, fmt.Errorf("bandwagon: empty veid/api_key")
	}
	if client == nil {
		client = http.DefaultClient
	}
	q := url.Values{}
	q.Set("veid", veid)
	q.Set("api_key", apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiEndpoint+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("bandwagon: request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bandwagon: status %d", resp.StatusCode)
	}
	var raw rawResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&raw); err != nil {
		return nil, fmt.Errorf("bandwagon: decode: %w", err)
	}
	if raw.Error != 0 {
		return nil, fmt.Errorf("bandwagon: api error %d: %s", raw.Error, raw.Message)
	}
	return &ServiceInfo{
		PlanMonthlyData: raw.PlanMonthlyData,
		DataCounter:     raw.DataCounter,
		DataNextReset:   raw.DataNextReset,
	}, nil
}
