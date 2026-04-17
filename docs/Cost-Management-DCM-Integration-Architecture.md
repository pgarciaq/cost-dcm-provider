# Cost Management + DCM Integration Architecture

## Bridging Koku/Cost Management with DCM Service Providers

**Version:** 1.0
**Date:** 2026-04-17
**Prerequisites:** [DCM Architecture and Integration Guide](./DCM-Architecture-and-Integration-Guide.md)

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Problem Statement](#2-problem-statement)
3. [System Landscape](#3-system-landscape)
4. [How Koku Works Today](#4-how-koku-works-today)
5. [How DCM Works Today](#5-how-dcm-works-today)
6. [The Integration Gap](#6-the-integration-gap)
7. [Integration Architecture Options](#7-integration-architecture-options)
8. [Recommended Architecture: Option C — Hybrid Bridge](#8-recommended-architecture-option-c--hybrid-bridge)
9. [Data Mapping: DCM to Koku](#9-data-mapping-dcm-to-koku)
10. [Communication Bridge: NATS to Koku Pipeline](#10-communication-bridge-nats-to-koku-pipeline)
11. [Cost Service Provider for DCM](#11-cost-service-provider-for-dcm)
12. [Policy Integration](#12-policy-integration)
13. [Deployment Topology](#13-deployment-topology)
14. [Open Questions and Trade-offs](#14-open-questions-and-trade-offs)
15. [Phased Delivery Plan](#15-phased-delivery-plan)

---

## 1. Executive Summary

This document designs the integration between **DCM** (Data Center Management)
and **Koku/Cost Management** (the open-source FinOps engine upstream of Red Hat
Lightspeed Cost Management).

The goal: when DCM provisions infrastructure (clusters, VMs, containers), Cost
Management should automatically track, rate, and report the costs — without
requiring the end user to manually configure sources, install operators, or
manage cost models separately.

The integration must work in **on-premises** deployments where both DCM and Koku
run in the same data center, using PostgreSQL-only paths (no Trino).

---

## 2. Problem Statement

Today, Koku and DCM operate in completely separate worlds:

| Aspect | Koku | DCM |
|--------|------|-----|
| **Knows about infra** | Only after operator uploads metrics | At provisioning time |
| **Lifecycle events** | Discovers clusters reactively | Orchestrates them proactively |
| **Cost models** | Manual admin setup per source | No concept of cost |
| **Status** | No provisioning awareness | Tracks PENDING→READY→DELETED |
| **Messaging** | Kafka (upload pipeline) + Celery | NATS JetStream (CloudEvents) |
| **Cross-service** | Standalone | Hub-and-spoke, no SP↔SP |

**The core tension:** DCM knows *what* was provisioned and *when*, but not *what
it costs*. Koku knows *how to calculate costs* but not *what DCM provisioned*.
Neither system talks to the other.

---

## 3. System Landscape

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Data Center                                     │
│                                                                         │
│  ┌──────────────────────────┐    ┌──────────────────────────────────┐   │
│  │       DCM Control Plane  │    │    Koku / Cost Management        │   │
│  │                          │    │                                  │   │
│  │  Catalog ─► Placement    │    │  Koku API ◄── koku-ui            │   │
│  │               │          │    │    │                              │   │
│  │          Policy (OPA)    │    │  Masu ─► Celery Workers           │   │
│  │               │          │    │    │         │                    │   │
│  │           SPM/SPRM       │    │  PostgreSQL  Redis                │   │
│  │            │      │      │    │    │                              │   │
│  │         NATS    HTTP     │    │  [Kafka] (upload pipeline)        │   │
│  │            │      │      │    │                                  │   │
│  └────────────┼──────┼──────┘    └──────────────┬───────────────────┘   │
│               │      │                           │                      │
│  ┌────────────┼──────┼───────────────────────────┼──────────────────┐   │
│  │            ▼      ▼                           ▼                  │   │
│  │  ┌──────────────┐ ┌──────────────┐ ┌──────────────────────┐      │   │
│  │  │ ACM Cluster  │ │ KubeVirt VM  │ │  koku-metrics-       │      │   │
│  │  │ SP           │ │ SP           │ │  operator             │      │   │
│  │  └──────┬───────┘ └──────┬───────┘ └──────────┬───────────┘      │   │
│  │         │                │                     │                  │   │
│  │         ▼                ▼                     ▼                  │   │
│  │    OpenShift Clusters    KubeVirt VMs     Prometheus Metrics      │   │
│  │                                                                  │   │
│  │                     Infrastructure                               │   │
│  └──────────────────────────────────────────────────────────────────┘   │
│                                                                         │
│  ════════════════════════════════════════════════════════════════════    │
│  THE GAP: No connection between DCM provisioning events and              │
│           Koku's cost pipeline. No shared identity for clusters.         │
│  ════════════════════════════════════════════════════════════════════    │
│                                                                         │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 4. How Koku Works Today

### Data Ingestion (OCP Path)

1. **koku-metrics-operator** runs on each OpenShift cluster
2. Queries **Prometheus/Thanos** every hour for node, pod, storage, namespace,
   VM, and GPU metrics
3. Generates **CSV files** (pod usage, storage usage, node labels, namespace
   labels, VM usage, GPU usage)
4. Packages CSVs + `manifest.json` into a **tar.gz**
5. Uploads to the **ingress** endpoint (content-type:
   `application/vnd.redhat.hccm.tar+tgz`)
6. Koku's **Kafka listener** (or dev HTTP endpoint) downloads and extracts
7. CSVs are processed into **line item tables** (PostgreSQL on-prem, or Parquet
   on S3 + Trino for cloud)
8. **Summary tables** aggregate usage data
9. **Cost model** rates are applied (usage costs, tag costs, monthly costs,
   markup, distribution)
10. **UI summary tables** are populated for the API

### Key CSV Columns (Pod Usage)

```
report_period_start, report_period_end, interval_start, interval_end,
node, namespace, pod,
pod_usage_cpu_core_seconds, pod_request_cpu_core_seconds, pod_limit_cpu_core_seconds,
pod_usage_memory_byte_seconds, pod_request_memory_byte_seconds, pod_limit_memory_byte_seconds,
node_capacity_cpu_cores, node_capacity_cpu_core_seconds,
node_capacity_memory_bytes, node_capacity_memory_byte_seconds,
node_role, resource_id, pod_labels
```

### Cost Model Application Order

1. **Usage costs** — tiered rates × usage quantities (CPU, memory, volume,
   node/cluster hourly)
2. **VM usage costs** — VM-specific hourly rates
3. **Markup** — `infrastructure_raw_cost × markup_percentage`
4. **Monthly costs** — node/cluster/PVC/VM monthly lump sums, distributed by
   usage share
5. **Tag-based costs** — rates keyed by label key:value pairs
6. **Distribution** — platform, worker, storage, network, GPU overhead
   redistributed to user projects
7. **UI summary population** — aggregated into partitioned summary tables

### What Koku Needs Per Cluster

To calculate costs for an OpenShift cluster, Koku requires:

- A **Provider/Source** record with a stable `cluster_id`
- A **CostModel** linked via **CostModelMap** (rates, markup, distribution config)
- **Hourly usage data**: pod CPU/memory, node capacity, PVC storage, namespace
  labels, optionally VM and GPU data
- For **OCP-on-cloud**: infrastructure raw costs from the cloud provider
  (AWS CUR, Azure exports, GCP BigQuery)

---

## 5. How DCM Works Today

### Cluster Provisioning Flow

1. User orders from **Catalog** (e.g., "Production Cluster" catalog item)
2. **Placement Manager** stores intent, evaluates **policies** (OPA/Rego)
3. **SPRM** forwards to **ACM Cluster SP**
4. SP creates `HostedCluster` + `NodePool` on ACM hub
5. SP monitors conditions, publishes **CloudEvents** to NATS
6. SPM stores status: `PENDING → PROVISIONING → READY → DELETED`

### What DCM Knows at Provisioning Time

| Data Point | Where in DCM | Relevance to Cost |
|------------|-------------|-------------------|
| Cluster name | `spec.metadata.name` | Maps to Koku `cluster_alias` |
| Instance ID | `dcm-instance-id` label | Stable cross-system correlator |
| K8s version | `spec.version` | Version-specific pricing |
| Worker count | `spec.nodes.workers.count` | Capacity baseline |
| Worker CPU/memory/storage | `spec.nodes.workers.*` | Resource sizing |
| Platform (kubevirt/baremetal) | `provider_hints.acm.platform` | Affects cost model choice |
| Provider name | Registration `name` | Multi-SP cost attribution |
| Lifecycle status | CloudEvents on NATS | Cost start/stop dates |
| Labels | `spec.metadata.labels` | Tag-based cost allocation |

### What DCM Does NOT Know

- Actual runtime usage (CPU utilization, memory pressure, pod counts)
- Infrastructure costs from underlying cloud providers
- Cost rates or financial models
- Historical usage trends

---

## 6. The Integration Gap

```
  DCM knows:                    Koku knows:
  ┌────────────────┐            ┌────────────────┐
  │ What was        │            │ How to calculate │
  │ provisioned     │            │ costs            │
  │                 │            │                  │
  │ When it became  │  ══GAP══  │ What metrics     │
  │ ready/deleted   │            │ to collect       │
  │                 │            │                  │
  │ Who requested   │            │ How to apply     │
  │ it and why      │            │ cost models      │
  └────────────────┘            └────────────────┘
```

### Specific gaps:

1. **No shared cluster identity** — DCM's `dcm-instance-id` ≠ Koku's
   `cluster_id` (which is the OpenShift cluster's actual cluster ID)
2. **No automatic source creation** — Koku requires a Provider/Source record;
   nobody creates it when DCM provisions a cluster
3. **No operator deployment** — The metrics operator must be installed on each
   cluster to feed Koku; DCM doesn't do this
4. **No cost model assignment** — Even if data flows, Koku needs a CostModel
   linked to the source
5. **No lifecycle synchronization** — When DCM deletes a cluster, Koku doesn't
   know to stop expecting data or to mark the source inactive
6. **Different messaging systems** — DCM uses NATS, Koku uses Kafka + Celery;
   no shared bus

---

## 7. Integration Architecture Options

### Option A: DCM-Unaware (Operator-Only)

```
DCM provisions cluster → Admin manually installs koku-metrics-operator
                        → Admin manually creates Koku source + cost model
                        → Operator uploads data → Koku processes
```

**Pros:** No code changes. Works today.
**Cons:** Fully manual. No lifecycle sync. DCM and Koku are islands.
No value-add from integration.

### Option B: DCM as Koku Data Source (Push Model)

```
DCM provisions cluster → DCM calls Koku API to create Source
                        → DCM deploys operator on new cluster
                        → Operator uploads to Koku normally
                        → DCM lifecycle events update Koku source status
```

**Pros:** Leverages existing Koku pipeline. Minimal Koku changes.
**Cons:** DCM must understand Koku's API. Tight coupling. DCM becomes
responsible for operator deployment (out of scope for current SP model).

### Option C: Hybrid Bridge (Recommended)

```
DCM provisions cluster → Bridge service watches NATS for DCM events
                        → Bridge creates Koku Source + CostModel
                        → Bridge triggers operator deployment (via ACM policy)
                        → Operator uploads to Koku normally
                        → Bridge syncs lifecycle (READY/DELETED → Source active/inactive)

For cost queries:
User asks DCM "how much does my cluster cost?"
  → Cost SP queries Koku API
  → Returns cost data through DCM's standard SP response
```

**Pros:** Loose coupling. Each system does what it's best at. Koku's proven
pipeline handles actual cost calculation. Bridge is a focused translation layer.
**Cons:** New component to build and maintain.

### Option D: Cost as a DCM Service Provider

```
DCM provisions cluster → Normal DCM flow
                        → Cost SP subscribes to NATS dcm.cluster subject
                        → On READY: creates Koku source, deploys operator
                        → On cost query: wraps Koku API
                        → On DELETED: deactivates source
```

**Pros:** Fits DCM's SP model natively. Discoverable through DCM registry.
**Cons:** Stretches the SP concept (cost SP doesn't "provision" anything in the
traditional sense). Status events are one-directional in current DCM.

### Option E: Full Cost SP (Koku Embedded)

```
Cost SP receives cluster spec from DCM
  → Runs its own metrics collection (embeds Prometheus queries)
  → Applies cost models internally
  → Returns cost data as SP response
  → No dependency on Koku at all
```

**Pros:** Self-contained. No Koku dependency.
**Cons:** Reimplements most of Koku. Massive scope. Loses Koku's mature
cost model, distribution, and reporting features.

---

## 8. Recommended Architecture: Option C — Hybrid Bridge

The recommended approach combines a **bridge service** for lifecycle
synchronization with a **cost service provider** for DCM-native cost queries.

```
┌─────────────────────────────────────────────────────────────────┐
│                       DCM Control Plane                         │
│                                                                 │
│  Catalog → Placement → Policy → SPRM → ACM Cluster SP          │
│                                          │                      │
│                                     NATS JetStream              │
│                                     (dcm.cluster events)        │
│                                          │                      │
│  ┌───────────────────────────────────────┼──────────────────┐   │
│  │            DCM-Koku Bridge            │                  │   │
│  │                                       ▼                  │   │
│  │  ┌─────────────┐  ┌──────────────────────────────────┐   │   │
│  │  │ NATS        │  │ Lifecycle Synchronizer            │   │   │
│  │  │ Consumer    │──│                                   │   │   │
│  │  │             │  │ On READY:                         │   │   │
│  │  │ Subscribes  │  │   1. Create Koku Provider/Source  │   │   │
│  │  │ dcm.cluster │  │   2. Assign default CostModel     │   │   │
│  │  │ dcm.vm      │  │   3. Trigger operator deployment  │   │   │
│  │  │ dcm.*       │  │                                   │   │   │
│  │  └─────────────┘  │ On DELETED:                       │   │   │
│  │                    │   1. Deactivate Koku Source       │   │   │
│  │                    │   2. Mark cost data as final      │   │   │
│  │                    └──────────────────────────────────┘   │   │
│  │                                                           │   │
│  │  ┌──────────────────────────────────┐                     │   │
│  │  │ Cost Query Proxy                 │                     │   │
│  │  │                                  │                     │   │
│  │  │ GET /cost-summary?cluster=X      │                     │   │
│  │  │   → maps DCM instance ID         │                     │   │
│  │  │   → queries Koku report API      │                     │   │
│  │  │   → returns structured cost data │                     │   │
│  │  └──────────────────────────────────┘                     │   │
│  │                                                           │   │
│  │  ┌──────────────────────────────────┐                     │   │
│  │  │ ID Mapping Store                 │                     │   │
│  │  │                                  │                     │   │
│  │  │ dcm_instance_id ↔ cluster_id     │                     │   │
│  │  │ dcm_instance_id ↔ koku_source_uuid│                    │   │
│  │  │ dcm_provider    ↔ koku_cost_model │                    │   │
│  │  └──────────────────────────────────┘                     │   │
│  └───────────────────────────────────────────────────────────┘   │
│                            │                                     │
│                            │ Koku REST API                       │
│                            ▼                                     │
│  ┌───────────────────────────────────────────────────────────┐   │
│  │                  Koku / Cost Management                   │   │
│  │                                                           │   │
│  │  Provider/Source ← created by bridge                      │   │
│  │  CostModel ← assigned by bridge (or admin override)      │   │
│  │  Masu pipeline ← fed by koku-metrics-operator             │   │
│  │  Report API ← queried by bridge for cost responses        │   │
│  └───────────────────────────────────────────────────────────┘   │
│                                                                   │
│  ┌───────────────────────────────────────────────────────────┐   │
│  │              Provisioned Clusters                         │   │
│  │                                                           │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │   │
│  │  │ Cluster A   │  │ Cluster B   │  │ Cluster C   │       │   │
│  │  │ metrics-op  │  │ metrics-op  │  │ metrics-op  │       │   │
│  │  │ → uploads   │  │ → uploads   │  │ → uploads   │       │   │
│  │  │   to Koku   │  │   to Koku   │  │   to Koku   │       │   │
│  │  └─────────────┘  └─────────────┘  └─────────────┘       │   │
│  └───────────────────────────────────────────────────────────┘   │
└───────────────────────────────────────────────────────────────────┘
```

---

## 9. Data Mapping: DCM to Koku

### Identity Mapping

| DCM Concept | Koku Concept | Mapping Strategy |
|-------------|-------------|------------------|
| `dcm-instance-id` (UUID on HostedCluster label) | `cluster_id` (OpenShift cluster ID from ClusterVersion CR) | Bridge maintains a mapping table. The actual `cluster_id` is only known after the cluster is READY (read from the provisioned cluster). |
| `spec.metadata.name` | `cluster_alias` (Provider name in Koku) | Direct mapping. |
| DCM provider name (e.g., `acm-cluster-sp-prod`) | Koku `Provider.type = OCP` | All DCM-provisioned OCP clusters become `PROVIDER_OCP` sources in Koku. |
| `spec.metadata.labels` | Koku tag-based rates / cost categories | Labels flow through the metrics operator as `namespace_labels` and `pod_labels`. DCM labels on the cluster spec could be added as default namespace labels by the bridge. |
| DCM catalog item ID | (no equivalent) | Could be stored in Koku Provider `additional_context` or as a label for grouping. |

### Lifecycle Mapping

| DCM Status (CloudEvent) | Koku Action |
|--------------------------|-------------|
| `PENDING` | No action (cluster not yet usable) |
| `PROVISIONING` | No action (no metrics yet) |
| `READY` | Create Source, assign CostModel, trigger operator install |
| `UNAVAILABLE` | Mark source as paused (stop expecting data) |
| `DELETED` | Deactivate source, finalize cost data |

### Resource Spec Mapping (for cost model selection)

| DCM Cluster Spec | Relevance to Cost Model |
|-------------------|------------------------|
| `nodes.workers.count` | Determines expected node count for node monthly rates |
| `nodes.workers.cpu` | Drives CPU tiered rate selection |
| `nodes.workers.memory` | Drives memory tiered rate selection |
| `nodes.workers.storage` | Drives volume rate selection |
| `provider_hints.acm.platform` (kubevirt/baremetal) | Different cost profiles: KubeVirt has VM overhead; baremetal has hardware costs |
| `version` (K8s) | May affect rate tiers or cost model version |

---

## 10. Communication Bridge: NATS to Koku Pipeline

### NATS Consumer (DCM Side)

The bridge subscribes to NATS JetStream subjects that DCM SPs publish to:

| Subject | Event Type | Bridge Action |
|---------|-----------|---------------|
| `dcm.cluster` | `dcm.status.cluster` | Cluster lifecycle events → trigger Koku source management |
| `dcm.container` | `dcm.status.container` | Container lifecycle → future container cost tracking |
| `dcm.vm` | `dcm.status.vm` | VM lifecycle → future VM cost tracking |

### CloudEvent to Koku Action Translation

The bridge processes CloudEvents with this data structure:

```json
{
  "data": {
    "id": "dcm-instance-uuid",
    "status": "READY",
    "message": "Cluster is ready and all nodes are available"
  }
}
```

**On `READY`:**

1. Query DCM's SPM for the full instance details:
   `GET /api/v1alpha1/service-type-instances/{id}`
2. Query the provisioned cluster (via kubeconfig from ACM) for the real
   `cluster_id` (from `ClusterVersion` CR)
3. Store the mapping: `dcm_instance_id ↔ cluster_id`
4. Call Koku API: `POST /api/cost-management/v1/sources/` to create a Provider
5. Assign a CostModel (based on DCM catalog item, platform, or default)
6. Trigger operator deployment on the new cluster (via ACM policy or
   ManagedClusterAction)

**On `DELETED`:**

1. Look up `cluster_id` from mapping table
2. Call Koku API to deactivate the source (set `paused=True` or delete)
3. Optionally: finalize cost data, generate final cost report

### Koku API Calls (Bridge → Koku)

The bridge uses Koku's existing REST API:

```
POST /api/cost-management/v1/sources/
  → Create OCP source with cluster_id and authentication

POST /api/cost-management/v1/cost-models/
  → Create or assign cost model to the new source

PATCH /api/cost-management/v1/sources/{uuid}/
  → Update source status (paused, active)

GET /api/cost-management/v1/reports/openshift/costs/
  → Query cost data for the cost service provider responses
```

### Operator Deployment Strategy

The metrics operator must run on each provisioned cluster. Options:

| Strategy | How | Complexity |
|----------|-----|------------|
| **ACM Policy** (recommended) | ACM PolicySet that auto-deploys the operator to clusters labeled `dcm.project/managed-by=dcm` | Low — leverages ACM's built-in governance. Bridge just ensures the label exists (already set by ACM Cluster SP). |
| **ManagedClusterAction** | Bridge directly creates a ManagedClusterAction CR on the hub to install the operator | Medium — more control, but more code. |
| **ArgoCD ApplicationSet** | GitOps-driven: ApplicationSet generator matches DCM-labeled clusters | Medium — requires ArgoCD infrastructure. |
| **Manual** | Admin installs operator post-provisioning | None — but defeats automation goal. |

**Recommended:** ACM Policy. The ACM Cluster SP already labels `HostedCluster`
with `dcm.project/managed-by=dcm`. An ACM `ConfigurationPolicy` can match that
label and ensure the koku-metrics-operator subscription exists on the managed
cluster.

---

## 11. Cost Service Provider for DCM

Optionally, the bridge can also register as a **DCM service provider** to
expose cost data back through DCM's native APIs.

### Registration

```json
{
  "name": "cost-management-sp",
  "service_type": "cost_report",
  "endpoint": "http://cost-bridge:8080/api/v1alpha1/cost-reports",
  "schema_version": "v1alpha1",
  "operations": ["read"],
  "display_name": "Cost Management Service Provider"
}
```

This is a **read-only SP** — it doesn't provision anything, it provides cost
data for resources managed by other SPs.

### API Surface

```
GET /api/v1alpha1/cost-reports/{dcm-instance-id}
  → Returns cost summary for a DCM-provisioned resource

GET /api/v1alpha1/cost-reports/{dcm-instance-id}/breakdown
  → Returns detailed cost breakdown (CPU, memory, storage, distributed)

GET /api/v1alpha1/cost-reports?provider_name=acm-cluster-sp-prod
  → Returns cost summaries for all resources from a specific SP
```

### Response Format (wrapping Koku data)

```json
{
  "dcm_instance_id": "abc-123",
  "cluster_id": "ocp-cluster-xyz",
  "period": {
    "start": "2026-04-01",
    "end": "2026-04-17"
  },
  "cost": {
    "total": "4523.67",
    "infrastructure": "2100.00",
    "supplementary": "1200.50",
    "distributed": "1223.17",
    "currency": "USD"
  },
  "breakdown": {
    "cpu": "1800.00",
    "memory": "950.50",
    "storage": "450.00",
    "platform_distributed": "623.17",
    "worker_distributed": "600.00",
    "markup": "100.00"
  },
  "status": "current",
  "last_data_received": "2026-04-17T09:00:00Z"
}
```

### Where This Shows Up

With a cost SP registered, the data could surface through:

1. **DCM CLI**: `dcm cost-report get --instance abc-123`
2. **DCM API**: clients query the gateway, which routes to the cost SP
3. **Future DCM UI**: a cost panel on the resource detail page
4. **DCM Policies**: Rego policies could query cost data for budget enforcement
   (future capability)

---

## 12. Policy Integration

### Cost-Aware Provisioning Policies

With cost data accessible via the bridge, DCM policies could enforce financial
governance:

```rego
package policies.budget_control

import future.keywords

# Reject cluster requests that would exceed the monthly budget
main := {
    "rejected": true,
    "rejection_reason": sprintf(
        "Estimated monthly cost $%v exceeds budget $%v for %s clusters",
        [estimated_cost, budget_limit, input.spec.provider_hints.acm.platform]
    )
} {
    input.spec.service_type == "cluster"
    estimated_cost := estimate_monthly_cost(input.spec)
    budget_limit := 10000
    estimated_cost > budget_limit
}

estimate_monthly_cost(spec) := cost {
    workers := object.get(spec, ["nodes", "workers", "count"], 3)
    cpu_per_worker := object.get(spec, ["nodes", "workers", "cpu"], 4)
    rate_per_core_hour := 0.05
    cost := workers * cpu_per_worker * 730 * rate_per_core_hour
}
```

This is a **future capability** — it requires either:
- Static cost estimation in Rego (based on spec, no live data)
- External data integration in OPA (loading cost rates as data bundles)

### Cost Model Assignment Policies

Policies could also automate which cost model gets assigned:

```rego
package policies.cost_model_assignment

main := {
    "rejected": false,
    "patch": {
        "metadata": {
            "labels": {
                "cost_model": "production-standard"
            }
        }
    }
} {
    input.spec.service_type == "cluster"
    input.spec.metadata.labels.environment == "production"
}
```

The bridge reads the `cost_model` label and assigns the corresponding Koku
CostModel.

---

## 13. Deployment Topology

### On-Premises Stack

```
┌─────────────────────────────────────────────────────┐
│  Management Cluster (ACM Hub)                       │
│                                                     │
│  ┌────────────┐  ┌──────────────┐  ┌────────────┐  │
│  │ DCM Stack  │  │ DCM-Koku     │  │ Koku Stack │  │
│  │            │  │ Bridge       │  │            │  │
│  │ Gateway    │  │              │  │ Koku API   │  │
│  │ Catalog    │  │ NATS consumer│  │ Masu       │  │
│  │ Placement  │  │ Koku client  │  │ Workers    │  │
│  │ Policy     │  │ ID mapper    │  │ Listener   │  │
│  │ SPM        │  │ Cost proxy   │  │            │  │
│  │ NATS       │  │              │  │ PostgreSQL │  │
│  │ PostgreSQL │  │ PostgreSQL   │  │ Redis      │  │
│  └────────────┘  └──────────────┘  └────────────┘  │
│                                                     │
│  ┌──────────────┐  ┌──────────────┐                 │
│  │ ACM Cluster  │  │ KubeVirt VM  │                 │
│  │ SP           │  │ SP           │                 │
│  └──────────────┘  └──────────────┘                 │
│                                                     │
│  ACM Policies:                                      │
│  - Deploy koku-metrics-operator on dcm clusters     │
│  - Configure operator with Koku ingress URL         │
└─────────────────────────────────────────────────────┘
         │
         │  HyperShift
         ▼
┌──────────────────┐  ┌──────────────────┐
│ Hosted Cluster A │  │ Hosted Cluster B │
│ koku-metrics-op  │  │ koku-metrics-op  │
│ → uploads to     │  │ → uploads to     │
│   Koku ingress   │  │   Koku ingress   │
└──────────────────┘  └──────────────────┘
```

### Shared Infrastructure

| Component | Shared? | Notes |
|-----------|---------|-------|
| PostgreSQL | Separate DBs recommended | DCM and Koku each need their own databases; bridge needs a small one for mappings |
| NATS | DCM-owned | Bridge is an additional consumer |
| Kafka | Koku-owned (if used) | On-prem may use direct HTTP ingress instead |
| Redis/Valkey | Koku-owned | DCM doesn't use Redis |
| S3/MinIO | Koku-owned (on-prem) | For CSV/tarball staging |

---

## 14. Open Questions and Trade-offs

### Architectural Decisions Needed

| Question | Options | Recommendation |
|----------|---------|----------------|
| **Should the bridge be a DCM SP?** | (a) Standalone service, (b) DCM SP, (c) Both | **(c) Both** — NATS consumer for lifecycle, SP registration for cost queries |
| **How to get `cluster_id`?** | (a) Read from provisioned cluster, (b) Use DCM instance ID as cluster_id, (c) Wait for first operator upload | **(a)** — most accurate, but requires kubeconfig access |
| **Cost model assignment** | (a) Default model for all DCM clusters, (b) Per-catalog-item model, (c) Per-label model | **(b)** with **(c)** override — catalog items map to cost profiles |
| **Operator deployment** | (a) ACM Policy, (b) ManagedClusterAction, (c) Manual | **(a)** — least code, leverages existing ACM |
| **On-prem Koku path** | (a) PostgreSQL-only (`ONPREM=True`), (b) Full stack with Trino | **(a)** — matches DCM's on-prem target |
| **Cost data freshness** | (a) Hourly (matches operator), (b) Daily summary, (c) On-demand | **(a)** — operator already uploads hourly |

### Risk Areas

1. **Kubeconfig access:** The bridge needs to read the provisioned cluster's
   kubeconfig to discover its `cluster_id`. This kubeconfig is available from
   the ACM hub (`HostedCluster.status.kubeConfig` secret) but requires RBAC.

2. **Operator bootstrap delay:** After a cluster is READY, the metrics operator
   must be deployed, must collect at least one hour of data, and must upload it
   before Koku has anything to price. First cost data is ~2 hours after READY.

3. **DCM has no SP-to-SP communication:** The cost SP cannot natively react to
   events from the cluster SP. The NATS subscription is a custom extension of
   the current architecture.

4. **Koku API authentication:** On-prem Koku uses identity headers. The bridge
   needs a service account or development identity to call Koku's API.

5. **Multi-tenancy:** DCM doesn't have tenancy yet (v1). Koku uses
   schema-per-tenant. The bridge must know which Koku tenant to create sources
   in.

---

## 15. Phased Delivery Plan

### Phase 1: Manual Integration (No Code)

**Goal:** Validate the cost pipeline works for DCM-provisioned clusters.

- Admin manually installs koku-metrics-operator on provisioned clusters
- Admin manually creates Koku sources and cost models
- Documents the manual steps as a "getting started" guide
- Validates that cost data flows correctly

**Deliverable:** Documentation + validation.

### Phase 2: Lifecycle Bridge (NATS Consumer)

**Goal:** Automate source creation and operator deployment.

- Build the NATS consumer that watches `dcm.cluster`
- On READY: create Koku source, assign default cost model
- On DELETED: deactivate source
- Deploy operator via ACM Policy
- ID mapping store (dcm_instance_id ↔ cluster_id ↔ koku_source_uuid)

**Deliverable:** `dcm-koku-bridge` Go service.

### Phase 3: Cost Query Proxy (Cost SP)

**Goal:** Expose cost data through DCM's API.

- Register as a DCM service provider
- Implement cost report endpoints that query Koku's report API
- Add DCM CLI commands for cost queries
- Support filtering by instance ID, provider, time range

**Deliverable:** Cost SP endpoints + CLI extension.

### Phase 4: Policy Integration

**Goal:** Financial governance in DCM policies.

- Cost estimation in Rego policies (static model from catalog item specs)
- Cost model assignment via labels/policies
- Budget enforcement rules
- Cost data as OPA external data (advanced)

**Deliverable:** Example policies + documentation.

### Phase 5: Multi-Resource Cost Tracking

**Goal:** Extend beyond clusters to VMs and containers.

- Subscribe to `dcm.vm` and `dcm.container` subjects
- Map VM lifecycle to Koku's OCP virtualization cost models
- Container cost tracking (namespace-level)
- Unified cost dashboard across all DCM-provisioned resources

**Deliverable:** Full multi-resource cost coverage.
