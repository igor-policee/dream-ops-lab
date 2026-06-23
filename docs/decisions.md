# Decisions

## 2026-06-24 — OpenTofu over Terraform for infrastructure provisioning

**Decision:** Use OpenTofu instead of HashiCorp Terraform.

**Reason:** HashiCorp changed Terraform's license from MPL to BSL (Business Source
License) in August 2023 — the same move made with Vault. BSL is not open source by
the OSI definition and restricts commercial use in competing products. OpenTofu is the
Linux Foundation fork of Terraform under the original MPL license, maintained by the
community. It is API and configuration-compatible with Terraform; all providers,
modules, and tooling work without changes.

**Alternatives considered:** HashiCorp Terraform.

**Trade-offs:** OpenTofu is slightly behind Terraform on bleeding-edge features but
covers all required functionality. Community and ecosystem are growing rapidly.

---

## 2026-06-24 — OpenBao as standalone Incus VM outside Kubernetes

**Decision:** Run OpenBao as a dedicated Incus VM (`openbao-01`), independent of
Kubernetes, provisioned before the cluster.

**Reason:** OpenBao stores operational secrets required to bootstrap and recover
Kubernetes (Talos machine configs, kubeconfig, API tokens). Running it inside K8s
creates a circular dependency — the cluster cannot be bootstrapped or recovered if
the secrets store is unavailable. The same reasoning applies as for step-ca-01.
openbao-01 serves both operational secrets and application secrets for K8s workloads.

**Alternatives considered:** OpenBao inside Kubernetes as a Helm chart.

**Trade-offs:** One additional VM to operate. openbao-01 and step-ca-01 are both
critical pre-K8s dependencies — their availability and backup must be maintained.

---

## 2026-06-24 — Container registry: GitLab Container Registry

**Decision:** Use the built-in GitLab Container Registry on gitlab-01. No separate
registry service.

**Reason:** GitLab CE includes a container registry that integrates natively with
GitLab CI/CD pipelines. Images built in pipelines are pushed directly without
additional configuration. Trivy handles image scanning independently in the pipeline.
Adding Harbor would duplicate functionality without meaningful benefit for a
single-team, single-GitLab setup.

**Alternatives considered:** Harbor — CNCF graduated registry with built-in Trivy
scanning, image signing (Cosign), proxy cache, and multi-tenancy. Harbor is the
right choice when multiple CI/CD systems share a registry, when proxy caching of
public registries is needed, or when air-gapped operation is required.

**Trade-offs:** GitLab registry lacks proxy caching and image signing. Acceptable
for a lab; Harbor can replace it if requirements grow.

---

## 2026-06-24 — OpenTofu state backend: GitLab HTTP backend

**Decision:** Store OpenTofu state in GitLab's built-in HTTP backend.

**Reason:** GitLab CE provides a Terraform/OpenTofu HTTP state backend out of the
box. It supports state locking and versioning, requires no additional infrastructure,
and keeps state alongside the code under the same access control model as GitLab.

**Alternatives considered:**
- MinIO S3 backend — requires separate locking mechanism; adds complexity.
- Local file — not shareable, no locking, lost on machine failure.

**Trade-offs:** State depends on GitLab availability. Acceptable since gitlab-01 is
a standalone VM independent of Kubernetes.

---

## 2026-06-24 — Operational secrets in OpenBao

**Decision:** All operational secrets (Talos machine configs, kubeconfig, Ansible
sensitive vars, API tokens) are stored in OpenBao. Nothing is stored in Git or on disk.

**Reason:** Centralises secret management. OpenBao provides audit logs, TTL-based
leases, AppRole auth, and fine-grained access policies. Consistent with the platform's
use of OpenBao for application secrets.

**Trade-offs:** OpenBao must be available to bootstrap or recover infrastructure.
step-ca-01 is provisioned first, OpenBao is provisioned early in the K8s cluster
bootstrap sequence.

---

## 2026-06-24 — VM resource allocation

**Decision:** Allocate resources as follows:

| VM | vCPU | RAM | Disk |
|----|------|-----|------|
| step-ca-01 | 1 | 1 GB | 10 GB |
| openbao-01 | 1 | 2 GB | 20 GB |
| gitlab-01 | 4 | 6 GB | 200 GB |
| talos-cp-01 | 2 | 4 GB | 100 GB |
| talos-worker-01 | 6 | 20 GB | 200 GB |
| talos-worker-gpu-01 | 6 | 20 GB | 200 GB |

talos-worker-gpu-01 runs the full platform services workload alongside GPU jobs.
The `-gpu` suffix in the name signals that the node has GPU resources available
via PCI passthrough and the NVIDIA GPU Operator.

**Reason:** A single dedicated GPU-only VM with minimal CPU/RAM (the original
2 vCPU / 6 GB design) creates a bottleneck — GPU workloads (ML training, inference)
require CPU and RAM for data loading and preprocessing. Merging the GPU node with a
full-size worker gives it sufficient general compute while keeping the GPU accessible.
Two workers instead of three reduces VM count without meaningful loss of capacity on
a single physical host, where an extra worker node provides no real resilience benefit.

**Trade-offs:** 20 vCPUs on 16 physical threads — acceptable overcommit for a lab.
Total RAM 53 GB leaves an 11 GB OS reserve on the 64 GB host.

---

## 2026-06-24 — Single Kubernetes control plane node

**Decision:** Deploy one control plane node (talos-cp-01) instead of three.

**Reason:** All VMs run on a single physical host. Three control plane nodes would
provide no real high availability — if the host goes down, all nodes go down
simultaneously. A single CP node saves ~8 GB RAM and 4 vCPU for worker workloads.

**Alternatives considered:** 3-node HA control plane.

**Trade-offs:** No control plane redundancy. Acceptable for a single-host lab.

---

## 2026-06-24 — Platform services stack

**Decision:** The following services run inside Kubernetes, managed by ArgoCD.

| Category | Solution |
|----------|----------|
| Secrets | OpenBao (open-source Vault fork, MPL license) |
| Policy | Kyverno |
| Runtime security | Tetragon (eBPF, Cilium ecosystem) |
| Image scanning | Trivy |
| Metrics | kube-prometheus-stack (Prometheus + Alertmanager + Grafana) |
| Logs | Loki |
| Traces | Tempo |
| Telemetry collection | OpenTelemetry Collector |
| Network observability | Cilium Hubble |
| Object storage | MinIO (S3-compatible) |
| Streaming | Strimzi (Kafka operator) |
| Batch processing | Spark Operator |
| PostgreSQL | CloudNativePG (CNPG) |
| ClickHouse | Altinity clickhouse-operator |
| GPU | NVIDIA GPU Operator |

**Reason:** Each tool is selected as the most modern, K8s-native solution in its
category. The stack forms a complete base platform enabling DevSecOps, AI/ML (GPU),
and big data workloads.

**Key rationale per component:**

- **OpenBao** — HashiCorp Vault changed to BSL (non-OSS) in 2023. OpenBao is the
  Linux Foundation fork under MPL, API-compatible with Vault.
- **Kyverno** — K8s-native policies as YAML resources, no separate language required.
- **Tetragon** — eBPF-based runtime security from the Cilium team; integrates
  natively with Cilium's network layer already in use.
- **kube-prometheus-stack** — industry standard, PromQL is a universally applicable
  skill, richer ecosystem than alternatives.
- **MinIO** — de facto standard S3-compatible object storage for self-hosted K8s;
  used by Loki, Tempo, Spark, ML frameworks, and more.
- **Strimzi** — mature Kafka operator, the standard for Kafka on K8s.
- **CloudNativePG** — CNCF project, the most modern K8s-native PostgreSQL operator.
- **Altinity clickhouse-operator** — de facto standard for ClickHouse on K8s.

**Alternatives considered:**
- VictoriaMetrics over Prometheus — lighter but less standard; Prometheus chosen
  for wider ecosystem and learning value.
- HashiCorp Vault — BSL license excludes it from open-source use cases.
- OPA/Gatekeeper — more powerful but steeper learning curve; Kyverno preferred.
- Falco — mature but kernel-module based; Tetragon preferred for eBPF coherence.

---

## 2026-06-23 — Incus as hypervisor over libvirt/KVM stack

**Decision:** Use Incus to manage VMs on the physical host. Remove libvirt, libvirtd,
and virt-manager. Retain kvm kernel module and qemu-kvm (Incus uses them directly).

**Reason:** Incus provides a modern, unified API for both VMs and containers, aligns
with the immutable/API-driven philosophy of the platform, and has cleaner integration
with Talos provisioning via OpenTofu.

**Alternatives considered:** Keep existing KVM/libvirt stack.

**Trade-offs:** Requires removing the existing KVM management layer. Incus ecosystem
is smaller than libvirt but sufficient for this use case.

---

## 2026-06-23 — NAT bridge (incusbr0) for VM networking

**Decision:** Use an Incus internal bridge (incusbr0, 10.10.0.0/24) with NAT to the
WiFi uplink (wlan0).

**Reason:** WiFi (802.11) does not support L2 bridging due to the three-address frame
limitation. NAT bridge is the only viable option without additional hardware.

**Alternatives considered:**
- Direct L2 bridge to wlan0 — not possible on WiFi
- Incus OVN — adds complexity with no benefit given single-host setup

**Trade-offs:** VMs are behind NAT. External access to services requires explicit
port forwarding or routing through the reverse tunnel.

---

## 2026-06-23 — Reverse SSH tunnel via VPS for remote access

**Decision:** The physical host maintains a persistent outbound reverse SSH tunnel
to a VPS (autossh + systemd). Remote access is via SSH into the host through the VPS.
All platform interaction (kubectl, talosctl, incus) happens directly on the host.

**Reason:** The host is behind WiFi NAT with no public IP. Reverse SSH is the
simplest reliable solution. Works on standard SSH port, avoids DPI/ТСПУ filtering
issues relevant to the operating region (Russia).

**Alternatives considered:**
- WireGuard — more capable but subject to DPI filtering by Russian ISPs
- AmneziaWG (obfuscated WireGuard) — viable future option, deferred to end of project
- SOCKS5 proxy — considered but unnecessary; direct SSH to host provides equivalent access

**Trade-offs:** No direct browser access to internal UIs from a laptop without
an ad-hoc SSH port forward. Acceptable for a training environment.

---

## 2026-06-24 — Internal domain dream.lab

**Decision:** Use `dream.lab` as the internal domain for all platform services and VMs.

**Reason:** No public domain available. `dream.lab` derives from the project name,
is short and readable. `.local` is reserved for mDNS and avoided.

**Alternatives considered:** `lab.internal`, `ops.lab`, `homelab.internal`.

**Trade-offs:** Internal domain requires adding the step-ca root certificate to
browsers and tools once. No public CA trust.

---

## 2026-06-24 — Two-tier DNS: Incus dnsmasq + CoreDNS with k8s_gateway

**Decision:** Use two DNS servers with distinct roles:
- Incus dnsmasq: authoritative for VM hostnames, auto-registered
- CoreDNS + k8s_gateway plugin: authoritative for platform service names in K8s

CoreDNS is exposed via Cilium LoadBalancer at a stable IP on incusbr0.
Each server forwards queries it cannot answer to the other.

**Reason:** Incus dnsmasq handles VM lifecycle automatically. k8s_gateway
auto-registers DNS records when Gateway API resources are created — no manual
DNS management. Single domain `dream.lab` spans both layers transparently.

**Alternatives considered:**
- Static hosts file — does not scale, manual management
- External DNS server (separate VM) — adds unnecessary component
- CoreDNS only — would not auto-register VM names from Incus

**Trade-offs:** Two DNS servers must be aware of each other. CoreDNS depends on
Kubernetes being available; VM name resolution via dnsmasq remains independent.

---

## 2026-06-24 — step-ca as dedicated Incus VM for internal PKI

**Decision:** Run step-ca (Smallstep) as a standalone Incus VM (`step-ca-01`),
independent of Kubernetes. It is the root CA for the entire platform.

**Reason:** PKI must be available before Kubernetes exists and during cluster
rebuilds. Running the CA inside K8s creates a circular dependency — the cluster
cannot be bootstrapped or recovered if the CA is unavailable. A dedicated VM
eliminates this dependency. step-ca supports ACME, allowing cert-manager and
any other ACME client to request certificates without custom integrations.

**Alternatives considered:** cert-manager self-signed ClusterIssuer inside K8s.

**Trade-offs:** One additional VM to operate. step-ca-01 becomes a critical
infrastructure dependency — its availability and backup must be maintained.

---

## 2026-06-24 — Numbered hostnames for all VMs

**Decision:** All VMs use numbered hostnames (e.g., `step-ca-01`, `gitlab-01`,
`talos-cp-01`) regardless of whether multiple replicas are expected.

**Reason:** Consistent naming across the entire inventory. Avoids renaming if
a second instance is ever added. DNS records and certificates are unambiguous.

**Alternatives considered:** Non-numbered names for single-instance services.

**Trade-offs:** Slightly more verbose, negligible in practice.

---

## 2026-06-24 — ZFS over LVM thin pool for Incus storage

**Decision:** Use a ZFS pool as the Incus storage backend. The pool is backed by a
dedicated LVM logical volume created from the free space in ubuntu-vg (~828 GB).

**Reason:** ZFS provides instant snapshots, copy-on-write cloning, and `zfs send/recv`
— all directly useful in a lab where VMs are frequently snapshotted, cloned from
templates, or reset to a clean state. LVM thin pool offers similar features but with
a significantly worse tooling experience.

**Alternatives considered:** LVM thin pool using the existing ubuntu-vg directly.

**Trade-offs:** ZFS-on-LVM adds a second storage layer. Performance overhead is
negligible for a single-host lab. ZFS ARC memory usage is configurable.

---

## 2026-06-24 — Automation model: manual → Ansible → OpenTofu → ArgoCD

**Decision:** Divide automation responsibility across four layers with no overlap.

| Layer | Tool |
|-------|------|
| Host OS install | Manual |
| Host configuration | Ansible |
| VM provisioning | OpenTofu (incus provider, local Unix socket) |
| GitLab configuration | Ansible (inside the GitLab VM) |
| step-ca configuration | Ansible (inside the step-ca-01 VM) |
| Kubernetes workloads | ArgoCD (GitOps, source in GitLab) |

**Reason:** Each tool is used at the layer where it provides the most value. Ansible
is idempotent and appropriate for host-level config. OpenTofu manages declarative
infrastructure resources. ArgoCD provides GitOps continuous reconciliation inside K8s.

**Alternatives considered:** Shell scripts for host config; OpenTofu for everything.

**Trade-offs:** Requires familiarity with three tools, but each layer is independently
operable and testable.

---

## 2026-06-24 — GitLab CE as a standalone Incus VM outside Kubernetes

**Decision:** Run GitLab CE in a dedicated Incus VM, not inside Kubernetes.

**Reason:** GitLab is the GitOps source of truth for ArgoCD. Running it inside the
cluster it manages creates a bootstrap dependency — the cluster cannot self-heal or
be rebuilt if GitLab is unavailable. A standalone VM eliminates this circular dependency.

**Alternatives considered:** GitLab inside Kubernetes.

**Trade-offs:** One additional VM to maintain. GitLab CE is resource-heavy; dedicated
VM isolates its resource usage from the cluster.

---

## 2026-06-24 — GitLab Runner inside Kubernetes (Kubernetes executor)

**Decision:** Run GitLab Runner as a pod inside Kubernetes using the Kubernetes executor.

**Reason:** Application pipelines (image builds, tests, K8s deployments) are the
primary Runner workload. These run naturally as pods inside the cluster. Cluster
infrastructure management (OpenTofu, Ansible, talosctl) is performed directly from
the host, not through GitLab pipelines.

**Alternatives considered:** Dedicated runner VM outside K8s.

**Trade-offs:** Runner is unavailable if the cluster is down. Acceptable because
infrastructure pipelines are not routed through GitLab Runner.

---

## 2026-06-23 — Ubuntu 24.04 LTS as host OS

**Decision:** Retain existing Ubuntu 24.04 LTS installation. No reinstall needed.

**Reason:** Ubuntu 24.04 is well-supported for Incus (via Zabbly repository),
has good kernel support for eBPF and KVM, and is already installed.

**Alternatives considered:** Debian 12, NixOS.

**Trade-offs:** Ubuntu adds some default bloat vs Debian, but the difference is
negligible for a single-host lab. NixOS would be more declarative but adds
significant operational complexity for Incus.

