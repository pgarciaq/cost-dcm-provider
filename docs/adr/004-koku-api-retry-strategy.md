# ADR-004: Koku API Retry Strategy

**Status:** Accepted  
**Date:** 2026-06-16 (retrospective)

## Context

The Koku API is an external dependency that may experience transient failures (network blips, 502/503 from load balancers, brief maintenance windows).

## Decision

Implement retry with exponential backoff in the Koku client:
- Up to 3 attempts.
- Initial backoff of 1 second, doubling each attempt.
- Retry on network errors and 5xx responses.
- Do not retry on 4xx (client errors are not transient).
- Respect `context.Context` cancellation during backoff.

## Rationale

- Simple exponential backoff handles the most common transient failure modes.
- 3 attempts with 1s/2s/4s backoff keeps total latency under 10 seconds.
- Context-aware backoff ensures graceful shutdown is not blocked.
- No external dependency (no retry library needed).

## Alternatives Considered

- **No retry:** Too fragile for a production integration.
- **Circuit breaker:** Adds complexity; the health checker already provides a form of circuit detection.
- **Retry library (e.g., `cenkalti/backoff`):** Adds a dependency for a simple pattern.

## Consequences

- Transient Koku outages of a few seconds are handled transparently.
- Sustained outages will still fail after ~7 seconds total, which is acceptable.
