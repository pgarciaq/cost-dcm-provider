# ADR-001: SQLite for Instance Persistence

**Status:** Accepted  
**Date:** 2026-05-01 (retrospective)

## Context

The cost-dcm-provider needs to persist instance state (Koku source UUID, cost model UUID, status) across restarts. Options considered:

1. **PostgreSQL** — Full RDBMS, used by Koku itself.
2. **SQLite** — Embedded, zero-config, single-file database.
3. **In-memory map** — Simplest, but loses state on restart.

## Decision

Use SQLite via GORM with WAL mode.

## Rationale

- The provider manages a modest number of instances (tens to hundreds, not millions).
- SQLite requires no external infrastructure, simplifying deployment.
- WAL mode provides good concurrent read performance.
- GORM gives us migration support and a familiar ORM interface.
- If scale demands grow, migrating to PostgreSQL via GORM's driver swap is straightforward.

## Consequences

- Single-writer limitation is acceptable for our workload.
- The database file must be on a persistent volume in container deployments.
- `CGO_ENABLED=1` is required at build time for the SQLite driver.
