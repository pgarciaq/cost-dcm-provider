# Adversarial Due Diligence Review — cost-dcm-provider

## Version & Date
Version: 1.0 | Date: 2026-06-16 | Reviewer: AI-assisted | Commit: 108fa2b

## Executive Summary

The `cost-dcm-provider` is a ~3,000 line (hand-written) Go microservice that
integrates Red Hat Lightspeed Cost Management (Koku) with the DCM control
plane. It implements the standard DCM service provider contract: registration,
CRUD lifecycle, NATS CloudEvent publishing, and health checks.

**Overall assessment: Solid for a prototype.** The codebase has clearly
benefited from a prior due diligence pass (commit a0cf883) that addressed
critical issues including atomicity of store reservation, idempotent delete,
secret handling, and body size limits. The code is clean, well-structured, and
has reasonable test coverage for its size.

**Key risks remaining:** (1) The Koku client sends the `x-rh-identity` header
over HTTP with no TLS enforcement, making credential interception trivial in
non-localhost deployments. (2) Response bodies from Koku are proxied verbatim
without size limits, creating an unbounded memory risk. (3) The NATS publisher
hard-fails the startup if NATS is unreachable, making the SP fragile to
transient infrastructure issues. (4) Prometheus metrics are defined but never
instrumented (counters/histograms never incremented). (5) Several governance
gaps exist (no CHANGELOG, SECURITY.md, CONTRIBUTING.md, or ADRs).

**Strengths to acknowledge:**
- Clean separation of concerns (handler / store / koku / reconciler / monitoring)
- Interface-based design enables testability (`InstanceStore`, `KokuClient`)
- RFC 7807 error responses throughout
- OpenAPI-first with generated server/client code and CI-enforced sync
- Prior due diligence fixes are well-documented in code comments
- Path traversal protection on `resource_id` input
- WAL mode and busy timeout on SQLite

## Scorecard

| Dimension | Rating | Key gap |
|-----------|--------|---------|
| Security | ★★★☆☆ | No TLS enforcement for Koku credentials; unbounded proxy response body |
| Correctness | ★★★★☆ | Create returns 500 but leaves orphaned store row on Koku source failure |
| Auditability | ★★★★☆ | Good structured logging; Prometheus metrics defined but never wired |
| Operational robustness | ★★★☆☆ | Hard NATS failure at startup; no circuit breaker on Koku calls; no retry on Koku 5xx |
| Performance | ★★★★☆ | Adequate for expected load; unbounded response proxy is the main risk |
| Design quality | ★★★★★ | Clean package layout, interface-based DI, OpenAPI-first |
| Maintainability | ★★★★☆ | No dead code, clear naming, decent test coverage; some proxy handlers near-identical |
| Governance | ★★☆☆☆ | No CHANGELOG, SECURITY.md, CONTRIBUTING, ADRs, or vulnerability scanning |

## Findings Status Summary

| # | Title | Severity | Dimension | Status |
|---|-------|----------|-----------|--------|
| 1 | Koku identity sent over plaintext HTTP | High | Security | Open |
| 2 | Unbounded Koku response body proxied into memory | High | Security / Performance | Open |
| 3 | Prometheus metrics defined but never instrumented | Medium | Auditability | Open |
| 4 | NATS connection failure is fatal at startup | Medium | Operational | Open |
| 5 | Failed Koku source creation leaves orphaned store row in ERROR | Medium | Correctness | Open |
| 6 | No retry/backoff on Koku API calls | Medium | Operational | Open |
| 7 | Health probe creates new http.Client on every call | Low | Performance | Open |
| 8 | Koku client has no request context propagation | Medium | Correctness | Open |
| 9 | Pagination token is a plain integer offset | Low | Security | Open |
| 10 | Cost model creation failure silently continues | Low | Correctness | Open |
| 11 | Duplicate code across proxy handlers | Low | Maintainability | Open |
| 12 | No CHANGELOG, SECURITY.md, CONTRIBUTING.md | Informational | Governance | Open |
| 13 | No ADRs (Architecture Decision Records) | Informational | Governance | Open |
| 14 | `Store.Delete` hard-deletes but handler uses soft-delete | Low | Correctness | Open |
| 15 | `Containerfile` references `VERSION` file that doesn't exist | Low | Governance | Open |
| 16 | Currency hardcoded to USD in rate conversion | Low | Correctness | Open |

## Findings Detail

### Finding 1: Koku identity sent over plaintext HTTP

| Field | Value |
|-------|-------|
| **Severity** | High |
| **Dimension** | Security |
| **Location** | `internal/koku/client.go:47` |
| **Description** | The Koku client sends `x-rh-identity` (a base64-encoded identity header containing account/org information) on every request. There is no validation that `KOKU_API_URL` uses HTTPS. In non-localhost deployments, this credential is transmitted in cleartext. |
| **Risk** | Network-level attacker between the SP and Koku can intercept the identity header and impersonate the SP against Koku, creating/deleting sources and cost models at will. |
| **Recommendation** | Add a startup validation in `config.Load()` that rejects `KOKU_API_URL` values that don't start with `https://` unless an explicit `KOKU_ALLOW_INSECURE=true` env var is set. Log a warning when insecure mode is used. |
| **Effort** | S (hours) |

---

### Finding 2: Unbounded Koku response body proxied into memory

| Field | Value |
|-------|-------|
| **Severity** | High |
| **Dimension** | Security / Performance |
| **Location** | `internal/koku/client.go:164,184` (`GetReports`, `GetForecasts`) |
| **Description** | `io.ReadAll(resp.Body)` reads the entire Koku response into memory without any size limit. Koku report responses can be multi-megabyte JSON blobs for large clusters with months of data. |
| **Risk** | A legitimate or malicious query for a large date range could cause OOM on the SP, which runs as a single-process binary with SQLite. This is amplified because the SP has no rate limiting. |
| **Recommendation** | Use `io.LimitReader(resp.Body, maxResponseBytes)` with a configurable limit (e.g., 10 MB default). Return 502 if the limit is exceeded. |
| **Effort** | S (hours) |

---

### Finding 3: Prometheus metrics defined but never instrumented

| Field | Value |
|-------|-------|
| **Severity** | Medium |
| **Dimension** | Auditability |
| **Location** | `internal/metrics/metrics.go` (entire file) |
| **Description** | Five metric variables are defined (`RequestsTotal`, `RequestDuration`, `InstancesManaged`, `KokuRequestsTotal`, `NATSEventsPublished`) and the `/metrics` endpoint is mounted, but none of these counters/histograms/gauges are incremented anywhere in the codebase. The `/metrics` endpoint returns only Go runtime and process metrics. |
| **Risk** | False sense of observability. Operators who scrape `/metrics` will see an empty dashboard and may miss real issues. |
| **Recommendation** | Wire the metrics: (1) Add a chi middleware that increments `RequestsTotal` and observes `RequestDuration`. (2) Update `InstancesManaged` in Create/Delete handlers. (3) Increment `KokuRequestsTotal` in `koku.Client.do()`. (4) Increment `NATSEventsPublished` in `NATSPublisher.Publish()`. |
| **Effort** | M (days) |

---

### Finding 4: NATS connection failure is fatal at startup

| Field | Value |
|-------|-------|
| **Severity** | Medium |
| **Dimension** | Operational robustness |
| **Location** | `cmd/koku-cost-provider/main.go:80-83` |
| **Description** | If NATS is unreachable at startup, `monitoring.NewNATSPublisher()` returns an error and `main.run()` returns a fatal error, killing the process. Unlike the kcli SP which falls back to `NoopPublisher`, the cost SP has no fallback. |
| **Risk** | A transient NATS outage during deployment restarts (which is common in container orchestrators) prevents the SP from starting. The SP would be fully functional without NATS — it just can't publish status events. |
| **Recommendation** | Follow the kcli SP pattern: log a warning and fall back to `NoopPublisher` when NATS is unreachable. Alternatively, use `nats.RetryOnFailedConnect(true)` (which the publisher already configures) but don't treat initial connection failure as fatal. |
| **Effort** | S (hours) |

---

### Finding 5: Failed Koku source creation leaves orphaned store row in ERROR

| Field | Value |
|-------|-------|
| **Severity** | Medium |
| **Dimension** | Correctness |
| **Location** | `internal/handler/handler.go:131-137` |
| **Description** | When `CreateSource` fails, the handler marks the store row as ERROR and returns 500. The client cannot retry with the same `target.resource_id` because the unique constraint prevents it. The only recovery is manual — finding and deleting the ERROR row. |
| **Risk** | Transient Koku failures (network blip, restart) permanently block the target from being provisioned without operator intervention. |
| **Recommendation** | On Koku source creation failure, delete the store row before returning 500, so the client can retry cleanly. Alternatively, implement the ERROR → retry path in the reconciler. |
| **Effort** | S (hours) |

---

### Finding 6: No retry/backoff on Koku API calls

| Field | Value |
|-------|-------|
| **Severity** | Medium |
| **Dimension** | Operational robustness |
| **Location** | `internal/koku/client.go` (all methods) |
| **Description** | All Koku API calls (`CreateSource`, `CreateCostModel`, `PauseSource`, `DeleteCostModel`, `GetSourceStats`, `GetReports`, `GetForecasts`) are single-shot. A transient 5xx from Koku fails the operation immediately. |
| **Risk** | Koku restarts, which are common in the Celery/Django architecture, cause cascading failures in the cost SP. The reconciler mitigates this for `GetSourceStats` (it retries on the next poll cycle), but create/delete operations have no retry. |
| **Recommendation** | Add a retry wrapper with exponential backoff for 5xx and connection errors (3 retries, 1s/2s/4s). Do not retry 4xx (client errors). |
| **Effort** | M (days) |

---

### Finding 7: Health probe creates new http.Client on every call

| Field | Value |
|-------|-------|
| **Severity** | Low |
| **Dimension** | Performance |
| **Location** | `internal/health/health.go:79` |
| **Description** | `probeKoku()` creates a new `http.Client` on every invocation (at most once per 10s due to caching). This means a new connection pool per probe rather than reusing connections. |
| **Risk** | Minor: the 10s cache mitigates the frequency. But under thundering-herd health checks (multiple orchestrator probes), it could create unnecessary TCP connections. |
| **Recommendation** | Move the `http.Client` to a field on the `Checker` struct, created once in `NewChecker()`. |
| **Effort** | S (hours) |

---

### Finding 8: Koku client has no request context propagation

| Field | Value |
|-------|-------|
| **Severity** | Medium |
| **Dimension** | Correctness |
| **Location** | `internal/koku/client.go:42` |
| **Description** | `http.NewRequest()` is used instead of `http.NewRequestWithContext()`. The caller's context (which may carry a deadline from the request timeout middleware) is not propagated to Koku API calls. |
| **Risk** | If the client request times out (30s default), the HTTP handler returns 504 to the client, but the Koku API call continues running in the background. For `CreateSource`, this can create orphaned Koku resources that the SP doesn't track. |
| **Recommendation** | Change `client.do()` to accept a `context.Context` parameter and use `http.NewRequestWithContext(ctx, ...)`. Thread the context through all callers. |
| **Effort** | M (days) |

---

### Finding 9: Pagination token is a plain integer offset

| Field | Value |
|-------|-------|
| **Severity** | Low |
| **Dimension** | Security |
| **Location** | `internal/handler/handler.go:209-213` |
| **Description** | The `page_token` is a plain integer offset (e.g., "2", "4"). Clients can enumerate the dataset size by sending arbitrary offsets and observing `next_page_token` presence. |
| **Risk** | Information disclosure (dataset cardinality) in multi-tenant scenarios. Low severity because the SP currently has no authN/authZ and the dataset is small. |
| **Recommendation** | Encode the offset as a base64 or opaque string so it's not trivially inspectable. This is a best-practice improvement, not an urgent fix. |
| **Effort** | S (hours) |

---

### Finding 10: Cost model creation failure silently continues

| Field | Value |
|-------|-------|
| **Severity** | Low |
| **Dimension** | Correctness |
| **Location** | `internal/handler/handler.go:156-161` |
| **Description** | When `CreateCostModel` fails, the error is logged but the instance is still created with status PROVISIONING. The `koku_cost_model_uuid` is empty, so the reconciler will eventually transition it to READY — but without a cost model. |
| **Risk** | The tenant gets a cost instance with metering but no pricing/chargeback, which may be confusing. The status message doesn't indicate the partial failure. |
| **Recommendation** | Add a `status_message` note like "cost model creation failed; metering will be available but cost rates will not be applied" so the tenant knows. |
| **Effort** | S (hours) |

---

### Finding 11: Duplicate code across proxy handlers

| Field | Value |
|-------|-------|
| **Severity** | Low |
| **Dimension** | Maintainability |
| **Location** | `internal/handler/handler.go:292-388` |
| **Description** | `GetUsage`, `GetCostReport`, and `GetCostForecast` follow the exact same pattern: get instance from store, build params, call Koku, unmarshal, return. The only differences are the Koku endpoint path and the response type. |
| **Risk** | Bug fixes or improvements (e.g., adding the response size limit from Finding 2) must be applied in three places. |
| **Recommendation** | Extract a common `proxyKokuReport(instanceId, endpoint, params)` helper that the three handlers call. |
| **Effort** | S (hours) |

---

### Finding 12: No CHANGELOG, SECURITY.md, CONTRIBUTING.md

| Field | Value |
|-------|-------|
| **Severity** | Informational |
| **Dimension** | Governance |
| **Location** | Repository root |
| **Description** | The repository has no CHANGELOG (release notes are only in git commits), no SECURITY.md (no vulnerability reporting instructions), and no CONTRIBUTING.md (no contributor guidelines). |
| **Risk** | External contributors don't know how to report security issues or contribute. Release tracking requires reading git history. |
| **Recommendation** | Add SECURITY.md with a responsible disclosure email. Add CONTRIBUTING.md with build/test/lint instructions. Add a CHANGELOG.md updated on each release. |
| **Effort** | S (hours) |

---

### Finding 13: No ADRs (Architecture Decision Records)

| Field | Value |
|-------|-------|
| **Severity** | Informational |
| **Dimension** | Governance |
| **Location** | Repository-wide |
| **Description** | Key design decisions (SQLite vs PostgreSQL, soft-delete vs hard-delete, reserve-then-create pattern, reconciler vs webhook) are documented in design docs and the cursor transcript, but not in a structured ADR format. |
| **Risk** | Future maintainers may not understand why certain tradeoffs were made (e.g., "why not use PostgreSQL?"). |
| **Recommendation** | Create a `docs/adr/` directory. Retroactively document 3-4 key decisions: (1) SQLite for single-binary deployment, (2) reserve-before-create for atomicity, (3) soft-delete with status tracking, (4) reconciler polling vs NATS-driven state machine. |
| **Effort** | S (hours) |

---

### Finding 14: `Store.Delete` hard-deletes but handler uses soft-delete pattern

| Field | Value |
|-------|-------|
| **Severity** | Low |
| **Dimension** | Correctness |
| **Location** | `internal/store/store.go:86-92`, `internal/handler/handler.go:272` |
| **Description** | The `Store.Delete()` method performs a hard `DELETE FROM` SQL operation, but the handler never calls it — it uses `UpdateStatus(id, "DELETED", ...)` for soft-delete semantics. The `Delete()` method is unused in production code and only exists on the `InstanceStore` interface. |
| **Risk** | If a future developer calls `Store.Delete()` thinking it's the correct way to remove an instance, the audit trail (status transition to DELETED) is lost. Also, GORM's `CreatedAt`/`UpdatedAt` auto-management means the hard-deleted row's timestamps are gone. |
| **Recommendation** | Either: (a) remove `Store.Delete()` and the interface method if soft-delete is the intended pattern, or (b) add a `DeletedAt` field for GORM soft-delete support and use it consistently. Document the chosen pattern. |
| **Effort** | S (hours) |

---

### Finding 15: Containerfile references VERSION file that doesn't exist

| Field | Value |
|-------|-------|
| **Severity** | Low |
| **Dimension** | Governance |
| **Location** | `Containerfile:11` |
| **Description** | The Containerfile runs `cat VERSION 2>/dev/null \|\| echo 0.0.1` to determine the build version. There is no `VERSION` file in the repository, so every container build silently uses `0.0.1`. |
| **Risk** | Container images are always tagged with version `0.0.1` regardless of the actual release version. This makes it impossible to identify which code is running from the binary's version string. |
| **Recommendation** | Either create a `VERSION` file maintained alongside releases, or pass the version as a build arg: `ARG VERSION=0.0.1-dev` and use `--build-arg VERSION=$(git describe --tags)` in CI. |
| **Effort** | S (hours) |

---

### Finding 16: Currency hardcoded to USD in rate conversion

| Field | Value |
|-------|-------|
| **Severity** | Low |
| **Dimension** | Correctness |
| **Location** | `internal/handler/handler.go:429` |
| **Description** | `convertRates()` hardcodes `Unit: "USD"` for all tiered rates, ignoring the `currency` field in `CostSpec` (which defaults to "USD" but can be set to any ISO 4217 code). |
| **Risk** | If a tenant specifies `currency: EUR`, the rates are still sent to Koku as USD. Koku may misinterpret the currency, leading to incorrect cost calculations. |
| **Recommendation** | Thread the `spec.Currency` value through `convertRates()` and use it as the `Unit`. |
| **Effort** | S (hours) |

## Priority Remediation Order

| Priority | Finding | Effort | Rationale |
|----------|---------|--------|-----------|
| 1 | #5 (orphaned store row on Koku failure) | S | Blocks retry without operator intervention |
| 2 | #8 (no context propagation to Koku) | M | Can create orphaned Koku resources on timeout |
| 3 | #2 (unbounded response proxy) | S | OOM risk on legitimate large queries |
| 4 | #4 (fatal NATS startup) | S | Fragile deployment |
| 5 | #1 (plaintext identity) | S | Credential exposure on non-localhost |
| 6 | #3 (metrics not wired) | M | False sense of observability |
| 7 | #6 (no Koku retry) | M | Fragile to transient Koku restarts |
| 8 | #10 (silent cost model failure) | S | Confusing for tenants |
| 9 | #16 (hardcoded USD) | S | Incorrect cost calculation |
| 10 | #11 (proxy duplication) | S | Maintainability |
| 11 | #14 (hard vs soft delete) | S | Confusing API surface |
| 12 | #7 (health http.Client) | S | Minor performance |
| 13 | #15 (VERSION file) | S | Governance |
| 14 | #9 (pagination token) | S | Best practice |
| 15 | #12 (missing governance docs) | S | Governance |
| 16 | #13 (missing ADRs) | S | Governance |

## Accepted Risks

None explicitly accepted at this time.

## Current State

| Metric | Value |
|--------|-------|
| Total findings | 16 |
| Critical | 0 |
| High | 2 |
| Medium | 4 |
| Low | 6 |
| Informational | 2 |
| Resolved | 0 |
| Accepted | 0 |
| Open | 16 |

**Prior review findings (commit a0cf883):** 15+ findings were identified and
resolved in the initial due diligence pass. This review confirms those fixes
remain intact and focuses on new issues and gaps not covered by the initial
review.
