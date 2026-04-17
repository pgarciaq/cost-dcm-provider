# Cost Management Service Provider for DCM

Architecture and design documents for integrating
[Red Hat Lightspeed Cost Management](https://github.com/project-koku/koku)
(Project Koku) with [DCM](https://dcm-project.github.io/) (Data Center
Management) as a native DCM service provider.

## What This Is

DCM is a control plane for provisioning infrastructure (clusters, VMs,
containers) in on-premises and sovereign cloud environments. Koku is the
open-source FinOps engine upstream of Red Hat Lightspeed Cost Management.

Today these systems operate independently. This repository contains the
architectural design for connecting them — so that when DCM provisions
infrastructure, Cost Management automatically tracks metering and costs
without manual configuration.

## Documents

| Document | Description |
|----------|-------------|
| [DCM Architecture and Integration Guide](docs/DCM-Architecture-and-Integration-Guide.md) | Comprehensive reference on DCM's architecture: service providers, catalog model, policy engine, communication patterns, and provisioning lifecycle. |
| [Cost Management + DCM Integration Architecture](docs/Cost-Management-DCM-Integration-Architecture.md) | How Koku's data pipeline and cost models map to DCM's provisioning events. Covers the NATS-to-Koku bridge, data mapping, operator deployment strategy, and phased delivery plan. |
| [Cost Service Provider Design](docs/Cost-Service-Provider-Design.md) | The full service provider proposal: new `cost` service type, three-tier model (basic metering → distribution → full cost), catalog items, operator-first workflows, Rego policies, and the `koku-cost-provider` microservice design. |

## Key Concepts

**Three tiers of visibility**, each building on the previous:

1. **Basic Metering** — CPU, memory, and disk utilization/capacity. No cost
   model needed.
2. **Metering + Distribution** — Tier 1 plus OpenShift overhead categorization
   (control plane, platform projects, worker unallocated, storage/GPU/network
   unattributed) distributed across projects.
3. **Full Cost** — Tier 2 plus a price list. Every metric becomes a measurable
   quantity with a price: `cost = metering × rate`.

**Operator-first design.** The platform operator (the organization running the
data center) configures cost profiles and policies once. A bridge component
watches for new clusters via NATS and automatically creates cost-tracking
instances through DCM's standard catalog pipeline. Tenants consume metering
and cost data read-only.

**No special plumbing.** The cost service provider uses DCM's existing SP
contract — the same `POST` / `DELETE` / CloudEvent lifecycle as any other
provider. Koku's REST API handles all metering storage, cost calculation,
rate application, distribution, and reporting.

## Architecture Overview

```
Tenant provisions       ACM Cluster SP        Bridge (NATS consumer)
a cluster via DCM  ──►  creates cluster  ──►  sees READY event
                        on ACM hub            selects cost profile from labels
                                              creates cost instance via DCM catalog
                                                        │
                                              ┌─────────┘
                                              ▼
                                    DCM Catalog → Policy → SPRM
                                              │
                                              ▼
                                    koku-cost-provider
                                    creates Koku Source + Cost Model
                                    deploys metrics operator via ACM Policy
                                              │
                                              ▼
                                    Koku processes hourly metering data
                                    Tenants query cost SP for reports
```

## Related Projects

- **[Project Koku](https://github.com/project-koku/koku)** — Cost Management backend (Django, Celery, PostgreSQL)
- **[koku-metrics-operator](https://github.com/project-koku/koku-metrics-operator)** — OpenShift operator that collects Prometheus metrics and uploads to Koku
- **[DCM](https://dcm-project.github.io/)** — Data Center Management control plane

## License

Apache License 2.0
