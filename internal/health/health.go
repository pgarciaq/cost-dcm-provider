// Package health provides a cached health checker that probes the Koku API.
package health

import (
	"net/http"
	"sync"
	"time"

	"github.com/dcm-project/koku-cost-provider/internal/util"
)

// Checker implements the health check for the cost service provider.
type Checker struct {
	version   string
	startTime time.Time
	kokuURL   string
	cacheTTL  time.Duration

	mu           sync.RWMutex
	cachedAt     time.Time
	cachedKokuOK bool
}

func NewChecker(version string, startTime time.Time, kokuURL string) *Checker {
	return &Checker{
		version:   version,
		startTime: startTime,
		kokuURL:   kokuURL,
		cacheTTL:  10 * time.Second,
	}
}

// HealthResult matches the generated Health schema.
type HealthResult struct {
	Type    *string `json:"type"`
	Status  *string `json:"status"`
	Path    *string `json:"path"`
	Version *string `json:"version"`
	Uptime  *int    `json:"uptime"`
}

func (c *Checker) Check() HealthResult {
	uptime := max(0, int(time.Since(c.startTime).Seconds()))
	status := "healthy"

	if !c.kokuHealthy() {
		status = "unhealthy"
	}

	return HealthResult{
		Type:    util.Ptr("koku-cost-provider.dcm.io/health"),
		Status:  &status,
		Path:    util.Ptr("health"),
		Version: &c.version,
		Uptime:  &uptime,
	}
}

func (c *Checker) kokuHealthy() bool {
	c.mu.RLock()
	if time.Since(c.cachedAt) < c.cacheTTL {
		ok := c.cachedKokuOK
		c.mu.RUnlock()
		return ok
	}
	c.mu.RUnlock()

	ok := c.probeKoku()

	c.mu.Lock()
	c.cachedAt = time.Now()
	c.cachedKokuOK = ok
	c.mu.Unlock()

	return ok
}

func (c *Checker) probeKoku() bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(c.kokuURL + "/api/cost-management/v1/status/")
	if err != nil {
		return false
	}
	defer resp.Body.Close() //nolint:errcheck // best-effort health probe
	return resp.StatusCode < 500
}
