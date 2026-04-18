// Package koku provides a client for the Koku (Red Hat Lightspeed Cost
// Management) REST API: sources, cost models, reports, and forecasts.
package koku

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Client wraps the Koku REST API.
type Client struct {
	baseURL    string
	identity   string // base64-encoded x-rh-identity header
	httpClient *http.Client
}

func NewClient(baseURL, identity string) *Client {
	return &Client{
		baseURL:  baseURL,
		identity: identity,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) do(method, path string, body any) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-rh-identity", c.identity)
	return c.httpClient.Do(req)
}

func decodeJSON[T any](resp *http.Response) (*T, error) {
	defer resp.Body.Close() //nolint:errcheck // response body
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &v, nil
}

func (c *Client) CreateSource(clusterID, name string) (*SourceResponse, error) {
	payload := SourceRequest{
		Name:       name,
		SourceType: "OCP",
		Authentication: &Authentication{
			Credentials: &Credentials{ClusterID: clusterID},
		},
		BillingSource: &BillingSource{Bucket: ""},
	}
	resp, err := c.do(http.MethodPost, "/api/cost-management/v1/sources/", payload)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusCreated {
		defer resp.Body.Close() //nolint:errcheck // error path
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create source failed: status %d: %s", resp.StatusCode, truncate(body))
	}
	return decodeJSON[SourceResponse](resp)
}

func (c *Client) PauseSource(uuid string) error {
	resp, err := c.do(http.MethodPatch, "/api/cost-management/v1/sources/"+uuid+"/", map[string]any{"paused": true})
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // response body
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pause source failed: status %d: %s", resp.StatusCode, truncate(body))
	}
	return nil
}

func (c *Client) CreateCostModel(name string, sourceUUID string, rates []CostModelRate, markup *CostModelMarkup, distribution string) (*CostModelResponse, error) {
	payload := CostModelRequest{
		Name:         name,
		Description:  "DCM-managed cost model",
		SourceType:   "OCP",
		SourceUUIDs:  []string{sourceUUID},
		Rates:        rates,
		Markup:       markup,
		Distribution: distribution,
	}
	resp, err := c.do(http.MethodPost, "/api/cost-management/v1/cost-models/", payload)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusCreated {
		defer resp.Body.Close() //nolint:errcheck // error path
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("create cost model failed: status %d: %s", resp.StatusCode, truncate(body))
	}
	return decodeJSON[CostModelResponse](resp)
}

func (c *Client) DeleteCostModel(uuid string) error {
	resp, err := c.do(http.MethodDelete, "/api/cost-management/v1/cost-models/"+uuid+"/", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // response body
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete cost model failed: status %d: %s", resp.StatusCode, truncate(body))
	}
	return nil
}

func (c *Client) GetSourceStats(uuid string) (SourceStatsResponse, error) {
	resp, err := c.do(http.MethodGet, "/api/cost-management/v1/sources/"+uuid+"/stats/", nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close() //nolint:errcheck // error path
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get source stats failed: status %d: %s", resp.StatusCode, truncate(body))
	}
	result, err := decodeJSON[SourceStatsResponse](resp)
	if err != nil {
		return nil, err
	}
	return *result, nil
}

// GetReports proxies a report request. Returns raw JSON bytes.
func (c *Client) GetReports(clusterID, reportType string, params url.Values) (json.RawMessage, error) {
	kokuPath := reportTypeToPath(reportType)
	if params == nil {
		params = url.Values{}
	}
	params.Set("filter[cluster]", clusterID)
	fullPath := fmt.Sprintf("/api/cost-management/v1/reports/openshift/%s/?%s", kokuPath, params.Encode())

	resp, err := c.do(http.MethodGet, fullPath, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck // response body
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get reports failed: status %d: %s", resp.StatusCode, truncate(body))
	}
	return io.ReadAll(resp.Body)
}

// GetForecasts proxies a forecast request. Returns raw JSON bytes.
func (c *Client) GetForecasts(clusterID string, params url.Values) (json.RawMessage, error) {
	if params == nil {
		params = url.Values{}
	}
	params.Set("filter[cluster]", clusterID)
	fullPath := fmt.Sprintf("/api/cost-management/v1/forecasts/openshift/costs/?%s", params.Encode())

	resp, err := c.do(http.MethodGet, fullPath, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() //nolint:errcheck // response body
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get forecasts failed: status %d: %s", resp.StatusCode, truncate(body))
	}
	return io.ReadAll(resp.Body)
}

func reportTypeToPath(metric string) string {
	switch metric {
	case "compute":
		return "compute"
	case "memory":
		return "memory"
	case "storage":
		return "volumes"
	default:
		return "costs"
	}
}

func truncate(b []byte) string {
	const max = 200
	if len(b) > max {
		return string(b[:max])
	}
	return string(b)
}
