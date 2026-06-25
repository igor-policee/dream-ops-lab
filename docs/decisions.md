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
sensitive vars, API tokens) are stored in OpenBao. Nothing is stored in Git.
On-disk exceptions: bootstrap tfstate (local state backend, removed after Phase 1.4)
and the OpenBao backup token (least-privilege, scoped to Raft snapshot only).

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
WiFi uplink (wlp5s0).

**Reason:** WiFi (802.11) does not support L2 bridging due to the three-address frame
limitation. NAT bridge is the only viable option without additional hardware.

**Alternatives considered:**
- Direct L2 bridge to wlp5s0 — not possible on WiFi
- Incus OVN — adds complexity with no benefit given single-host setup

**Trade-offs:** VMs are behind NAT. External access to services requires explicit
port forwarding or routing through the reverse tunnel.

---

## 2026-06-23 — Reverse SSH tunnel via dev-ubuntu-01 for remote access

**Decision:** The physical host maintains a persistent outbound reverse SSH tunnel
to dev-ubuntu-01 (autossh + systemd). Remote access is via SSH into the host through dev-ubuntu-01.
All platform interaction (kubectl, talosctl, incus) happens directly on the host.

**Reason:** The host is behind WiFi NAT with no public IP. Reverse SSH is the
simplest reliable solution. Works on standard SSH port, avoids DPI/TSPU filtering
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
Each server forwards to the other only for names it knows the other can answer.
Both servers are terminal for unknown `dream.lab` names — they return NXDOMAIN
rather than forwarding back. This prevents infinite forwarding loops.

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

## 2026-06-24 — Service DNS names carry no numeric suffix

**Decision:** DNS names used in application config, certificates, and URLs do not
include the numeric VM suffix. VM hostnames (with suffix) are resolvable but used
only for direct VM access.

| VM hostname | Service DNS name |
|-------------|-----------------|
| gitlab-01 | gitlab.dream.lab |
| step-ca-01 | step-ca.dream.lab |
| openbao-01 | openbao.dream.lab |

Implemented via static aliases in Incus dnsmasq (CNAME or address records).

**Reason:** A numbered DNS name leaks infrastructure topology into application
config, certificates, and OpenTofu backends. If a VM is replaced or renamed, all
references must be updated. A stable service name decouples the service identity
from the VM instance.

**Alternatives considered:** Use the numbered hostname everywhere — simpler, but
creates coupling between service identity and VM naming.

**Trade-offs:** Requires explicit alias entries in dnsmasq config.

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

## 2026-06-24 — Backup strategy: age + dev-ubuntu-01 + Bitwarden

**Decision:** Back up critical VM data (OpenBao snapshots, step-ca CA material,
GitLab data, OpenTofu state) to dev-ubuntu-01, encrypted with `age` (asymmetric). Store the
age private key and OpenBao unseal shards in Bitwarden. Trigger backups via
systemd (on host startup + hourly timer), not cron.

**Reason:** The physical host is frequently off — a cron schedule is unreliable.
Systemd event-based triggers (OnBootSec + OnUnitActiveSec) fire correctly regardless
of how long the host has been off. dev-ubuntu-01 is online 24/7 and already present
in the architecture as the reverse SSH tunnel endpoint — adding a backup role reuses
existing infrastructure without new components. Bitwarden Premium is available and
provides secure, accessible off-site storage for small secrets that do not change.

Asymmetric `age` encryption allows the backup script to encrypt without any secret
on the host — only the public key is embedded in the script. The private key lives
in Bitwarden and is only needed for recovery.

**Alternatives considered:**
- Symmetric encryption (GPG/age -p): requires the passphrase on the host for
  automation, creating a secret that must be protected there.
- Bitwarden file attachments for backups: Premium supports it, but Bitwarden is
  not designed as a file backup store; automated uploads via `bw` CLI are cumbersome.
- Separate cloud storage (S3/B2): viable but adds an external dependency and cost;
  dev-ubuntu-01 already exists and is sufficient.

**Trade-offs:** If dev-ubuntu-01 is lost, backups are lost. Acceptable because
dev-ubuntu-01 is a managed VPS — more durable than the homelab host itself.
OpenBao snapshot is skipped if OpenBao is sealed (requires manual unseal after
host boot); at worst, the previous backup is used for recovery.

---

## 2026-06-24 — OpenTofu state: local backend during bootstrap, then GitLab HTTP

**Decision:** Use the `local` backend for OpenTofu state during Phase 1 (while
gitlab-01 does not yet exist). Back up the state file manually to dev-ubuntu-01
after each `tofu apply` until migration (see "Manual tfstate Backup" in runbooks.md).
After gitlab-01 is operational, migrate state to the GitLab HTTP backend via
`tofu init -migrate-state`.

**Reason:** The GitLab HTTP backend is not available until gitlab-01 is provisioned
(Phase 1.4). Using gitlab.com as a temporary backend introduces an external SaaS
dependency and an extra migration step. Local state with automated encrypted backup
to dev-ubuntu-01 is simpler: no external accounts, no temporary service to set up,
and the same backup infrastructure already covers OpenBao and step-ca.

**Alternatives considered:**
- gitlab.com as temporary backend: zero setup, but introduces external SaaS dependency.
- Lightweight HTTP state server on dev-ubuntu-01 (e.g., terrastate): avoids local
  state but adds a new service to operate on dev-ubuntu-01.

**Trade-offs:** State file on local disk; if host fails between backups, up to one
hour of state changes may be lost. Recoverable via `tofu import` for the small number
of VMs involved. Risk is acceptable for the bootstrap phase.

---

## 2026-06-23 — Ubuntu 24.04 LTS as host OS

**Decision:** Retain existing Ubuntu 24.04 LTS installation. No reinstall needed.

**Reason:** Ubuntu 24.04 is well-supported for Incus (via Zabbly repository),
has good kernel support for eBPF and KVM, and is already installed.

**Alternatives considered:** Debian 12, NixOS.

**Trade-offs:** Ubuntu adds some default bloat vs Debian, but the difference is
negligible for a single-host lab. NixOS would be more declarative but adds
significant operational complexity for Incus.

---

## 2026-06-25 — Custom Go CLI tools for security posture and secret rotation

**Decision:** Build two custom Go CLI tools (`dream-checker`, `bao-rotator`) rather
than relying entirely on ad-hoc shell scripts or separate third-party CLIs for
security validation and secret rotation.

**Reason:**

`dream-checker` covers four check domains in a single binary with a unified
`CheckResult` struct and consistent JSON/table output:

- **k8s** (K8S-001..007): privileged pods, hostNetwork, root containers, resource
  limits, default SA automount, NetworkPolicies, pod-security labels
- **vault** (VAULT-001..005): sealed state, root token, audit devices, required
  engines, token TTL
- **pki** (PKI-001..004): CA reachability, cert-manager Certificate expiry and
  Ready status, CRL endpoint
- **supply** (SUPPLY-001..005): gitleaks/trivy availability, cosign annotations,
  SBOM annotations, mutable image tags

No existing single tool covers this combination. Kubescape covers K8s posture but
not Vault, PKI, or supply-chain state. The vault CLI and cert-manager CLI require
separate invocations and produce inconsistent output formats. A unified tool with
a common exit-code contract (`0` = no FAIL, `1` = at least one FAIL) integrates
cleanly into GitLab CI as a blocking gate and into Loki via CronJob stdout.

`bao-rotator` provides recursive KV v2 listing (directory markers `"/"` traversed
automatically), a 90-day rotation threshold audit, and structured `slog` output
compatible with Loki ingestion. The `vault kv` CLI does not support recursive listing
or policy-based age auditing without custom scripting.

**Go** was chosen for both tools because:
- `k8s.io/client-go` and `github.com/hashicorp/vault/api` provide typed K8s and
  OpenBao access with in-cluster auth support
- Single static binary output simplifies container images (multi-stage Dockerfile,
  final stage `alpine:3.19` with CA certs)
- Strong type system and table-driven tests support long-term maintenance

**Alternatives considered:**
- Shell scripts wrapping `kubectl`, `vault`, `step`, `cosign` — fragmented output
  formats, no unified exit code contract, harder to test
- Python with kubernetes/hvac libraries — viable but heavier container image,
  slower startup, no single-binary distribution
- Kubescape + vault-benchmark + custom scripts — three tools with different
  invocation patterns and no shared output schema

**Trade-offs:**
- Custom code requires maintenance; checked mitigated by unit tests and CI
- `go.sum` must be committed (generated via `go mod tidy` — first step of Phase 3.9)
  before CI builds are reproducible
- PKI checks depend on cert-manager CRDs being installed; gracefully SKIPs otherwise
- CronJob image tags are pinned (`v0.1.0`); CI publishes both `:v0.1.0` and `:latest`
  so the manifest reference is always satisfiable

---

## 2026-06-25 — Talos Linux as Kubernetes node OS

**Decision:** Use Talos Linux as the OS for all Kubernetes VMs (`talos-cp-01`,
`talos-worker-01`, `talos-worker-gpu-01`).

**Reason:** Talos Linux is a purpose-built, immutable Kubernetes OS with no SSH
daemon, no shell, and no package manager. All node configuration is declarative
(machine config YAML), version-controlled, and applied via the Talos HTTPS API.
This eliminates configuration drift, unauthorized access paths, and manual
intervention — directly aligned with the platform's immutable, API-driven philosophy.
GPU passthrough and kernel parameters are expressed in the machine config, keeping
all node state in a single auditable artifact stored in OpenBao.

**Alternatives considered:**
- Ubuntu + kubeadm — mutable OS; configuration drift risk; SSH access; does not
  fit immutable philosophy; Ansible required for node configuration
- k3s — simplified distribution; reduced control surfaces; insufficient for a
  production-grade platform
- RKE2 — closer to upstream K8s; mutable node OS; SSH-accessible
- Flatcar Container Linux — immutable, but smaller K8s provisioning tooling ecosystem

**Trade-offs:**
- No SSH access to nodes; debugging uses `talosctl logs`, container exec, or the
  Talos API — requires learning the Talos toolchain
- Machine config is the only path to reconfigure a node; config must be stored
  securely (OpenBao) and backed up before any destructive operation
- Hardware configuration (GPU passthrough, IOMMU) must be expressed in machine
  config; tested in isolation before attaching to the worker node

---

## 2026-06-25 — Cilium as CNI (kube-proxy replacement mode)

**Decision:** Use Cilium as the Kubernetes CNI plugin, deployed in kube-proxy
replacement mode via Helm before any other cluster component.

**Reason:** Cilium provides the complete network and security stack required by
the platform in a single component:

- **eBPF dataplane** — packet processing without iptables; lower latency, no
  conntrack table limits
- **kube-proxy replacement** — all service load-balancing in eBPF; kube-proxy
  pod is not deployed
- **WireGuard node-to-node encryption** — transparent in-cluster traffic encryption
  without per-application configuration
- **Hubble** — built-in network observability (flow inspector, Grafana integration,
  service map); no separate tool required
- **L2 LoadBalancer** (`CiliumLoadBalancerIPPool` + `CiliumL2AnnouncementPolicy`) —
  assigns real IPs on incusbr0 to `LoadBalancer` services without a cloud provider;
  required for CoreDNS stable IP (10.10.0.53)
- **Gateway API native implementation** — HTTPRoute, GatewayClass, TLSRoute handled
  in eBPF; no separate Ingress controller needed
- **Tetragon integration** — Tetragon runs inside the Cilium ecosystem and shares
  eBPF state, enabling correlated network + process security events

**Alternatives considered:**
- Flannel — simple VXLAN overlay; no eBPF, no security, no observability; not
  suitable for a security-focused platform
- Calico — mature, eBPF optional, strong NetworkPolicy; lacks native Gateway API
  support, Hubble observability, and Tetragon integration
- Weave Net — legacy; no longer actively developed for Kubernetes

**Trade-offs:**
- Requires kernel ≥ 5.10 for full eBPF support; Ubuntu 24.04 with kernel 6.8
  satisfies this
- kube-proxy replacement must be set at Cilium install time; cannot be changed
  in-place
- WireGuard encryption adds a small CPU overhead per node (acceptable for a lab)
- Must be installed via Helm before ArgoCD; is the one component not managed by
  ArgoCD

---

## 2026-06-25 — ArgoCD with App-of-Apps pattern for GitOps delivery

**Decision:** Use ArgoCD as the GitOps continuous delivery system. All Kubernetes
workloads are deployed and reconciled through ArgoCD. A single root ArgoCD Application
(App-of-Apps) in the infrastructure repo drives all other applications.

**Reason:**
- Declarative GitOps model: cluster state is derived from Git; no manual `kubectl
  apply` in steady-state operations; configuration drift is detected and corrected
  automatically
- ArgoCD provides a rich UI for observing sync status, resource health, and rollback
- App-of-Apps pattern: adding a new service means adding one `Application` manifest
  to the repo; no direct cluster access or imperative commands required
- Broad ecosystem: Helm, Kustomize, and plain YAML support; native ApplicationSet;
  large community and plugin ecosystem

**Alternatives considered:**
- Flux — lighter, GitOps Toolkit; no built-in UI; better for operator-driven
  deployment; less discoverable for learning and troubleshooting
- Helm only — no continuous reconciliation; configuration drift is not detected;
  updates require manual `helm upgrade`
- CI push model (kubectl in GitLab CI) — requires cluster credentials in CI;
  no self-healing on manual changes

**Trade-offs:**
- ArgoCD must be bootstrapped via Helm (Phase 3.4) before it can manage anything;
  the bootstrap is the one imperative step in the otherwise declarative model
- ArgoCD is not available if the cluster is down; infrastructure operations still
  go through host-level tools (talosctl, incus, tofu)

---

## 2026-06-25 — External Secrets Operator with AppRole auth for Kubernetes secret sync

**Decision:** Use External Secrets Operator (ESO) with OpenBao AppRole authentication
to sync secrets from OpenBao into Kubernetes Secrets. ESO is deployed in Phase 3.5 —
before the ArgoCD App-of-Apps (Phase 3.6) — with a permanent Kubernetes Secret
holding the AppRole credentials.

**Reason:** ESO provides a Kubernetes-native model for referencing OpenBao secrets
without embedding them in manifests or Git. An `ExternalSecret` resource declares
which OpenBao KV path maps to which K8s Secret — ESO handles sync and rotation
automatically.

AppRole was chosen over Kubernetes auth because:
- AppRole is pull-based with explicit `role_id` + `secret_id`; no dependency on
  Kubernetes API availability at auth time
- Kubernetes auth requires OpenBao to reach the Kubernetes API to validate service
  account tokens — creates an auth circular dependency during cluster bootstrap or
  recovery scenarios
- AppRole credentials are operationally simpler to issue, revoke, and audit

The ESO auth Kubernetes Secret is permanent: `ClusterSecretStore` reads the AppRole
`role_id` and `secret_id` from it via `secretRef` at runtime. Deleting this secret
breaks all `ExternalSecret` syncs and requires re-issuing AppRole credentials manually.

ESO is deployed before App-of-Apps so that secret-dependent applications can begin
syncing immediately when ArgoCD starts managing them in Phase 3.6.

**Alternatives considered:**
- Vault Agent Injector / vault-k8s sidecar — injects secrets as sidecar containers;
  tighter per-pod coupling; harder to audit at the K8s manifest level
- ESO with Kubernetes auth — eliminates the permanent secret but introduces auth-time
  dependency on the K8s API during recovery
- Sealed Secrets — encrypts K8s Secrets in Git; secrets still live in Git, even
  if encrypted; ESO preferred for centralised secret management in OpenBao

**Trade-offs:**
- The ESO AppRole Kubernetes Secret must not be deleted accidentally; document
  as a named operational risk (see handoff-context.md)
- All ESO-synced secrets are plaintext in etcd; Talos encrypts etcd at rest by default

---

## 2026-06-25 — cert-manager inside Kubernetes with step-ca as ACME issuer

**Decision:** Deploy cert-manager inside Kubernetes configured with an ACME
`ClusterIssuer` pointing to step-ca-01. cert-manager handles all certificate
issuance and renewal for Kubernetes workloads.

**Reason:** cert-manager is the de facto standard for Kubernetes certificate
management. It automates issuance and renewal via `Certificate` CRDs; no manual
certificate operations are needed for K8s services. Using step-ca as the ACME
issuer keeps the K8s PKI rooted in the same internal CA as the rest of the
platform — one consistent trust chain across VMs and K8s services.

Tracking certificates as cert-manager `Certificate` CRDs (not raw K8s Secrets)
allows dream-checker to monitor expiry and Ready status via the cert-manager API
without requiring RBAC access to raw Secret data.

**Alternatives considered:**
- Self-signed `ClusterIssuer` inside K8s — no central CA; certificates are not
  trusted outside the cluster without per-client trust store configuration
- Wildcard certificate managed externally and mounted as a K8s Secret — no
  automatic renewal; manual rotation required; not scalable beyond a few services

**Trade-offs:**
- cert-manager depends on step-ca-01 being reachable at issuance and renewal time;
  step-ca-01 is outside K8s specifically to ensure this independence
- The step-ca root certificate must be added to the Kubernetes trust bundle before
  cert-manager can reach the ACME endpoint — a one-time bootstrap step

---

## 2026-06-25 — DevSecOps tool selection

**Decision:** Use the following tools for security posture, supply chain security,
and compliance:

| Domain | Tool | Integration |
|--------|------|-------------|
| Secret scanning | Gitleaks | pre-commit hook + GitLab CI |
| IaC scanning | Checkov | GitLab CI, SARIF output |
| K8s posture | Kubescape | In-cluster operator, Prometheus + Grafana (Phase 5.8) |
| Image scanning | Trivy | GitLab CI (image + IaC + secret scan), SARIF output |
| Image signing | Cosign | GitLab CI, key in OpenBao, enforced via Kyverno verifyImages |
| SBOM generation | Syft | GitLab CI (CycloneDX format) |
| SCA / vuln management | Dependency-Track | In-cluster (Phase 4.6), NVD + OSV feeds, CI upload gate |
| Runtime security | Tetragon | In-cluster (Phase 4.7), custom TracingPolicies, Loki integration |
| Policy enforcement | Kyverno | In-cluster (Phase 4.2), CIS benchmark policies, verifyImages |

**Reason per component:**

- **Gitleaks** — purpose-built for secret detection; fast; configurable allowlist
  via `.gitleaks.toml`; native pre-commit framework integration
- **Checkov** — multi-framework IaC scanner (Terraform, Ansible, Kubernetes,
  Dockerfile); single tool covering all IaC types in the repo
- **Kubescape** — NSA/CISA and CIS K8s benchmarks; in-cluster operator with
  historical tracking; native Prometheus metrics for Grafana dashboard integration
- **Trivy** — CNCF project; covers images, filesystems, IaC, and secrets in one
  CLI; SARIF output compatible with GitLab CI security reports
- **Cosign** — CNCF project; OCI-native image signing; integrates with Kyverno
  `verifyImages` for admission-time signing enforcement
- **Syft + Dependency-Track** — Syft generates CycloneDX SBOMs in CI; Dependency-Track
  ingests them, tracks CVEs against NVD/OSV, and provides a persistent vulnerability
  dashboard independent of GitLab
- **Tetragon** — eBPF runtime security from the Cilium team; correlated network +
  process visibility; shares eBPF state with Cilium

**Alternatives considered:**
- TruffleHog over Gitleaks — heavier and more complex config; Gitleaks preferred
- tfsec/KICS over Checkov — single-framework; Checkov covers all IaC types
- Falco over Tetragon — kernel-module based; Tetragon preferred for eBPF coherence
- OPA/Gatekeeper over Kyverno — requires Rego; Kyverno policies are plain YAML
- GitLab Security Dashboard — requires Ultimate license (see next decision)

**Trade-offs:**
- Multiple tools require coordination; `dream-checker` supply module (SUPPLY-001..005)
  provides a unified availability check across all tools
- SLSA Level 2 is the target (signed images + build provenance); Level 3 requires
  hermetic builds not feasible in this environment

---

## 2026-06-25 — GitLab CE security reports via SARIF artifacts, not Security Dashboard

**Decision:** Use SARIF-format pipeline artifacts for security scan results (Trivy,
Checkov, Gitleaks). Use Dependency-Track as the standalone vulnerability management
platform. Do not depend on the GitLab Security Dashboard.

**Reason:** GitLab Security Dashboard — persistent vulnerability list, dependency
inventory, MR security widget — is a GitLab Ultimate feature. GitLab CE supports
`artifacts:reports:sast`, `artifacts:reports:dependency_scanning`, and
`artifacts:reports:secret_detection` for per-pipeline display only; no project-level
vulnerability tracking, no MR security gate.

Dependency-Track fills the gap: persistent CVE tracking, NVD/OSV feed integration,
component inventory across all projects, policy-based CI gates — all self-hosted.

**Alternatives considered:**
- GitLab.com Ultimate free trial — temporary; introduces external SaaS dependency
- Self-hosted GitLab EE — license required; not appropriate for a lab
- DefectDojo — open-source security aggregator; more complex to operate; better fit
  for multi-team enterprise environments than a single-operator lab

**Trade-offs:**
- No GitLab MR security widget; developers check Dependency-Track or pipeline
  SARIF artifacts directly
- Dependency-Track is an additional in-cluster service to operate (Phase 4.6)

---

## 2026-06-25 — Gateway API over Kubernetes Ingress

**Decision:** Use the Kubernetes Gateway API (HTTPRoute, GatewayClass, TLSRoute)
for HTTP/HTTPS traffic routing to platform services. Cilium is the implementation.
No Ingress controller is deployed.

**Reason:**
- Gateway API is the official successor to the Kubernetes Ingress API; Ingress is
  frozen and not being extended further
- Cilium implements Gateway API natively via eBPF — no additional pod-based
  controller (nginx, traefik) is required
- `k8s_gateway` (CoreDNS plugin) watches Gateway API resources and auto-generates
  DNS records, enabling zero-manual-DNS service exposure when a new HTTPRoute is
  created
- Gateway API provides role separation: cluster-admin manages `Gateway` (IP/TLS);
  developers manage `HTTPRoute` (routing rules); cleaner for a multi-service platform

**Alternatives considered:**
- nginx Ingress controller — mature; requires a separate deployment; Ingress API
  is limited (no traffic splitting or header matching without annotations)
- Traefik — flexible middleware; adds another component; does not integrate with
  k8s_gateway as cleanly
- Kubernetes Ingress API directly — frozen; no role separation; annotation-heavy
  for advanced routing

**Trade-offs:**
- Gateway API CRDs must be installed before enabling Cilium Gateway API (Phase 3.7)
- Less community tooling for edge cases than Ingress, but sufficient for all
  platform use cases
