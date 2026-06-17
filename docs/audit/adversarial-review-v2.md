# Adversarial Due Diligence Review — cost-dcm-provider

## Version & Date

Version: 2.0 | Date: 2026-06-17 | Reviewer: AI-assisted | Codebase: commit `5c2b5fa` (main)

Incremental review following v1.0 (commit `108fa2b`). One commit reviewed:
`5c2b5fa` ("Fix all findings from adversarial due diligence review").

## Executive Summary

The fix commit addresses **14 of 16** v1.0 findings fully. Two governance items
are **partially resolved** (CHANGELOG still missing; ADR set differs slightly
from the v1 recommendation).

The remediation is substantial and well-tested overall. However, several **new
correctness issues** were introduced alongside the fixes — most notably
**currency validation that runs after Koku source creation** (NEW-01), leaving
orphaned upstream resources and blocking retries with 409.

**Overall assessment:** Suitable for controlled deployment with the High-severity
regression fixed first. Core v1.0 risks (TLS, response limits, metrics, NATS
fallback, retries, context propagation) are materially improved.

**Strengths to acknowledge:**
- Clean TLS enforcement with localhost escape hatch (`KOKU_ALLOW_INSECURE`)
- Well-structured ADRs documenting key design decisions
- `doWithRetry()` with proper exponential backoff and 4xx exclusion
- NATS graceful degradation following kcli-SP pattern
- Context propagation throughout handler → Koku → HTTP chain
- `readLimited()` with configurable cap for response body safety
- Base64url opaque pagination tokens
- Strong SECURITY.md and CONTRIBUTING.md

## Scorecard

| Dimension | v1.0 | v2.0 | Key gap |
|-----------|------|------|---------|
| Security | ★★★☆☆ | ★★★★☆ | HTTPS enforcement added; SQLite perms advisory only |
| Correctness | ★★★★☆ | ★★★☆☆ | Currency validation order regression; retry orphans Koku sources |
| Auditability | ★★★★☆ | ★★★★☆ | Metrics wired; gauge init gap after restart |
| Operational | ★★★☆☆ | ★★★★☆ | NATS fallback, retries, context; delete cleanup gap |
| Performance | ★★★★☆ | ★★★★☆ | 10 MB limit addresses main risk |
| Design | ★★★★★ | ★★★★★ | Maintained |
| Maintainability | ★★★★☆ | ★★★★☆ | proxyReport extraction; compensating-txn gaps remain |
| Governance | ★★☆☆☆ | ★★★☆☆ | SECURITY, CONTRIBUTING, ADRs added; CHANGELOG and vuln scanning missing |

## v1.0 Findings — Resolution Status

### Fully Resolved (14 of 16)

| # | Title | Status |
|---|-------|--------|
| 1 | Koku identity over plaintext HTTP | **Resolved** — `validateKokuURL()` rejects non-HTTPS unless `KOKU_ALLOW_INSECURE=true` or localhost |
| 2 | Unbounded Koku response body | **Resolved** — `readLimited()` with 10 MB cap |
| 3 | Prometheus metrics not wired | **Resolved** — HTTP middleware, KokuRequestsTotal, NATSEventsPublished, InstancesManaged |
| 4 | Fatal NATS startup | **Resolved** — falls back to `NoopPublisher`; ADR-003 |
| 5 | Orphaned store row on Koku failure | **Resolved** — ERROR status + retry path for ERROR/DELETED targets |
| 6 | No Koku retry/backoff | **Resolved** — `doWithRetry()` with 3 attempts, exponential backoff; ADR-004 |
| 7 | Health probe new http.Client per call | **Resolved** — reusable client on `Checker` struct |
| 8 | No context propagation | **Resolved** — `http.NewRequestWithContext` throughout |
| 9 | Pagination token plain int | **Resolved** — base64url encoding |
| 10 | Silent cost model failure | **Resolved** — status message set on partial failure |
| 11 | Proxy handler duplication | **Resolved** — shared `proxyReport()` helper |
| 14 | Hard-delete vs soft-delete | **Resolved** — `Store.Delete()` removed; ADR-002 |
| 15 | VERSION file missing | **Resolved** — Containerfile ARG + Makefile ldflags |
| 16 | Currency hardcoded USD | **Resolved** — `spec.Currency` threaded through; *but see NEW-01 for validation order regression* |

### Partially Resolved (2 of 16)

| # | Title | Gap |
|---|-------|-----|
| 12 | No governance docs | SECURITY.md and CONTRIBUTING.md added; **no CHANGELOG.md** |
| 13 | No ADRs | Four ADRs added; missing "reconciler polling vs NATS-driven" ADR |

## New Findings (v2.0)

---

### NEW-01: Currency validation runs after Koku source creation

| Field | Value |
|-------|-------|
| **Severity** | High |
| **Dimension** | Correctness |
| **Location** | `internal/handler/handler.go:163-186` |
| **Description** | `CreateSource` is called before currency validation. If the currency is unsupported, the handler returns 400 but the Koku OCP source already exists. The store row remains in PROVISIONING without a saved `koku_source_uuid`. |
| **Risk** | Tenant submits invalid currency → gets 400 → retries with corrected spec → receives 409 Conflict (row still PROVISIONING). An orphaned Koku source accumulates per failed attempt. |
| **Recommendation** | Move currency (and cost-model spec) validation **before** `h.store.Create()` / `CreateSource`. On any post-reservation failure, mark the row ERROR (or delete it). |
| **Effort** | S |

---

### NEW-02: Retry after ERROR/DELETED orphans Koku sources

| Field | Value |
|-------|-------|
| **Severity** | Medium |
| **Dimension** | Correctness |
| **Location** | `internal/handler/handler.go:139-147,163` |
| **Description** | When retrying a failed or deleted target, the handler clears Koku UUIDs and calls `CreateSource` again without pausing or deleting the previous Koku source. |
| **Risk** | Each retry creates an additional active Koku OCP source for the same cluster, causing duplicate metering and operator confusion. |
| **Recommendation** | Before creating a new source on retry, pause/delete the existing Koku source if UUIDs were previously stored. Alternatively, reuse the existing source UUID. |
| **Effort** | M |

---

### NEW-03: `instances_managed` gauge not initialized at startup

| Field | Value |
|-------|-------|
| **Severity** | Low |
| **Dimension** | Auditability |
| **Location** | `internal/metrics/metrics.go:28-32`, `cmd/koku-cost-provider/main.go` |
| **Description** | The gauge is incremented/decremented on create/delete but never set from the database count at startup. After restart, `/metrics` reports 0 until the next operation. |
| **Risk** | Alerting on `instances_managed` misfires after restarts. |
| **Recommendation** | On startup, query active instance count and call `InstancesManaged.Set(count)`. |
| **Effort** | S |

---

### NEW-04: Invalid pagination token silently returns first page

| Field | Value |
|-------|-------|
| **Severity** | Low |
| **Dimension** | Correctness |
| **Location** | `internal/handler/handler.go:491-500` |
| **Description** | `decodePageToken` returns offset 0 on any decode/parse error instead of rejecting the request. |
| **Risk** | Corrupted tokens silently return the first page, causing duplicate processing in consumers. |
| **Recommendation** | Return 400 when the token cannot be decoded or is negative. |
| **Effort** | S |

---

### NEW-05: Delete returns 204 despite Koku cleanup failures

| Field | Value |
|-------|-------|
| **Severity** | Medium |
| **Dimension** | Correctness |
| **Location** | `internal/handler/handler.go:303-332` |
| **Description** | `DeleteCostModel` and `PauseSource` errors are logged but not propagated. The handler always returns 204 and marks the instance DELETED. |
| **Risk** | DCM believes the instance is deleted while the Koku source remains active and continues ingesting cost data. |
| **Recommendation** | Return 502 if Koku cleanup fails, leaving status unchanged. Or mark DELETING and let the reconciler retry cleanup. |
| **Effort** | M |

---

### NEW-06: Create returns 201 when store persistence fails after Koku provisioning

| Field | Value |
|-------|-------|
| **Severity** | Medium |
| **Dimension** | Correctness |
| **Location** | `internal/handler/handler.go:209-227` |
| **Description** | If `store.Update(inst)` fails after successful Koku source/cost-model creation, the error is logged but the handler still returns 201 with Koku UUIDs. |
| **Risk** | Client believes provisioning succeeded, but the store may lack Koku UUIDs. Reconciler cannot transition to READY; retries may create duplicate Koku resources. |
| **Recommendation** | Return 500 on store update failure. Consider compensating transaction (pause newly created Koku source). |
| **Effort** | S |

---

### NEW-07: Oversized Koku responses surface as generic 502

| Field | Value |
|-------|-------|
| **Severity** | Low |
| **Dimension** | Operational |
| **Location** | `internal/handler/handler.go:358-363`, `internal/koku/client.go:120` |
| **Description** | `ErrResponseTooLarge` is defined but never checked in handlers. All Koku proxy errors return a generic 502. |
| **Risk** | Operators cannot distinguish OOM-protection triggers from upstream outages. |
| **Recommendation** | Use `errors.Is(err, koku.ErrResponseTooLarge)` and return a specific error detail. |
| **Effort** | S |

---

### NEW-08: Proxy handlers silently ignore JSON unmarshal errors

| Field | Value |
|-------|-------|
| **Severity** | Low |
| **Dimension** | Correctness |
| **Location** | `internal/handler/handler.go:424-425,437-438` |
| **Description** | `_ = json.Unmarshal(data, &result)` discards errors. Malformed Koku JSON returns 200 with null or partial data. |
| **Risk** | Downstream consumers receive empty cost reports without error indication. |
| **Recommendation** | Check unmarshal error and return 502 on failure. |
| **Effort** | S |

---

### NEW-09: Reconciler provision timeout leaves active Koku source

| Field | Value |
|-------|-------|
| **Severity** | Low |
| **Dimension** | Operational |
| **Location** | `internal/reconciler/reconciler.go:94-103` |
| **Description** | On provision timeout, the reconciler marks the instance ERROR but does not pause the Koku source. |
| **Risk** | Orphaned active Koku sources continue collecting data for clusters DCM considers failed. |
| **Recommendation** | Call `PauseSource` when transitioning to ERROR on timeout. |
| **Effort** | S |

---

### NEW-10: No CHANGELOG.md

| Field | Value |
|-------|-------|
| **Severity** | Informational |
| **Dimension** | Governance |
| **Location** | Repository root |
| **Description** | v1.0 Finding #12 requested CHANGELOG. Two of three governance docs exist; release notes remain only in git history. |
| **Recommendation** | Add `CHANGELOG.md` following Keep a Changelog format. |
| **Effort** | S |

---

### NEW-11: No vulnerability scanning in CI

| Field | Value |
|-------|-------|
| **Severity** | Informational |
| **Dimension** | Governance |
| **Location** | `.github/workflows/ci.yaml` |
| **Description** | No `govulncheck`, Dependabot, or container image scanning. |
| **Recommendation** | Add a `govulncheck ./...` CI job and enable Dependabot for Go modules. |
| **Effort** | S |

---

### NEW-12: Container build defaults to dev version in CI

| Field | Value |
|-------|-------|
| **Severity** | Low |
| **Dimension** | Governance |
| **Location** | `Containerfile:10`, `.github/workflows/ci.yaml` |
| **Description** | Containerfile accepts `ARG VERSION` but CI does not pass `--build-arg VERSION=$(git describe)`. Default remains `0.0.1-dev`. |
| **Recommendation** | Pass `--build-arg VERSION=$(git describe --tags --always)` in CI/release pipeline. |
| **Effort** | S |

---

### NEW-13: SQLite database permissions not enforced at runtime

| Field | Value |
|-------|-------|
| **Severity** | Low |
| **Dimension** | Security |
| **Location** | `internal/store/store.go:27-28` |
| **Description** | SECURITY.md recommends `0600` permissions on the SQLite file, but the code doesn't enforce them. |
| **Recommendation** | After opening the DB, chmod the file to `0600`. |
| **Effort** | S |

---

### NEW-14: `prometheus/client_golang` listed as indirect dependency

| Field | Value |
|-------|-------|
| **Severity** | Informational |
| **Dimension** | Maintainability |
| **Location** | `go.mod` |
| **Description** | The project directly imports `prometheus/client_golang` but it may appear as indirect in `go.mod`. |
| **Recommendation** | Run `go mod tidy` to correct dependency annotations. |
| **Effort** | S |

## Priority Remediation Order

| Priority | Finding | Effort | Rationale |
|----------|---------|--------|-----------|
| 1 | NEW-01 | S | Blocks provisioning after invalid currency; creates orphaned Koku sources |
| 2 | NEW-06 | S | 201 on store failure causes split-brain |
| 3 | NEW-02 | M | Koku source accumulation on retry |
| 4 | NEW-05 | M | Silent delete failures leave active upstream resources |
| 5 | NEW-03 | S | False metrics after restart |
| 6 | NEW-04 | S | Pagination contract violation |
| 7 | NEW-07, NEW-08 | S | Better error surfaces for operators |
| 8 | NEW-09 | S | Timeout cleanup consistency |
| 9 | NEW-10–14 | S | Governance and hygiene |

## Current State

| Metric | Value |
|--------|-------|
| v1.0 findings resolved | 14 |
| v1.0 findings partially resolved | 2 |
| v1.0 findings still open | 0 |
| New findings (v2.0) | 14 |
| Critical | 0 |
| High | 1 (NEW-01) |
| Medium | 3 (NEW-02, NEW-05, NEW-06) |
| Low | 6 (NEW-03, NEW-04, NEW-07, NEW-08, NEW-09, NEW-12, NEW-13) |
| Informational | 3 (NEW-10, NEW-11, NEW-14) |
