# Metering and Cost Management for Sovereign Clouds

**How datacenter and sovereign cloud providers can monetize infrastructure services
without compromising data sovereignty**

*A Red Hat White Paper*

---

## Executive Summary

Sovereign cloud providers and enterprise datacenter operators face a paradox.
They build infrastructure to keep data, operations, and control within
jurisdictional borders — yet the tools they need to meter usage and bill for
services often require sending telemetry to external SaaS platforms, breaking
the very sovereignty guarantees they promise their customers.

This paper argues that **metering, costing, and chargeback are foundational
capabilities** for any provider that wants to monetize sovereign infrastructure.
It examines why existing FinOps tools fall short in sovereign and on-premise
environments, introduces an architecture that keeps all financial and usage data
inside the provider's perimeter, and shows how Red Hat Lightspeed Cost
Management and the DCM (Data Center Management) project together close the gap
between "we provision infrastructure" and "we get paid for it."

The audience is CIOs, CTOs, and technology leaders at sovereign cloud providers,
datacenter operators, managed service providers, and large enterprises that
operate an internal IT-as-a-service model for their divisions or subsidiaries.

---

## 1. The Sovereignty Imperative — and Its Blind Spot

Digital sovereignty has moved from a niche concern to a defining principle of
technology strategy. The numbers are unambiguous:

- **85%** of cloud decision-makers say sovereignty constraints completely or
  partially influence their choice of cloud vendor.¹
- **79%** of countries worldwide now have data protection and privacy
  legislation.²
- **50%** of European organizations plan to adopt sovereign cloud solutions,
  up from 31% in 2024.³
- The EU AI Factories initiative alone commits **EUR 12 billion** to
  sovereign AI infrastructure.⁴

Governments, banks, insurers, defense organizations, and critical infrastructure
operators are building or procuring sovereign clouds at an accelerating pace.
The regulatory drivers are well understood: GDPR, DORA, NIS2, the EU AI Act,
and their equivalents in Asia-Pacific, Latin America, and the Middle East.

But there is a blind spot. While the industry has invested heavily in sovereign
compute, storage, networking, and security controls, **the financial layer —
metering, costing, chargeback, and billing — has received far less attention.**

Most sovereign cloud projects begin with a focus on provisioning: "Can we deploy
clusters, VMs, and containers inside our borders?" The question of "How do we
know what was consumed, what it cost, and how to bill for it?" is often deferred
to a later phase — or addressed with tools that were never designed for
sovereign environments.

This gap is not merely an operational inconvenience. It is a business-model
risk. A sovereign cloud provider that cannot accurately meter and price its
services cannot sustain itself commercially. An internal IT organization that
cannot show business units what their infrastructure costs cannot justify its
budget. The sovereignty promise is incomplete without a sovereign
financial layer.

---

## 2. Why Existing FinOps Tools Fail in Sovereign Environments

The FinOps market offers a range of tools for cloud cost management:
Cloudability, Apptio, and others in the commercial space; KubeCost and OpenCost
in the open source ecosystem. These tools were designed for a world where
workloads run on public hyperscalers and telemetry flows freely to centralized
SaaS platforms.

In a sovereign cloud, this model breaks down in three ways.

### 2.1 Data Exfiltration by Design

Commercial FinOps platforms such as Cloudability and Apptio operate exclusively
as SaaS. They require that usage data — CPU hours, memory consumption, storage
volumes, network traffic, namespace metadata, and workload identifiers — be
transmitted to servers outside the provider's sovereign perimeter.

For a sovereign cloud provider, this is a fundamental conflict. Usage telemetry
is not just operational data; it reveals the structure, scale, and behavior of
workloads running inside the sovereign boundary. Transmitting it externally
violates the same data sovereignty principles the provider exists to uphold.
Under regulations like DORA and GDPR, it may also create compliance risk.

### 2.2 Insufficient Depth for Commercial Operations

Open source tools like KubeCost and OpenCost provide useful cost visibility
for Kubernetes environments and offer some multi-cluster capabilities.
However, they lack the depth required for a sovereign cloud provider to build
a commercial billing operation on top of OpenShift:

- **No overhead distribution.** They cannot categorize and allocate the cost of
  running the platform itself — control plane nodes, platform-level projects,
  unallocated worker capacity, storage overhead, GPU overhead, and network
  overhead — back to tenant workloads. Without this, the provider cannot
  account for the true cost of running the infrastructure.
- **Limited cost model flexibility.** They do not support the fine-grained
  metering and rating that a commercial provider needs: configurable
  per-core-hour, per-GB-month, per-PVC, per-VM-hour, or tiered rates with
  markup percentages. A provider serving multiple customers with different
  contract terms and pricing structures needs this flexibility.
- **No deep OpenShift awareness.** These tools treat OpenShift as generic
  Kubernetes. They lack native understanding of OpenShift-specific constructs
  such as OpenShift Virtualization VMs, operator-managed workloads, and the
  platform's overhead categorization model — all of which are essential for
  accurate metering in a sovereign OpenShift environment.

### 2.3 No Path from Metering to Billing

Even where an open source tool provides raw utilization data, it stops at the
boundary of metering. It does not answer the questions a provider needs to run
a business:

- What is the fully loaded cost of running Tenant A's workloads, including
  their share of platform overhead?
- How should I price a namespace, a VM, or a managed cluster for a customer
  who signed a 3-year contract with committed spend?
- What is my margin on each customer, and which services are underpriced?

These are the questions that separate a technology platform from a viable
business. Answering them requires a cost management system that goes beyond raw
metrics and supports configurable cost models, price lists, markup,
distribution rules, and exportable reports that can feed into billing
and ERP systems.

---

## 3. The Business Case: Why a CIO Should Care

Sovereign cloud infrastructure is expensive to build and operate. Hardware
procurement, datacenter facilities, power and cooling, staffing with
security-cleared personnel, compliance certification — these represent
significant capital and operating expenditure. Without a clear path to
monetization, the investment case is difficult to sustain.

### 3.1 Revenue Enablement

A sovereign cloud provider's revenue model depends entirely on its ability to
measure what was consumed and attach a price to it. Without metering, there is
no usage record. Without costing, there is no invoice. Without an invoice,
there is no revenue. The financial layer is not a nice-to-have — it is the
mechanism that turns infrastructure investment into a going concern.

The same logic applies to large enterprises that operate an internal
IT-as-a-service model. A multinational corporation, a government ministry, or
a conglomerate with multiple subsidiaries — each running workloads on shared
OpenShift infrastructure — faces the identical challenge: demonstrating what
each business unit consumed and what it cost. Without this, the central IT
organization cannot recover costs, justify its budget, or make credible
capacity-planning decisions. Whether the "customer" is an external client
paying an invoice or an internal division receiving a chargeback allocation,
the requirement is the same: accurate, auditable metering and costing.

### 3.2 Operational Visibility

Even before pricing is applied, metering data provides critical operational
insight. Understanding where compute capacity is consumed, which namespaces are
over-provisioned, and how platform overhead compares to tenant workloads enables
the provider to optimize infrastructure utilization and defer capital
expenditure.

A leading Swiss sovereign cloud provider found that introducing transparent
metering reduced customer onboarding time from weeks to hours and enabled a
self-service portal where tenants could see their own consumption in real
time — increasing customer satisfaction while reducing the provider's
support burden.

### 3.3 Compliance and Auditability

Regulators increasingly expect not just technical sovereignty but financial
transparency. DORA, for example, requires financial institutions to manage ICT
third-party risk, which includes understanding the cost and dependency
structure of cloud services. A sovereign cloud provider that can demonstrate
auditable metering and costing — with all data retained within the sovereign
perimeter — is better positioned to serve regulated customers.

### 3.4 Competitive Differentiation

The sovereign cloud market is growing rapidly, but so is competition. National
and regional providers in Europe, the Middle East, and Asia-Pacific are
building offerings at pace. A provider that can offer not just sovereign
infrastructure but also transparent, self-service cost management and
chargeback creates a differentiated customer experience that rivals the
financial transparency of the hyperscalers — without the sovereignty
trade-offs.

A UAE-based digital infrastructure company building a sovereign-by-design
private cloud found that integrating cost transparency into its platform from
day one was a key differentiator in winning government and regulated-industry
contracts, where procurement teams demanded full visibility into
per-workload costs.

---

## 4. The On-Premise Advantage: Keeping Financial Data Sovereign

The core requirement is straightforward: **all metering, costing, and financial
data must stay inside the sovereign perimeter.** This eliminates SaaS-only
FinOps tools from consideration and narrows the field to solutions that can be
deployed and operated entirely on-premise.

Red Hat Lightspeed Cost Management meets this requirement. It is 100% open
source, developed as the upstream project Koku, and available both as a managed
SaaS service on console.redhat.com and as a fully self-managed, on-premise
deployment. The on-premise mode is specifically designed for environments where
data must not leave the provider's infrastructure. Because it is open source,
sovereign cloud providers gain the same transparency and auditability for their
financial layer that they expect from the rest of their technology stack — no
proprietary black boxes in the cost pipeline.

### What It Measures

Red Hat Lightspeed Cost Management provides three tiers of visibility, each
building on the previous:

**Tier 1 — Basic Metering.** Without any configuration beyond deploying the
metrics operator on each OpenShift cluster, the system collects and reports raw
utilization and capacity data: CPU, memory, and storage. This provides
immediate visibility into what is being consumed across the fleet.

**Tier 2 — Metering with Distribution.** By attaching a cost model to a
cluster (without a price list), the system activates its overhead distribution
engine. The cost of running OpenShift itself — control plane, platform
projects, worker unallocated capacity, storage, GPU, and network overhead — is
categorized and distributed proportionally across tenant projects. This answers
the question "Where does the platform overhead go?" — essential for any
provider that needs to understand the fully loaded cost of serving each
tenant.

**Tier 3 — Full Cost Management.** By adding a price list to the cost model,
the system applies rates to every measurable dimension: CPU core-hours, memory
GB-hours, persistent volume claims, node utilization, VM hours, and more. The
provider can define different rate cards for different customers, apply markup
percentages, and export cost reports suitable for feeding into billing and
ERP systems. This is the tier that enables the transition from "we know what
was used" to "we know what to charge."

### Why It Leads the Market for On-Premise OpenShift

For sovereign cloud providers running OpenShift — which, as the industry's
leading enterprise Kubernetes platform, is the natural foundation for sovereign
infrastructure — Red Hat Lightspeed Cost Management offers capabilities that
no competing tool matches in an on-premise deployment:

- **Deep OpenShift integration.** The metrics operator is purpose-built for
  OpenShift, collecting data directly from Prometheus and Thanos with full
  awareness of OpenShift constructs: projects, nodes, persistent volumes,
  OpenShift Virtualization VMs, operators, and GPU workloads.
- **Overhead distribution.** No other on-premise tool can categorize and
  distribute platform overhead (control plane, platform projects, worker
  unallocated, storage, GPU, network) back to tenant projects.
- **Configurable cost models.** Providers can define per-cluster cost models
  with tiered rates, tag-based rates, markup, and distribution rules that
  match their commercial agreements.
- **Multi-cluster aggregation.** A single instance manages cost data from
  an entire fleet of clusters, providing a unified view across the
  provider's infrastructure.
- **Complete data residency.** All metering data, cost calculations, and
  reports are stored and processed locally. No telemetry leaves the
  sovereign perimeter.
- **100% open source.** The entire stack — from the metrics operator to the
  cost engine — is open source (Project Koku). Providers can inspect, audit,
  and modify any component, ensuring the same level of transparency in the
  financial layer that sovereignty demands from the rest of the stack.

---

## 5. From Metering to Monetization: The DCM Approach

Metering and costing solve the financial visibility problem. But a sovereign
cloud provider also needs to solve the operational problem: how to provision
infrastructure, enforce policies, manage a service catalog, and integrate cost
tracking as a seamless part of the infrastructure lifecycle.

This is where the DCM (Data Center Management) project enters the picture.

### 5.1 What Is DCM?

DCM is an open source, API-first control plane that provides a
hyperscaler-like cloud experience for on-premise and sovereign cloud
infrastructure. It is not a provisioning engine — it is a routing, governance,
and lifecycle management framework that delegates actual provisioning to
pluggable service providers.

DCM provides the capabilities that sovereign cloud operators need to run a
self-service platform:

- **Service Catalog.** A curated catalog of infrastructure offerings
  (clusters, VMs, namespaces, databases) with validated configurations and
  constraints. Tenants request services from the catalog; the platform
  provisions them according to policy.
- **Policy Engine.** An Open Policy Agent (OPA)-based governance layer that
  enforces rules at every decision point: who can request what, which
  provider handles the request, what size limits apply, and what compliance
  constraints must be met.
- **Provider Abstraction.** Infrastructure provisioning is delegated to
  service providers — pluggable microservices that implement a standard HTTP
  contract. This allows the same platform to support multiple provisioning
  backends (e.g., Red Hat Advanced Cluster Management for clusters, OpenShift
  Virtualization for VMs) without coupling the control plane to any
  specific technology.
- **Lifecycle Management.** DCM tracks every provisioned resource through its
  lifecycle — from request to provisioning to operational monitoring to
  decommissioning — providing a single source of truth for what exists in
  the provider's infrastructure.

### 5.2 Closing the Loop: Cost as a First-Class Service

The integration of Red Hat Lightspeed Cost Management with DCM closes the loop
between provisioning and monetization. When DCM provisions a new cluster for
a tenant, the cost management integration automatically:

1. **Registers the cluster** for metering — the metrics operator begins
   collecting utilization data within minutes.
2. **Applies the appropriate cost model** based on the tenant's contract tier
   and the catalog item they selected.
3. **Begins tracking costs** from the moment the cluster is operational, with
   no manual setup required.

When the cluster is decommissioned, the integration cleanly removes the cost
model and stops billing — again, automatically.

This automation is critical at scale. A sovereign cloud provider operating
dozens or hundreds of clusters cannot rely on manual processes to configure
metering for each one. The DCM integration ensures that every provisioned
resource is immediately metered and costed, eliminating the gap between
"the cluster exists" and "we are tracking what it costs."

### 5.3 Three Operating Models

The architecture supports three operating models, matching the diversity of
sovereign cloud deployments:

**Fully Automated.** A policy-driven model where every cluster provisioned
through DCM is automatically enrolled for metering and costing. The platform
operator configures the rules once; the system applies them consistently. This
is the recommended model for providers at scale.

**Operator-Directed.** The platform operator manually selects which clusters
to enroll and which cost model to apply, using DCM's catalog as an interface.
Suitable for smaller deployments or environments where cost tracking is
introduced incrementally.

**Tenant Self-Service.** Tenants can view their own metering and cost data
through read-only dashboards, understanding their consumption without needing
access to the underlying cost management system. This model mirrors the
cost transparency that tenants expect from hyperscaler environments.

---

## 6. Real-World Applications

Sovereign cloud providers and datacenter operators across multiple regions and
industries are already finding that the combination of on-premise cost
management and sovereign infrastructure orchestration addresses
critical business needs.

### Sovereign Cloud Providers

A leading Swiss sovereign cloud provider built its platform on Red Hat
OpenShift, offering public and private cloud options to customers in regulated
sectors including finance, healthcare, and government. By integrating cost
transparency directly into its self-service portal, the provider reduced
customer onboarding time and enabled tenants to monitor their own consumption
in real time — a capability that was decisive in winning contracts where
procurement teams demanded full audit trails of resource usage.

An Indian sovereign AI infrastructure provider built a sovereign AI factory
on Red Hat technologies, serving government entities and businesses that
require localized, high-performance AI infrastructure with data that remains
within the country's borders. For this provider, the ability to meter GPU
utilization and AI workload consumption — without transmitting that sensitive
operational data to external platforms — was a non-negotiable requirement.

### Regulated Financial Services

A leading European bank is running a sovereign AI factory platform built on
Red Hat OpenShift AI, delivering GPU-as-a-Service and LLM-as-a-Service across
the group. The platform serves a large and diverse internal user base. To
sustain the investment and allocate costs fairly across business units, the
bank requires granular metering of GPU hours, model inference requests, and
storage consumption — all retained within its own infrastructure.

A Turkish financial institution with more than 150 data scientists operates a
sovereign model development environment on Red Hat technologies. The
institution needs to track and allocate the cost of compute consumed by each
team and project, enabling internal chargeback and budget governance while
meeting the country's strict data sovereignty requirements.

### Defense and Government

Defense organizations represent perhaps the most demanding use case for
sovereign cost management. A European air force deployed Red Hat OpenShift on
portable edge datacenters to maintain operations during network outages,
running AI workloads locally in disconnected environments. In these settings,
every aspect of the technology stack — including metering and cost tracking —
must operate autonomously without any external dependency.

Government agencies across Europe are building sovereign cloud infrastructure
aligned with national strategies: France's SecNumCloud, Germany's BSI C5,
Italy's Polo Strategico Nazionale, Spain's National Cloud Strategy, and Nordic
"Plan B" hybrid architectures. Each of these initiatives requires the
financial layer to remain within the sovereign boundary, making on-premise
cost management an architectural requirement rather than a preference.

### Internal IT-as-a-Service

The same architecture applies to large enterprises operating an internal
shared-services model. A multinational corporation running OpenShift clusters
for multiple divisions or subsidiaries needs to answer the same questions as
an external provider: What did each business unit consume? What did it cost?
How should the infrastructure budget be allocated?

Red Hat Lightspeed Cost Management's overhead distribution capability is
particularly valuable here. It enables the central IT organization to
transparently allocate not just direct workload costs but also the shared
platform overhead — control plane, monitoring, security infrastructure —
proportionally to each business unit based on their actual usage. This
transforms the IT budget conversation from "IT costs too much" to "Here is
exactly what each division's infrastructure costs, and here is why."

---

## 7. Architecture at a Glance

The following diagram illustrates the high-level architecture of a sovereign
cloud with integrated metering and cost management.

```
┌─────────────────────────────────────────────────────────────────────┐
│                     SOVEREIGN PERIMETER                            │
│                                                                     │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────────────┐  │
│  │   Tenants    │    │   Platform   │    │   Billing / ERP      │  │
│  │  (self-svc)  │    │  Operator    │    │   Systems            │  │
│  └──────┬───────┘    └──────┬───────┘    └──────────▲───────────┘  │
│         │                   │                       │              │
│         ▼                   ▼                       │              │
│  ┌─────────────────────────────────────┐   Cost     │              │
│  │           DCM Control Plane         │   Reports  │              │
│  │  ┌────────┐ ┌────────┐ ┌────────┐  │            │              │
│  │  │Catalog │ │ Policy │ │  SP    │  │            │              │
│  │  │Manager │ │ Engine │ │Manager │  │            │              │
│  │  └────────┘ └────────┘ └───┬────┘  │            │              │
│  └────────────────────────────┼───────┘            │              │
│                               │                     │              │
│              ┌────────────────┼─────────────┐       │              │
│              ▼                ▼              ▼       │              │
│  ┌──────────────┐  ┌──────────────┐  ┌─────────────┴──────────┐  │
│  │   Cluster    │  │     VM       │  │ Red Hat Lightspeed     │  │
│  │   Service    │  │   Service    │  │ Cost Management        │  │
│  │   Provider   │  │   Provider   │  │ (on-premise)           │  │
│  │  (e.g. ACM)  │  │ (e.g. OCP-V)│  │                        │  │
│  └──────┬───────┘  └──────┬───────┘  │  ┌──────────────────┐  │  │
│         │                 │          │  │  Cost Models      │  │  │
│         ▼                 ▼          │  │  Price Lists      │  │  │
│  ┌──────────────────────────────┐    │  │  Distribution     │  │  │
│  │     OpenShift Clusters       │    │  │  Reports          │  │  │
│  │  ┌────────┐    ┌────────┐   │    │  └──────────────────┘  │  │
│  │  │Metrics │    │Metrics │   │    │           ▲             │  │
│  │  │Operator│    │Operator│   │    └───────────┼─────────────┘  │
│  │  └───┬────┘    └───┬────┘   │               │                │
│  │      │             │        │    Metering    │                │
│  │      └─────────────┼────────┼────────────────┘                │
│  │                    │        │                                  │
│  └────────────────────┘────────┘                                  │
│                                                                     │
│         ❌ No data leaves the sovereign perimeter                  │
└─────────────────────────────────────────────────────────────────────┘
```

Key characteristics of this architecture:

- **All components run on-premise** within the provider's infrastructure.
- **Metering data flows locally** from OpenShift clusters to the cost
  management system via the metrics operator — no external endpoints.
- **Cost models and reports are stored locally** in the provider's database.
- **DCM orchestrates the lifecycle**, ensuring every provisioned resource is
  automatically enrolled for metering and costing.
- **Cost reports can feed local billing/ERP systems**, completing the path
  from metering to monetization.
- **The sovereign perimeter is never breached** for financial or
  operational data.

---

## 8. Getting Started

The path from sovereign infrastructure to sovereign monetization is
incremental. Organizations do not need to implement all three tiers
simultaneously.

**Start with metering.** Deploy the metrics operator on existing OpenShift
clusters. Within minutes, Tier 1 basic metering provides visibility into CPU,
memory, and storage consumption across the fleet. This alone delivers value:
operational insight, capacity planning data, and evidence of utilization for
budget discussions.

**Add distribution.** Attach cost models to clusters to activate Tier 2
overhead distribution. This reveals the true cost structure of the platform
and enables the provider to understand — and communicate to tenants — the
fully loaded cost of their workloads.

**Enable full costing.** Add price lists to cost models to activate Tier 3.
Define rate cards that match commercial agreements, apply markup, and generate
cost reports suitable for billing. This is the tier that enables revenue
recovery.

**Automate with DCM.** Integrate the cost management system with DCM to
automate enrollment, cost model assignment, and lifecycle management. This
eliminates manual processes and ensures consistent financial tracking across
the entire fleet.

Each tier delivers standalone value. The progression is additive, not
disruptive. A provider can begin with basic metering today and evolve toward
full cost management as its commercial model matures.

---

## 9. Conclusion

The sovereign cloud market is growing rapidly, driven by regulation, geopolitics,
and the strategic imperative for digital autonomy. Providers are investing
heavily in sovereign compute, storage, networking, and security. But without a
sovereign financial layer — metering, costing, chargeback, and billing that
operates entirely within the sovereign perimeter — the business model
remains incomplete.

Existing FinOps tools were designed for the public cloud era. SaaS-only
platforms cannot operate within sovereign boundaries. Open source alternatives
lack the depth for commercial billing operations. The gap between
infrastructure provisioning and infrastructure monetization is a real and
present challenge for every sovereign cloud provider.

Red Hat Lightspeed Cost Management and the DCM project close this gap. Together,
they provide a complete, on-premise architecture that takes a sovereign cloud
from provisioning through metering, costing, overhead distribution, and
reporting — all without a single byte of financial or operational data
leaving the sovereign perimeter.

For the CIO of a sovereign cloud provider or a datacenter operator, this is
not a technology decision. It is a business-model decision. The question is
not whether you need metering and cost management, but how quickly you can
implement it — and whether you can afford the sovereignty trade-offs of the
alternatives.

---

## References

1. Forrester, *The Sovereign Cloud Platform Landscape, Q4 2025*, October 2025.
2. UN Trade and Development, *Privacy and Data Protection Legislation
   Worldwide*, February 2025.
3. IDC InfoBrief, *Digital Sovereignty in Action: Building Resilient, Compliant,
   and Transparent Cloud Ecosystems*, Document #EUR153367225, August 2025.
4. IDC Market Perspective, *Beyond Cost and Compliance: How Red Hat Is Shaping
   AI, Sovereignty, and Modernization*, Document #EUR153890425, November 2025.
5. Gartner, *Predicts 70% of Enterprises Adopting GenAI Will Cite
   Sustainability and Digital Sovereignty as Top Criteria for Selecting Between
   Different Public Cloud GenAI Services by 2027*, February 2024.
6. Linux Foundation, *The State of Sovereign AI: Exploring the Role of Open
   Source Projects and Global Collaboration in Global AI Strategy*, August 2025.

---

*Red Hat, the Red Hat logo, OpenShift, and Ansible are trademarks or registered
trademarks of Red Hat, Inc. or its subsidiaries in the United States and other
countries. Linux is the registered trademark of Linus Torvalds in the U.S. and
other countries. All other trademarks are the property of their respective
owners.*

*© 2026 Red Hat, Inc.*
