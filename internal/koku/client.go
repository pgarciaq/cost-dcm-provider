// Package koku provides a client for the Koku (Red Hat Lightspeed Cost
// Management) REST API: sources, cost models, reports, and forecasts.
package koku

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"time"

	"github.com/dcm-project/koku-cost-provider/internal/metrics"
)

const (
	maxResponseBytes = 10 * 1024 * 1024 // 10 MB
	maxRetries       = 3
	initialBackoff   = 1 * time.Second
)

// BackoffFunc is called between retry attempts. The default sleeps for the
// given duration, but tests can replace it with a no-op.
type BackoffFunc func(ctx context.Context, d time.Duration)

// Client wraps the Koku REST API.
type Client struct {
	baseURL    string
	identity   string // base64-encoded x-rh-identity header
	httpClient *http.Client
	backoffFn  BackoffFunc
}

func NewClient(baseURL, identity string) *Client {
	return &Client{
		baseURL:  baseURL,
		identity: identity,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		backoffFn: defaultBackoff,
	}
}

func defaultBackoff(ctx context.Context, d time.Duration) {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

func (c *Client) do(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, reqBody)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-rh-identity", c.identity)

	resp, err := c.httpClient.Do(req)
	op := metrics.NormalizeKokuOp(method, path)
	if err != nil {
		metrics.KokuRequestsTotal.WithLabelValues(op, "error").Inc()
		return nil, err
	}
	result := "success"
	if resp.StatusCode >= 400 {
		result = "client_error"
	}
	if resp.StatusCode >= 500 {
		result = "server_error"
	}
	metrics.KokuRequestsTotal.WithLabelValues(op, result).Inc()
	return resp, nil
}

// doWithRetry retries on 5xx and transient errors with exponential backoff.
func (c *Client) doWithRetry(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var lastErr error
	for attempt := range maxRetries {
		resp, err := c.do(ctx, method, path, body)
		if err != nil {
			lastErr = err
			if ctx.Err() != nil {
				return nil, err
			}
			c.backoff(ctx, attempt)
			continue
		}
		if resp.StatusCode >= 500 {
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("server error: status %d", resp.StatusCode)
			c.backoff(ctx, attempt)
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("all %d attempts failed: %w", maxRetries, lastErr)
}

func (c *Client) backoff(ctx context.Context, attempt int) {
	d := time.Duration(float64(initialBackoff) * math.Pow(2, float64(attempt)))
	c.backoffFn(ctx, d)
}

var ErrResponseTooLarge = fmt.Errorf("response body exceeded %d bytes limit", maxResponseBytes)

func readLimited(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close() //nolint:errcheck // response body
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxResponseBytes {
		return nil, ErrResponseTooLarge
	}
	return data, nil
}

func decodeJSON[T any](resp *http.Response) (*T, error) {
	defer resp.Body.Close() //nolint:errcheck // response body
	r := io.LimitReader(resp.Body, maxResponseBytes)
	var v T
	if err := json.NewDecoder(r).Decode(&v); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	return &v, nil
}

func (c *Client) CreateSource(ctx context.Context, clusterID, name string) (*SourceResponse, error) {
	payload := SourceRequest{
		Name:       name,
		SourceType: "OCP",
		Authentication: &Authentication{
			Credentials: &Credentials{ClusterID: clusterID},
		},
		BillingSource: &BillingSource{Bucket: ""},
	}
	resp, err := c.doWithRetry(ctx, http.MethodPost, "/api/cost-management/v1/sources/", payload)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusCreated {
		defer resp.Body.Close() //nolint:errcheck // error path
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("create source failed: status %d: %s", resp.StatusCode, truncate(body))
	}
	return decodeJSON[SourceResponse](resp)
}

func (c *Client) PauseSource(ctx context.Context, uuid string) error {
	resp, err := c.doWithRetry(ctx, http.MethodPatch, "/api/cost-management/v1/sources/"+uuid+"/", map[string]any{"paused": true})
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // response body
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("pause source failed: status %d: %s", resp.StatusCode, truncate(body))
	}
	return nil
}

func (c *Client) CreateCostModel(ctx context.Context, name string, sourceUUID string, rates []CostModelRate, markup *CostModelMarkup, distribution string) (*CostModelResponse, error) {
	payload := CostModelRequest{
		Name:         name,
		Description:  "DCM-managed cost model",
		SourceType:   "OCP",
		SourceUUIDs:  []string{sourceUUID},
		Rates:        rates,
		Markup:       markup,
		Distribution: distribution,
	}
	resp, err := c.doWithRetry(ctx, http.MethodPost, "/api/cost-management/v1/cost-models/", payload)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusCreated {
		defer resp.Body.Close() //nolint:errcheck // error path
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("create cost model failed: status %d: %s", resp.StatusCode, truncate(body))
	}
	return decodeJSON[CostModelResponse](resp)
}

func (c *Client) DeleteCostModel(ctx context.Context, uuid string) error {
	resp, err := c.doWithRetry(ctx, http.MethodDelete, "/api/cost-management/v1/cost-models/"+uuid+"/", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close() //nolint:errcheck // response body
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("delete cost model failed: status %d: %s", resp.StatusCode, truncate(body))
	}
	return nil
}

func (c *Client) GetSourceStats(ctx context.Context, uuid string) (SourceStatsResponse, error) {
	resp, err := c.doWithRetry(ctx, http.MethodGet, "/api/cost-management/v1/sources/"+uuid+"/stats/", nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close() //nolint:errcheck // error path
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("get source stats failed: status %d: %s", resp.StatusCode, truncate(body))
	}
	result, err := decodeJSON[SourceStatsResponse](resp)
	if err != nil {
		return nil, err
	}
	return *result, nil
}

// GetReports proxies a report request. Returns raw JSON bytes (limited to 10 MB).
func (c *Client) GetReports(ctx context.Context, clusterID, reportType string, params url.Values) (json.RawMessage, error) {
	kokuPath := reportTypeToPath(reportType)
	if params == nil {
		params = url.Values{}
	}
	params.Set("filter[cluster]", clusterID)
	fullPath := fmt.Sprintf("/api/cost-management/v1/reports/openshift/%s/?%s", kokuPath, params.Encode())

	resp, err := c.doWithRetry(ctx, http.MethodGet, fullPath, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close() //nolint:errcheck // error path
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("get reports failed: status %d: %s", resp.StatusCode, truncate(body))
	}
	return readLimited(resp)
}

// GetForecasts proxies a forecast request. Returns raw JSON bytes (limited to 10 MB).
func (c *Client) GetForecasts(ctx context.Context, clusterID string, params url.Values) (json.RawMessage, error) {
	if params == nil {
		params = url.Values{}
	}
	params.Set("filter[cluster]", clusterID)
	fullPath := fmt.Sprintf("/api/cost-management/v1/forecasts/openshift/costs/?%s", params.Encode())

	resp, err := c.doWithRetry(ctx, http.MethodGet, fullPath, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close() //nolint:errcheck // error path
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("get forecasts failed: status %d: %s", resp.StatusCode, truncate(body))
	}
	return readLimited(resp)
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
