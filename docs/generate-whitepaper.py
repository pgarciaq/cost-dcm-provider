#!/usr/bin/env python3
"""Generate white paper DOCX and PDF from Red Hat Formal Document template."""
import shutil, os, zipfile, subprocess
from pathlib import Path
from xml.sax.saxutils import escape

SCRIPT_DIR = Path(__file__).parent
TEMPLATE = Path("/home/pgarciaq/rh/templates/General _ Formal document template.docx")
OUTPUT = SCRIPT_DIR / "Metering-and-Cost-Management-for-Sovereign-Clouds.docx"
ARCH_IMG = SCRIPT_DIR / "architecture-diagram.png"
WORK = Path("/tmp/rh-wp-build")

# Image dimensions from PNG header
def png_dimensions(path):
    with open(path, "rb") as f:
        f.read(16)
        w = int.from_bytes(f.read(4), "big")
        h = int.from_bytes(f.read(4), "big")
    return w, h

img_w, img_h = png_dimensions(ARCH_IMG)
CONTENT_WIDTH_EMU = 5943600  # 6.5 inches
scale = CONTENT_WIDTH_EMU / (img_w * 9525) * 0.92  # 92% of content width to fit caption on same page
cx = int(img_w * 9525 * scale)
cy = int(img_h * 9525 * scale)

# XML helpers using the template's style IDs
NS = 'xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture" xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml"'

para_id = [0]
def next_id():
    para_id[0] += 1
    return f"{para_id[0]:08x}"

def p(style=None, runs_xml="", extra_ppr="", break_before=False):
    pid = next_id()
    ppr_parts = []
    if style:
        ppr_parts.append(f'<w:pStyle w:val="{style}"/>')
    if break_before:
        ppr_parts.append('<w:pageBreakBefore w:val="1"/>')
    if extra_ppr:
        ppr_parts.append(extra_ppr)
    ppr = f"<w:pPr>{''.join(ppr_parts)}</w:pPr>" if ppr_parts else ""
    return f'<w:p w14:paraId="{pid}">{ppr}{runs_xml}</w:p>'

def r(text, bold=False, italic=False, color=None, sz=None, font=None, superscript=False):
    rpr_parts = []
    if font:
        rpr_parts.append(f'<w:rFonts w:ascii="{font}" w:hAnsi="{font}" w:cs="{font}" w:eastAsia="{font}"/>')
    if bold:
        rpr_parts.append('<w:b w:val="1"/><w:bCs w:val="1"/>')
    if italic:
        rpr_parts.append('<w:i w:val="1"/><w:iCs w:val="1"/>')
    if color:
        rpr_parts.append(f'<w:color w:val="{color}"/>')
    if sz:
        rpr_parts.append(f'<w:sz w:val="{sz}"/><w:szCs w:val="{sz}"/>')
    if superscript:
        rpr_parts.append('<w:vertAlign w:val="superscript"/>')
    rpr = f'<w:rPr>{"".join(rpr_parts)}</w:rPr>' if rpr_parts else ""
    t = escape(text)
    sp = ' xml:space="preserve"' if text.startswith(" ") or text.endswith(" ") else ""
    return f"<w:r>{rpr}<w:t{sp}>{t}</w:t></w:r>"

def page_break():
    pid = next_id()
    return f'<w:p w14:paraId="{pid}"><w:r><w:br w:type="page"/></w:r></w:p>'

def heading1(text, break_before=False):
    return p("Heading1", r(text), break_before=break_before)

def heading2(text, break_before=False):
    return p("Heading2", r(text, bold=True), break_before=break_before)

def heading3(text):
    return p("Heading3", r(text))

def body_para(*runs):
    return p(runs_xml="".join(runs))

def body_text(text):
    return body_para(r(text))

def bold_intro(label, rest):
    return body_para(r(label, bold=True), r(rest))

def stat_bullet(stat, rest):
    return p(runs_xml=r(stat, bold=True, color="ee0000") + r(rest),
             extra_ppr='<w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr>')

def bullet(text):
    runs = r(text) if isinstance(text, str) else text
    return p(runs_xml=runs,
             extra_ppr='<w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr>')

def bullet_bold(label, rest):
    return p(runs_xml=r(label, bold=True) + r(rest),
             extra_ppr='<w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr>')

def numbered(num_id, text_runs):
    return p(runs_xml=text_runs,
             extra_ppr=f'<w:numPr><w:ilvl w:val="0"/><w:numId w:val="4"/></w:numPr>')

def figure_caption(text):
    return p(runs_xml=r(text, italic=True, color="666666", sz="18"),
             extra_ppr='<w:jc w:val="center"/>')

def image_para(rid, cx, cy):
    pid = next_id()
    return f'''<w:p w14:paraId="{pid}"><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:drawing>
<wp:inline distT="0" distB="0" distL="0" distR="0">
<wp:extent cx="{cx}" cy="{cy}"/>
<wp:docPr id="100" name="arch-diagram"/>
<a:graphic><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">
<pic:pic><pic:nvPicPr><pic:cNvPr id="0" name="architecture-diagram.png"/>
<pic:cNvPicPr preferRelativeResize="0"/></pic:nvPicPr>
<pic:blipFill><a:blip r:embed="{rid}"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill>
<pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="{cx}" cy="{cy}"/></a:xfrm>
<a:prstGeom prst="rect"/></pic:spPr></pic:pic>
</a:graphicData></a:graphic></wp:inline></w:drawing></w:r></w:p>'''

def empty():
    return p()

# Build the document body
parts = []

# === COVER PAGE ===
# Title
parts.append(p("Title", r("Metering and Cost\nManagement for\nSovereign Clouds".replace("\n", ""),
               sz="72")))
# Fix: use line breaks in the title
parts[-1] = f'''<w:p w14:paraId="{next_id()}"><w:pPr><w:pStyle w:val="Title"/><w:spacing w:line="240" w:lineRule="auto"/></w:pPr>
<w:r><w:rPr><w:sz w:val="72"/><w:szCs w:val="72"/></w:rPr>
<w:t>Metering and Cost</w:t><w:br w:type="textWrapping"/>
<w:t>Management for</w:t><w:br w:type="textWrapping"/>
<w:t>Sovereign Clouds</w:t></w:r></w:p>'''

# Subtitle
parts.append(p("Subtitle", r("How datacenter and sovereign cloud providers can monetize infrastructure services without compromising data sovereignty")))

# Author & date
parts.append(empty())
parts.append(body_text("A Red Hat White Paper"))
parts.append(body_text("2026"))

# Contents heading + manual TOC entries
parts.append(page_break())
parts.append(heading1("Contents"))

toc_entries = [
    (1, "Executive Summary"),
    (1, "1. The Sovereignty Imperative \u2014 and Its Blind Spot"),
    (1, "2. Why Existing FinOps Tools Fail in Sovereign Environments"),
    (2, "2.1 Data Exfiltration by Design"),
    (2, "2.2 Insufficient Depth for Commercial Operations"),
    (2, "2.3 No Path from Metering to Billing"),
    (1, "3. The Business Case: Why a CIO Should Care"),
    (2, "3.1 Revenue Enablement"),
    (2, "3.2 Operational Visibility"),
    (2, "3.3 Compliance and Auditability"),
    (2, "3.4 Competitive Differentiation"),
    (1, "4. The On-Premise Advantage: Keeping Financial Data Sovereign"),
    (2, "What It Measures"),
    (2, "Why It Leads the Market"),
    (1, "5. From Metering to Monetization: The DCM Approach"),
    (2, "5.1 What Is DCM?"),
    (2, "5.2 Closing the Loop: Cost as a First-Class Service"),
    (2, "5.3 Three Operating Models"),
    (1, "6. Real-World Applications"),
    (2, "Sovereign Cloud Providers"),
    (2, "Regulated Financial Services"),
    (2, "Defense and Government"),
    (2, "Internal IT-as-a-Service"),
    (1, "7. Architecture at a Glance"),
    (1, "8. Getting Started"),
    (1, "9. Conclusion"),
    (1, "References"),
]

def toc_entry(level, title):
    indent = 0 if level == 1 else 360
    sz = "22" if level == 1 else "20"
    bold = level == 1
    pid = next_id()
    rpr = '<w:rPr>'
    if bold:
        rpr += '<w:b w:val="1"/><w:bCs w:val="1"/>'
    rpr += f'<w:sz w:val="{sz}"/><w:szCs w:val="{sz}"/>'
    rpr += '</w:rPr>'
    t = escape(title)
    indent_xml = f'<w:ind w:left="{indent}"/>' if indent else ""
    return f'''<w:p w14:paraId="{pid}"><w:pPr><w:spacing w:after="60" w:line="276" w:lineRule="auto"/>{indent_xml}<w:rPr><w:sz w:val="{sz}"/><w:szCs w:val="{sz}"/></w:rPr></w:pPr><w:r>{rpr}<w:t>{t}</w:t></w:r></w:p>'''

for lvl, title in toc_entries:
    parts.append(toc_entry(lvl, title))

# === EXECUTIVE SUMMARY ===
parts.append(page_break())
parts.append(heading1("Executive Summary"))
parts.append(body_para(
    r("Sovereign cloud providers and enterprise datacenter operators face a paradox. They build infrastructure to keep data, operations, and control within jurisdictional borders \u2014 yet the tools they need to meter usage and bill for services often require sending telemetry to external SaaS platforms, breaking the very sovereignty guarantees they promise their customers.")
))
parts.append(body_para(
    r("This paper argues that "),
    r("metering, costing, and chargeback are foundational capabilities", bold=True),
    r(" for any provider that wants to monetize sovereign infrastructure. It examines why existing FinOps tools fall short in sovereign and on-premise environments, introduces an architecture that keeps all financial and usage data inside the provider\u2019s perimeter, and shows how Red Hat Lightspeed Cost Management and the DCM (Data Center Management) project together close the gap between \u201Cwe provision infrastructure\u201D and \u201Cwe get paid for it.\u201D Red Hat Lightspeed Cost Management offers more than 40 cost dimensions, is the only tool that supports metering and costing of OpenShift on IBM Z, LinuxOne, and POWER, and makes all metering and cost data available via REST API for integration with any billing, ERP, or business intelligence system."),
))
parts.append(body_text("The audience is CIOs, CTOs, and technology leaders at sovereign cloud providers, datacenter operators, managed service providers, and large enterprises that operate an internal IT-as-a-service model for their divisions or subsidiaries."))

# === SECTION 1 ===
parts.append(heading1("1. The Sovereignty Imperative \u2014 and Its Blind Spot"))
parts.append(body_text("Digital sovereignty has moved from a niche concern to a defining principle of technology strategy. The numbers are unambiguous:"))
parts.append(stat_bullet("85% ", "of cloud decision-makers say sovereignty constraints completely or partially influence their choice of cloud vendor.\u00B9"))
parts.append(stat_bullet("79% ", "of countries worldwide now have data protection and privacy legislation.\u00B2"))
parts.append(stat_bullet("50% ", "of European organizations plan to adopt sovereign cloud solutions, up from 31% in 2024.\u00B3"))
parts.append(stat_bullet("EUR 12 billion ", "\u2014 the EU AI Factories initiative commitment to sovereign AI infrastructure.\u2074"))
parts.append(body_text("Governments, banks, insurers, defense organizations, and critical infrastructure operators are building or procuring sovereign clouds at an accelerating pace. The regulatory drivers are well understood: GDPR, DORA, NIS2, the EU AI Act, and their equivalents in Asia-Pacific, Latin America, and the Middle East."))
parts.append(body_para(
    r("But there is a blind spot. While the industry has invested heavily in sovereign compute, storage, networking, and security controls, "),
    r("the financial layer \u2014 metering, costing, chargeback, and billing \u2014 has received far less attention.", bold=True),
))
parts.append(body_text("Most sovereign cloud projects begin with a focus on provisioning: \u201CCan we deploy clusters, VMs, and containers inside our borders?\u201D The question of \u201CHow do we know what was consumed, what it cost, and how to bill for it?\u201D is often deferred to a later phase \u2014 or addressed with tools that were never designed for sovereign environments."))
parts.append(body_text("This gap is not merely an operational inconvenience. It is a business-model risk. A sovereign cloud provider that cannot accurately meter and price its services cannot sustain itself commercially. An internal IT organization that cannot show business units what their infrastructure costs cannot justify its budget. The sovereignty promise is incomplete without a sovereign financial layer."))

# === SECTION 2 ===
parts.append(heading1("2. Why Existing FinOps Tools Fail in Sovereign Environments"))
parts.append(body_text("The FinOps market offers a range of tools for cloud cost management: Cloudability, Apptio, and others in the commercial space; KubeCost and OpenCost in the open source ecosystem. These tools were designed for a world where workloads run on public hyperscalers and telemetry flows freely to centralized SaaS platforms."))
parts.append(body_text("In a sovereign cloud, this model breaks down in three ways."))

parts.append(heading2("2.1 Data Exfiltration by Design"))
parts.append(body_text("Commercial FinOps platforms such as Cloudability and Apptio operate exclusively as SaaS. They require that usage data \u2014 CPU hours, memory consumption, storage volumes, network traffic, namespace metadata, and workload identifiers \u2014 be transmitted to servers outside the provider\u2019s sovereign perimeter."))
parts.append(body_text("For a sovereign cloud provider, this is a fundamental conflict. Usage telemetry is not just operational data; it reveals the structure, scale, and behavior of workloads running inside the sovereign boundary. Transmitting it externally violates the same data sovereignty principles the provider exists to uphold. Under regulations like DORA and GDPR, it may also create compliance risk."))

parts.append(heading2("2.2 Insufficient Depth for Commercial Operations"))
parts.append(body_text("Open source tools like KubeCost and OpenCost provide useful cost visibility for Kubernetes environments and offer some multi-cluster capabilities. However, they lack the depth required for a sovereign cloud provider to build a commercial billing operation on top of OpenShift:"))
parts.append(bullet_bold("No overhead distribution. ", "They cannot categorize and allocate the cost of running the platform itself \u2014 control plane nodes, platform-level projects, unallocated worker capacity, storage overhead, GPU overhead, and network overhead \u2014 back to tenant workloads."))
parts.append(bullet_bold("Limited cost model flexibility. ", "They do not support the fine-grained metering and rating that a commercial provider needs: configurable per-core-hour, per-GB-month, per-PVC, per-VM-hour, or tiered rates with markup percentages."))
parts.append(bullet_bold("No deep OpenShift awareness. ", "These tools treat OpenShift as generic Kubernetes. They lack native understanding of OpenShift-specific constructs such as OpenShift Virtualization VMs, operator-managed workloads, and the platform\u2019s overhead categorization model."))
parts.append(bullet_bold("No support for non-x86 architectures. ", "These tools do not support OpenShift on IBM Z, LinuxOne, or POWER \u2014 platforms that are foundational in regulated banking, government, and mainframe-centric sovereign environments. Providers whose customers run workloads on these architectures have no open source FinOps option other than Red Hat Lightspeed Cost Management."))

parts.append(heading2("2.3 No Path from Metering to Billing"))
parts.append(body_text("Even where an open source tool provides raw utilization data, it stops at the boundary of metering. It does not answer the questions a provider needs to run a business:"))
parts.append(bullet("What is the fully loaded cost of running Tenant A\u2019s workloads, including their share of platform overhead?"))
parts.append(bullet("How should I price a namespace, a VM, or a managed cluster for a customer who signed a 3-year contract with committed spend?"))
parts.append(bullet("What is my margin on each customer, and which services are underpriced?"))
parts.append(body_text("These are the questions that separate a technology platform from a viable business. Answering them requires a cost management system that goes beyond raw metrics and supports configurable cost models, price lists, markup, distribution rules, and exportable reports that can feed into billing and ERP systems."))

# === SECTION 3 ===
parts.append(heading1("3. The Business Case: Why a CIO Should Care"))
parts.append(body_text("Sovereign cloud infrastructure is expensive to build and operate. Hardware procurement, datacenter facilities, power and cooling, staffing with security-cleared personnel, compliance certification \u2014 these represent significant capital and operating expenditure. Without a clear path to monetization, the investment case is difficult to sustain."))

parts.append(heading2("3.1 Revenue Enablement"))
parts.append(body_text("A sovereign cloud provider\u2019s revenue model depends entirely on its ability to measure what was consumed and attach a price to it. Without metering, there is no usage record. Without costing, there is no invoice. Without an invoice, there is no revenue. The financial layer is not a nice-to-have \u2014 it is the mechanism that turns infrastructure investment into a going concern."))
parts.append(body_text("The same logic applies to large enterprises that operate an internal IT-as-a-service model. A multinational corporation, a government ministry, or a conglomerate with multiple subsidiaries \u2014 each running workloads on shared OpenShift infrastructure \u2014 faces the identical challenge: demonstrating what each business unit consumed and what it cost."))
parts.append(body_text("Critically, all metering and cost data is available via a comprehensive REST API, enabling direct integration with any billing platform, ERP system, or business intelligence tool \u2014 Excel, Power BI, Tableau, Grafana, and others \u2014 so the financial data stays sovereign while still powering the provider\u2019s entire commercial stack."))

parts.append(heading2("3.2 Operational Visibility"))
parts.append(body_text("Even before pricing is applied, metering data provides critical operational insight. Understanding where compute capacity is consumed, which namespaces are over-provisioned, and how platform overhead compares to tenant workloads enables the provider to optimize infrastructure utilization and defer capital expenditure."))
parts.append(body_text("A leading Swiss sovereign cloud provider found that introducing transparent metering reduced customer onboarding time from weeks to hours and enabled a self-service portal where tenants could see their own consumption in real time \u2014 increasing customer satisfaction while reducing the provider\u2019s support burden."))

parts.append(heading2("3.3 Compliance and Auditability"))
parts.append(body_text("Regulators increasingly expect not just technical sovereignty but financial transparency. DORA, for example, requires financial institutions to manage ICT third-party risk, which includes understanding the cost and dependency structure of cloud services. A sovereign cloud provider that can demonstrate auditable metering and costing \u2014 with all data retained within the sovereign perimeter \u2014 is better positioned to serve regulated customers."))

parts.append(heading2("3.4 Competitive Differentiation"))
parts.append(body_text("The sovereign cloud market is growing rapidly, but so is competition. National and regional providers in Europe, the Middle East, and Asia-Pacific are building offerings at pace. A provider that can offer not just sovereign infrastructure but also transparent, self-service cost management and chargeback creates a differentiated customer experience that rivals the financial transparency of the hyperscalers \u2014 without the sovereignty trade-offs."))
parts.append(body_text("The differentiation extends to technology coverage. Red Hat Lightspeed Cost Management is the only metering and cost management tool \u2014 open source or commercial \u2014 that supports OpenShift on IBM Z, LinuxOne, and POWER. These platforms remain essential in sovereign banking, government, and mainframe-centric environments. No competing FinOps tool offers this coverage, making it a decisive factor for providers whose customers run workloads on these architectures."))

# === SECTION 4 ===
parts.append(heading1("4. The On-Premise Advantage: Keeping Financial Data Sovereign"))
parts.append(body_para(
    r("The core requirement is straightforward: "),
    r("all metering, costing, and financial data must stay inside the sovereign perimeter.", bold=True),
    r(" This eliminates SaaS-only FinOps tools from consideration and narrows the field to solutions that can be deployed and operated entirely on-premise."),
))
parts.append(body_text("Red Hat Lightspeed Cost Management meets this requirement. It is 100% open source, developed as the upstream project Koku, and available both as a managed SaaS service on console.redhat.com and as a fully self-managed, on-premise deployment. The on-premise mode is specifically designed for environments where data must not leave the provider\u2019s infrastructure. Because it is open source, sovereign cloud providers gain the same transparency and auditability for their financial layer that they expect from the rest of their technology stack."))

parts.append(heading2("What It Measures"))
parts.append(body_text("Red Hat Lightspeed Cost Management provides three tiers of visibility, each building on the previous:"))
parts.append(bold_intro("Tier 1 \u2014 Basic Metering. ", "Without any configuration beyond deploying the metrics operator on each OpenShift cluster, the system collects and reports raw utilization and capacity data: CPU, memory, and storage. The metrics operator runs on all supported architectures \u2014 x86-64, ARM, IBM Z, LinuxOne, and POWER \u2014 making Red Hat Lightspeed Cost Management the only tool that can meter OpenShift across the full range of platforms found in sovereign environments."))
parts.append(bold_intro("Tier 2 \u2014 Metering with Distribution. ", "By attaching a cost model to a cluster, the system activates its overhead distribution engine. The cost of running OpenShift itself \u2014 control plane, platform projects, worker unallocated capacity, storage, GPU, and network overhead \u2014 is categorized and distributed proportionally across tenant projects."))
parts.append(bold_intro("Tier 3 \u2014 Full Cost Management. ", "By adding a price list to the cost model, the system applies rates across more than 40 cost dimensions: CPU core-hours (usage, request, and effective), memory GB-hours, storage GB-months, node cost per core-hour or per month, cluster cost per month, OpenShift Virtualization VM hours and VM core-hours, PVC months, project months, and GPU (physical devices and NVIDIA MIG slices) \u2014 with every metric parameterizable by tag for fine-grained allocation. All metering and cost data is available via a comprehensive REST API, enabling integration with any billing, ERP, or BI system."))

parts.append(heading2("Why It Leads the Market"))
parts.append(body_text("For sovereign cloud providers running OpenShift, Red Hat Lightspeed Cost Management offers capabilities that no competing tool matches in an on-premise deployment:"))
parts.append(bullet_bold("Deep OpenShift integration. ", "The metrics operator is purpose-built for OpenShift, collecting data directly from Prometheus and Thanos with full awareness of OpenShift constructs."))
parts.append(bullet_bold("Overhead distribution. ", "No other on-premise tool can categorize and distribute platform overhead back to tenant projects."))
parts.append(bullet_bold("Configurable cost models. ", "Providers can define per-cluster cost models with tiered rates, tag-based rates, markup, and distribution rules."))
parts.append(bullet_bold("Multi-cluster aggregation. ", "A single instance manages cost data from an entire fleet of clusters."))
parts.append(bullet_bold("Fine-grained RBAC. ", "The provider aggregates all clusters, VMs, namespaces, and projects in a single instance, then uses role-based access control to restrict what each user \u2014 or each sovereign cloud tenant \u2014 can see and do. A tenant given read access sees only their own namespaces and costs, never another tenant\u2019s data."))
parts.append(bullet_bold("Complete data residency. ", "All metering data, cost calculations, and reports are stored and processed locally."))
parts.append(bullet_bold("Multi-architecture. ", "The only FinOps tool \u2014 open source or commercial \u2014 that supports OpenShift on IBM Z, LinuxOne, and POWER, alongside x86-64 and ARM."))
parts.append(bullet_bold("GPU and AI workload metering. ", "Native support for GPU cost tracking (physical devices and NVIDIA MIG slices) and OpenShift AI subscription metering. Support for Model-as-a-Service inference costing and agentic AI workload attribution is in active development."))
parts.append(bullet_bold("Full API data export. ", "All metering and cost data is available via REST API, enabling integration with billing platforms, ERP systems, and BI tools without data leaving the sovereign perimeter."))
parts.append(bullet_bold("Cloud cost management. ", "Beyond on-premise OpenShift, it also supports cloud costs on Amazon Web Services, Microsoft Azure, and Google Cloud \u2014 any cloud service, including private offers and managed OpenShift (ROSA and ARO). For ROSA and ARO, the cost of OpenShift subscriptions is automatically factored in and distributed to workloads, giving hybrid sovereign environments a single pane of glass."))
parts.append(bullet_bold("Resource optimization. ", "Beyond cost tracking, the platform includes a resource optimization feature that provides rightsizing recommendations for containers, deployments, and jobs \u2014 offering both cost-optimized and performance-optimized options."))
parts.append(bullet_bold("100% open source. ", "The entire stack is open source (Project Koku). Providers can inspect, audit, and modify any component."))

# === SECTION 5 ===
parts.append(heading1("5. From Metering to Monetization: The DCM Approach"))
parts.append(body_text("Metering and costing solve the financial visibility problem. But a sovereign cloud provider also needs to solve the operational problem: how to provision infrastructure, enforce policies, manage a service catalog, and integrate cost tracking as a seamless part of the infrastructure lifecycle."))
parts.append(body_text("This is where the DCM (Data Center Management) project enters the picture."))

parts.append(heading2("5.1 What Is DCM?"))
parts.append(body_text("DCM is an open source, API-first control plane that provides a hyperscaler-like cloud experience for on-premise and sovereign cloud infrastructure. It is not a provisioning engine \u2014 it is a routing, governance, and lifecycle management framework that delegates actual provisioning to pluggable service providers."))
parts.append(bullet_bold("Service Catalog. ", "A curated catalog of infrastructure offerings with validated configurations and constraints."))
parts.append(bullet_bold("Policy Engine. ", "An Open Policy Agent (OPA)-based governance layer that enforces rules at every decision point."))
parts.append(bullet_bold("Provider Abstraction. ", "Infrastructure provisioning is delegated to service providers \u2014 pluggable microservices that implement a standard HTTP contract."))
parts.append(bullet_bold("Lifecycle Management. ", "DCM tracks every provisioned resource through its lifecycle \u2014 from request to decommissioning."))

parts.append(heading2("5.2 Closing the Loop: Cost as a First-Class Service"))
parts.append(body_text("The integration of Red Hat Lightspeed Cost Management with DCM closes the loop between provisioning and monetization. When DCM provisions a new cluster for a tenant, the cost management integration automatically:"))
parts.append(numbered(4, r("Registers the cluster", bold=True) + r(" for metering \u2014 the metrics operator begins collecting utilization data within minutes.")))
parts.append(numbered(4, r("Applies the appropriate cost model", bold=True) + r(" based on the tenant\u2019s contract tier and the catalog item they selected.")))
parts.append(numbered(4, r("Begins tracking costs", bold=True) + r(" from the moment the cluster is operational, with no manual setup required.")))
parts.append(body_text("This applies uniformly across all supported architectures \u2014 x86-64, ARM, IBM Z, LinuxOne, and POWER \u2014 and includes GPU overhead distribution for clusters equipped with GPUs. All cost data is accessible via the REST API, enabling DCM or downstream billing systems to query costs programmatically."))

parts.append(heading2("5.3 Three Operating Models"))
parts.append(bold_intro("Fully Automated. ", "A policy-driven model where every cluster provisioned through DCM is automatically enrolled for metering and costing. The platform operator configures the rules once; the system applies them consistently."))
parts.append(bold_intro("Operator-Directed. ", "The platform operator manually selects which clusters to enroll and which cost model to apply. Suitable for smaller deployments or environments where cost tracking is introduced incrementally."))
parts.append(bold_intro("Tenant Self-Service. ", "Tenants can view their own metering and cost data through read-only dashboards, understanding their consumption without needing access to the underlying cost management system. Fine-grained RBAC ensures each tenant sees only their own namespaces, clusters, and costs \u2014 never another tenant\u2019s data \u2014 preserving multi-tenant isolation."))

# === SECTION 6 ===
parts.append(heading1("6. Real-World Applications"))
parts.append(body_text("Sovereign cloud providers and datacenter operators across multiple regions and industries are already finding that the combination of on-premise cost management and sovereign infrastructure orchestration addresses critical business needs."))

parts.append(heading2("Sovereign Cloud Providers"))
parts.append(body_text("A leading Swiss sovereign cloud provider built its platform on Red Hat OpenShift, offering public and private cloud options to customers in regulated sectors including finance, healthcare, and government. By integrating cost transparency directly into its self-service portal, the provider reduced customer onboarding time and enabled tenants to monitor their own consumption in real time. Fine-grained RBAC ensures each tenant sees only their own namespaces and costs, preserving multi-tenant isolation. The provider uses the REST API to export cost data into its existing billing platform, enabling automated invoice generation without any data leaving its sovereign perimeter."))
parts.append(body_text("An Indian sovereign AI infrastructure provider built a sovereign AI factory on Red Hat technologies, serving government entities and businesses that require localized, high-performance AI infrastructure with data that remains within the country\u2019s borders. GPU metering enables the provider to bill customers per GPU-hour and per NVIDIA MIG slice, a granularity that no other on-premise FinOps tool provides."))

parts.append(heading2("Regulated Financial Services"))
parts.append(body_text("A leading European bank is running a sovereign AI factory platform built on Red Hat OpenShift AI, delivering GPU-as-a-Service and LLM-as-a-Service across the group. To sustain the investment and allocate costs fairly across business units, the bank requires granular metering of GPU hours, model inference requests, and storage consumption \u2014 all retained within its own infrastructure. The bank\u2019s core systems run on IBM Z; Red Hat Lightspeed Cost Management is the only tool that can meter OpenShift workloads on Z and the GPU clusters in a unified view, with all cost data exported via the API to the bank\u2019s internal ERP system."))

parts.append(heading2("Defense and Government"))
parts.append(body_text("Defense organizations represent perhaps the most demanding use case for sovereign cost management. A European air force deployed Red Hat OpenShift on portable edge datacenters to maintain operations during network outages, running AI workloads locally in disconnected environments."))
parts.append(body_text("Government agencies across Europe are building sovereign cloud infrastructure aligned with national strategies: France\u2019s SecNumCloud, Germany\u2019s BSI C5, Italy\u2019s Polo Strategico Nazionale, Spain\u2019s National Cloud Strategy, and Nordic \u201CPlan B\u201D hybrid architectures. Many of these environments include IBM Z or POWER systems for legacy workloads, making cross-architecture metering a key requirement."))

parts.append(heading2("Internal IT-as-a-Service"))
parts.append(body_text("The same architecture applies to large enterprises operating an internal shared-services model. Red Hat Lightspeed Cost Management\u2019s overhead distribution capability enables the central IT organization to transparently allocate not just direct workload costs but also the shared platform overhead \u2014 proportionally to each business unit based on their actual usage. Fine-grained RBAC enables each business unit to view its own cost data directly \u2014 without seeing other divisions\u2019 consumption \u2014 making self-service chargeback practical even across organizational boundaries. In organizations that span x86-64, ARM, and IBM Z or POWER infrastructure, the tool provides a unified view of costs across all platforms. The REST API enables automated export of cost reports to the enterprise\u2019s financial systems, eliminating manual reconciliation."))

# === SECTION 7: Architecture ===
parts.append(heading1("7. Architecture at a Glance"))
parts.append(body_text("The following diagram illustrates the high-level architecture of a sovereign cloud with integrated metering and cost management."))
parts.append(image_para("rId20", cx, cy))
parts.append(figure_caption("Figure 1: Sovereign cloud architecture. All components \u2014 from provisioning through metering, costing, and reporting \u2014 run within the sovereign perimeter."))
parts.append(body_text("Key characteristics of this architecture:"))
parts.append(bullet_bold("All components run on-premise ", "within the provider\u2019s infrastructure."))
parts.append(bullet_bold("Metering data flows locally ", "from OpenShift clusters to the cost management system via the metrics operator."))
parts.append(bullet_bold("Cost models and reports are stored locally ", "in the provider\u2019s database."))
parts.append(bullet_bold("DCM orchestrates the lifecycle, ", "ensuring every provisioned resource is automatically enrolled for metering and costing."))
parts.append(bullet_bold("Cost reports can feed local billing/ERP systems, ", "completing the path from metering to monetization."))

# === SECTION 8 ===
parts.append(heading1("8. Getting Started"))
parts.append(body_text("The path from sovereign infrastructure to sovereign monetization is incremental. Organizations do not need to implement all three tiers simultaneously."))
parts.append(bold_intro("Start with metering. ", "Deploy the metrics operator on existing OpenShift clusters. Within minutes, Tier 1 basic metering provides visibility into CPU, memory, and storage consumption across the fleet."))
parts.append(bold_intro("Add distribution. ", "Attach cost models to clusters to activate Tier 2 overhead distribution. This reveals the true cost structure of the platform."))
parts.append(bold_intro("Enable full costing. ", "Add price lists to cost models to activate Tier 3. Define rate cards that match commercial agreements, apply markup, and generate cost reports suitable for billing."))
parts.append(bold_intro("Automate with DCM. ", "Integrate the cost management system with DCM to automate enrollment, cost model assignment, and lifecycle management."))
parts.append(body_text("Each tier delivers standalone value. The progression is additive, not disruptive."))

# === SECTION 9 ===
parts.append(heading1("9. Conclusion"))
parts.append(body_text("The sovereign cloud market is growing rapidly, driven by regulation, geopolitics, and the strategic imperative for digital autonomy. Providers are investing heavily in sovereign compute, storage, networking, and security. But without a sovereign financial layer \u2014 metering, costing, chargeback, and billing that operates entirely within the sovereign perimeter \u2014 the business model remains incomplete."))
parts.append(body_text("Red Hat Lightspeed Cost Management and the DCM project close this gap. Together, they provide a complete, on-premise architecture that takes a sovereign cloud from provisioning through metering, costing, overhead distribution, and reporting \u2014 all without a single byte of financial or operational data leaving the sovereign perimeter."))
parts.append(body_text("For the CIO of a sovereign cloud provider or a datacenter operator, this is not a technology decision. It is a business-model decision. The question is not whether you need metering and cost management, but how quickly you can implement it \u2014 and whether you can afford the sovereignty trade-offs of the alternatives."))

# === REFERENCES ===
parts.append(page_break())
parts.append(heading1("References"))
refs = [
    "1. Forrester, The Sovereign Cloud Platform Landscape, Q4 2025, October 2025.",
    "2. UN Trade and Development, Privacy and Data Protection Legislation Worldwide, February 2025.",
    "3. IDC InfoBrief, Digital Sovereignty in Action: Building Resilient, Compliant, and Transparent Cloud Ecosystems, Document #EUR153367225, August 2025.",
    "4. IDC Market Perspective, Beyond Cost and Compliance: How Red Hat Is Shaping AI, Sovereignty, and Modernization, Document #EUR153890425, November 2025.",
    "5. Gartner, Predicts 70% of Enterprises Adopting GenAI Will Cite Sustainability and Digital Sovereignty as Top Criteria, February 2024.",
    "6. Linux Foundation, The State of Sovereign AI, August 2025.",
]
for ref in refs:
    parts.append(body_para(r(ref, sz="18", color="666666")))

body_xml = "\n".join(parts)

# Section properties (preserve template's header/footer references)
sect_pr = '''<w:sectPr>
  <w:headerReference r:id="rId11" w:type="default"/>
  <w:headerReference r:id="rId12" w:type="first"/>
  <w:footerReference r:id="rId13" w:type="default"/>
  <w:footerReference r:id="rId14" w:type="first"/>
  <w:pgSz w:h="15840" w:w="12240" w:orient="portrait"/>
  <w:pgMar w:bottom="1440" w:top="1800" w:left="1440" w:right="1440" w:header="720" w:footer="720"/>
  <w:pgNumType w:start="0"/>
</w:sectPr>'''

doc_xml = f'''<?xml version="1.0" encoding="UTF-8"?>
<w:document xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" xmlns:o="urn:schemas-microsoft-com:office:office" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math" xmlns:v="urn:schemas-microsoft-com:vml" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:w10="urn:schemas-microsoft-com:office:word" xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:wne="http://schemas.microsoft.com/office/word/2006/wordml" xmlns:sl="http://schemas.openxmlformats.org/schemaLibrary/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture" xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:lc="http://schemas.openxmlformats.org/drawingml/2006/lockedCanvas" xmlns:dgm="http://schemas.openxmlformats.org/drawingml/2006/diagram" xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape" xmlns:wpg="http://schemas.microsoft.com/office/word/2010/wordprocessingGroup" xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml" xmlns:w15="http://schemas.microsoft.com/office/word/2012/wordml" xmlns:w16="http://schemas.microsoft.com/office/word/2018/wordml" xmlns:w16cex="http://schemas.microsoft.com/office/word/2018/wordml/cex" xmlns:w16cid="http://schemas.microsoft.com/office/word/2016/wordml/cid" xmlns:dt="http://schemas.microsoft.com/office/tasks/2019/documenttasks" xmlns:cr="http://schemas.microsoft.com/office/comments/2020/reactions">
<w:body>
{body_xml}
{sect_pr}
</w:body>
</w:document>'''

# === BUILD ===
if WORK.exists():
    shutil.rmtree(WORK)
shutil.copytree("/tmp/rh-template", str(WORK))

# Write new document.xml
(WORK / "word" / "document.xml").write_text(doc_xml, encoding="utf-8")

# Copy architecture image
shutil.copy2(ARCH_IMG, WORK / "word" / "media" / "architecture-diagram.png")

# Add image relationship to document.xml.rels
rels_path = WORK / "word" / "_rels" / "document.xml.rels"
rels_text = rels_path.read_text()
new_rel = '<Relationship Id="rId20" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/architecture-diagram.png"/>'
rels_text = rels_text.replace("</Relationships>", f"  {new_rel}\n</Relationships>")
rels_path.write_text(rels_text)

# Add PNG content type if not present
ct_path = WORK / "[Content_Types].xml"
ct_text = ct_path.read_text()
if 'Extension="png"' not in ct_text:
    ct_text = ct_text.replace("</Types>", '  <Default Extension="png" ContentType="image/png"/>\n</Types>')
    ct_path.write_text(ct_text)

# Add numbered list to numbering.xml
num_path = WORK / "word" / "numbering.xml"
num_text = num_path.read_text()
numbered_abstract = '''<w:abstractNum w:abstractNumId="4">
    <w:lvl w:ilvl="0">
      <w:start w:val="1"/>
      <w:numFmt w:val="decimal"/>
      <w:lvlText w:val="%1."/>
      <w:lvlJc w:val="left"/>
      <w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr>
    </w:lvl>
  </w:abstractNum>'''
numbered_num = '<w:num w:numId="4"><w:abstractNumId w:val="4"/></w:num>'
num_text = num_text.replace("</w:numbering>", f"  {numbered_abstract}\n  {numbered_num}\n</w:numbering>")
num_path.write_text(num_text)

# Update footer1.xml: replace placeholder with page number
footer1_path = WORK / "word" / "footer1.xml"
footer1 = footer1_path.read_text()
# Replace the "Footer Contents" text with "redhat.com"
footer1 = footer1.replace(
    '<w:highlight w:val="yellow"/>\n        <w:rtl w:val="0"/>\n      </w:rPr>\n      <w:t xml:space="preserve">Footer Contents</w:t>',
    '<w:rtl w:val="0"/>\n      </w:rPr>\n      <w:t xml:space="preserve">redhat.com</w:t>'
)
# Remove remaining yellow highlights
footer1 = footer1.replace('<w:highlight w:val="yellow"/>\n        ', '')
footer1 = footer1.replace('<w:highlight w:val="yellow"/>', '')
footer1_path.write_text(footer1)

# Add updateFields to settings.xml so TOC gets populated on open/convert
settings_path = WORK / "word" / "settings.xml"
settings = settings_path.read_text()
settings = settings.replace(
    '<w:defaultTabStop',
    '<w:updateFields w:val="true"/>\n  <w:defaultTabStop'
)
settings_path.write_text(settings)

# Update copyright year in footer2
footer2_path = WORK / "word" / "footer2.xml"
footer2 = footer2_path.read_text()
footer2 = footer2.replace("Copyright © 2022", "Copyright © 2026")
footer2 = footer2.replace("Red Hat Enterprise Linux, the Red Hat logo, and JBoss are",
                          "the Red Hat logo, OpenShift, and Ansible are")
footer2_path.write_text(footer2)

# Repack into DOCX
if OUTPUT.exists():
    OUTPUT.unlink()

with zipfile.ZipFile(OUTPUT, "w", zipfile.ZIP_DEFLATED) as zf:
    for root, dirs, files in os.walk(WORK):
        for f in files:
            full = Path(root) / f
            arcname = str(full.relative_to(WORK))
            zf.write(full, arcname)

print(f"Generated: {OUTPUT}")
print(f"  Image: {img_w}x{img_h}px -> {cx}x{cy} EMU ({cx/914400:.1f}x{cy/914400:.1f} inches)")

# === GENERATE PDF via LibreOffice ===
PDF_OUTPUT = OUTPUT.with_suffix(".pdf")
print(f"Converting to PDF...")
result = subprocess.run(
    ["libreoffice", "--headless", "--convert-to", "pdf", "--outdir", str(OUTPUT.parent), str(OUTPUT)],
    capture_output=True, text=True, timeout=120,
)
if result.returncode == 0 and PDF_OUTPUT.exists():
    print(f"Generated: {PDF_OUTPUT} ({PDF_OUTPUT.stat().st_size / 1024:.0f} KB)")
else:
    print(f"PDF conversion failed (exit {result.returncode})")
    if result.stderr:
        print(f"  stderr: {result.stderr.strip()}")
