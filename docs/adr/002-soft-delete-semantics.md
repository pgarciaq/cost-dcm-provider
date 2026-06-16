# ADR-002: Soft-Delete Semantics for Instances

**Status:** Accepted  
**Date:** 2026-06-16 (retrospective)

## Context

When a user deletes a cost instance, we need to decide whether to physically remove the database row or mark it as deleted.

## Decision

Use soft-delete: set `status = 'DELETED'` via `UpdateStatus()` rather than removing the row. The `Store.Delete()` hard-delete method was removed.

## Rationale

- Preserves the audit trail: creation time, Koku source/cost-model UUIDs, and the full status history are retained.
- Enables future features like "show deleted instances" or undo.
- The Koku source is paused (not deleted) on the upstream side, maintaining consistency — both sides keep the record.
- Instance volume is low enough that storage from soft-deleted rows is negligible.

## Consequences

- List queries return all instances including DELETED ones. The API consumer can filter by status if needed.
- A future cleanup job could purge rows older than a retention period if needed.
