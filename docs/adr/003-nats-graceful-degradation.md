# ADR-003: NATS Graceful Degradation

**Status:** Accepted  
**Date:** 2026-06-16 (retrospective)

## Context

The provider publishes CloudEvents to NATS JetStream for status updates. In development or edge deployments, NATS may not be available. Previously, a NATS connection failure at startup was fatal.

## Decision

Fall back to a `NoopPublisher` when NATS is unavailable or unconfigured (`SP_NATS_URL` empty or connection fails).

## Rationale

- The core functionality (Koku source creation, cost model management, report proxying) does not depend on NATS.
- NATS events are informational status updates consumed by the DCM control plane; the control plane also polls health endpoints as a fallback.
- Making NATS optional simplifies local development and testing.
- A warning log is emitted so operators know events are not being published.

## Consequences

- The DCM control plane will not receive real-time status events; it must rely on health-check polling.
- Operators should monitor the startup logs for NATS fallback warnings in production.
