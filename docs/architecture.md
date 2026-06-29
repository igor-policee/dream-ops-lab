# Architecture

## Overview

Single physical host running Incus as a hypervisor. Talos Linux VMs form a
Kubernetes cluster on top. All layers follow an immutable, API-driven approach.

## Physical Host

| Component | Detail                                                  |
| --------- | ------------------------------------------------------- |
| Hostname  | homelab-ubuntu                                          |
| Hardware  | MSI MAG Z590 Codex X5                                   |
| CPU       | Intel Core i7-11700KF @ 3.60 GHz (8 cores / 16 threads) |
| RAM       | 64 GB                                                   |
| OS        | Ubuntu 24.04.4 LTS                                      |
| Kernel    | 6.8.0-110-generic                                       |

### Storage

| Device  | Size     | Role                                                                                                 |
| ------- | -------- | ---------------------------------------------------------------------------------------------------- |
| sda     | 931.5 GB | Ubuntu (LVM). Only 100 GB allocated to root LV; ~828 GB free in VG — reserved for Incus storage pool |
| nvme0n1 | 953.9 GB | Windows (dual boot, NTFS) — do not modify                                                            |

### Hypervisor stack

Incus replaces libvirt. The kvm kernel module and qemu-kvm are retained — used by Incus directly.

Current state: libvirt removed, Incus installed and initialized (Phase 0 complete, 2026-06-29).

## Incus

### Installation

Installed from the Zabbly repository (maintained by the original LXD/Incus author).

### Storage

ZFS pool backed by a dedicated LVM logical volume carved from the free space in
ubuntu-vg (~828 GB). Incus manages the ZFS pool directly.

```
ubuntu-vg (~828 GB free)
  └── incus-zfs LV (block device)
        └── ZFS pool: incus-pool
              └── Incus VM volumes
```

### OpenTofu integration

OpenTofu manages Incus resources via the `lxc/incus` provider using a local Unix
socket. No remote API or TLS configuration required. The user running `tofu` must
be in the `incus-admin` group.

During bootstrap (before GitLab), `infra/` is deployed to the host via rsync from
the dev machine. Providers are distributed via a filesystem mirror (OpenTofu
registry is blocked from Russia). After Phase 1.4, code is pulled from GitLab.

### VM inventory

| VM                  | OS              | vCPU   | RAM       | Disk       | Role                                                        |
| ------------------- | --------------- | ------ | --------- | ---------- | ----------------------------------------------------------- |
| step-ca-01          | Ubuntu 24.04    | 1      | 1 GB      | 10 GB      | Internal PKI / CA — provisioned first                       |
| openbao-01          | Ubuntu 24.04    | 1      | 2 GB      | 20 GB      | Secrets management — provisioned before K8s                 |
| gitlab-01           | Ubuntu 24.04    | 4      | 6 GB      | 200 GB     | GitLab CE + Container Registry                              |
| talos-cp-01         | Talos Linux     | 2      | 4 GB      | 100 GB     | Kubernetes control plane (single node)                      |
| talos-worker-01     | Talos Linux     | 6      | 20 GB     | 200 GB     | Platform services                                           |
| talos-worker-gpu-01 | Talos Linux     | 6      | 20 GB     | 200 GB     | Platform services + GPU workloads (RTX 3070 Ti passthrough) |
| **Total**           |                 | **20** | **53 GB** | **730 GB** |                                                             |

Host budget: 64 GB RAM (11 GB reserve), ~828 GB disk (98 GB free), 16 threads.
vCPU overcommit is intentional and acceptable for a lab environment.

### GitLab Runner

Runs inside Kubernetes as a pod (Kubernetes executor). Handles application
pipelines: image builds, tests, deployments. Cluster infrastructure (OpenTofu,
Ansible, talosctl) is managed from the host directly, not through GitLab pipelines.

## Networking

### Constraint

The host connects via WiFi (802.11). WiFi interfaces do not support L2 bridging
due to the 802.11 three-address frame limitation.

### Solution

Incus internal bridge with NAT:

```
wlp5s0 (WiFi, 192.168.1.100/24, uplink to router)
  └── incusbr0 (Linux bridge, 10.10.0.0/24)
        ├── NAT → wlp5s0 (outbound internet for VMs)
        └── Incus VMs (static IPs within 10.10.0.0/24)
```

VMs have outbound internet access via NAT. Inbound access is handled by the
remote access layer.

### Remote Access

```
Internet
  └── dev-ubuntu-01 (fixed public IP)
        ← reverse SSH tunnel (outbound from host, persistent via autossh + systemd)
              └── Physical Host (:22)
                    └── incusbr0 (10.10.0.0/24)
                          └── Incus VMs
```

Access pattern: SSH into the host via dev-ubuntu-01 reverse tunnel, then interact with all
components (kubectl, talosctl, incus CLI) directly from the host.

Ad-hoc local port forwarding is used when browser access to internal UIs is needed.

## Supporting Infrastructure

### dev-ubuntu-01

A VPS with a fixed public IP, online 24/7. Serves two roles in the platform:

| Role                    | Details                                                                                                                             |
| ----------------------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| Remote access endpoint  | Accepts the reverse SSH tunnel from the physical host; provides external SSH entry point                                            |
| Off-site backup storage | Stores encrypted backups of critical VM data (OpenBao snapshots, step-ca CA material, GitLab data, OpenTofu state during bootstrap) |

The physical host is turned on and off as needed. dev-ubuntu-01 is always reachable,
making it a reliable anchor for both access and backup.

Backup files on dev-ubuntu-01 are encrypted with `age` (asymmetric). The private
decryption key is stored in Bitwarden, not on the host or on dev-ubuntu-01 itself.
See [runbooks.md](runbooks.md) for backup procedures.

## DNS and PKI

### Internal domain

All platform services use the internal domain `dream.lab`.

### DNS architecture

Two servers with distinct roles. See [network-diagram.md](network-diagram.md) for the
full resolution flow.

**Incus dnsmasq** (10.10.0.1, on incusbr0):

- Authoritative for VM hostnames within `dream.lab` (auto-registered on VM start)
- Hosts static service aliases for standalone VMs (see naming convention below)
- Forwards unresolved `dream.lab` queries to CoreDNS
- Returns NXDOMAIN for unknown `dream.lab` names — does not forward back to CoreDNS
- Forwards all other queries upstream (router / internet)

**CoreDNS** (stable IP via Cilium LoadBalancer, reachable from incusbr0):

- Authoritative for platform service names in `dream.lab` via the `k8s_gateway` plugin
- `k8s_gateway` watches Gateway API resources and auto-generates DNS records
- Handles `*.cluster.local` for K8s internal service discovery
- Forwards VM hostname queries to Incus dnsmasq (for VM names)
- Returns NXDOMAIN for unknown `dream.lab` names — does not forward back to dnsmasq

**Loop prevention:** each server is terminal for names it cannot resolve. Neither
server forwards a `dream.lab` query it received via forwarding back to the other.
This prevents infinite loops for non-existent names.

### DNS naming convention

VM hostnames use a numeric suffix (`gitlab-01`, `step-ca-01`). DNS service names
used in URLs, certificates, and application config do not:

| VM hostname | Service DNS name      | Resolved by                      |
| ----------- | --------------------- | -------------------------------- |
| gitlab-01   | gitlab.dream.lab      | Incus dnsmasq (static alias)     |
| step-ca-01  | step-ca.dream.lab     | Incus dnsmasq (static alias)     |
| openbao-01  | openbao.dream.lab     | Incus dnsmasq (static alias)     |
| talos-cp-01 | talos-cp-01.dream.lab | Incus dnsmasq (VM hostname only) |

Kubernetes nodes are addressed by hostname only — they expose no user-facing service DNS name.

### PKI

step-ca runs as a dedicated Incus VM (`step-ca-01`). It is the root CA for the entire
platform and is provisioned before any other VM.

- Issues certificates via ACME protocol
- cert-manager in Kubernetes uses step-ca as its ACME issuer
- All platform services (GitLab, Talos, K8s ingress) receive certificates from step-ca
- Wildcard certificate `*.dream.lab` used for platform UIs

step-ca is independent of Kubernetes — certificates can be issued before the cluster
exists and during cluster rebuilds.

## Platform Services (Kubernetes)

### Hardware

| Component | Detail                                                      |
| --------- | ----------------------------------------------------------- |
| GPU       | NVIDIA RTX 3070 Ti (PCI passthrough via Incus to worker VM) |

### Service catalogue

| Category                | Solution                                                    |
| ----------------------- | ----------------------------------------------------------- |
| GitOps                  | ArgoCD                                                      |
| Certificates            | cert-manager (ACME → step-ca-01)                            |
| Secrets (K8s workloads) | OpenBao (served from openbao-01 VM)                         |
| Policy                  | Kyverno                                                     |
| Runtime security        | Tetragon                                                    |
| Image scanning          | Trivy                                                       |
| Metrics                 | kube-prometheus-stack (Prometheus + Alertmanager + Grafana) |
| Logs                    | Loki                                                        |
| Traces                  | Tempo                                                       |
| Telemetry collection    | OpenTelemetry Collector                                     |
| Network observability   | Cilium Hubble (included with Cilium)                        |
| Object storage          | MinIO                                                       |
| Streaming               | Strimzi (Kafka operator)                                    |
| Batch processing        | Spark Operator                                              |
| PostgreSQL              | CloudNativePG (CNPG)                                        |
| ClickHouse              | Altinity clickhouse-operator                                |
| GPU                     | NVIDIA GPU Operator                                         |
| GitLab Runner           | Kubernetes executor (pod-based)                             |
| Container registry      | GitLab Container Registry (built into gitlab-01)            |

## Operational Data

### OpenTofu state

During bootstrap (Phase 1, before GitLab is available), state is stored locally on the
host using the `local` backend. The state file is backed up manually to dev-ubuntu-01
after each `tofu apply` until migration to GitLab in Phase 1.4.

After gitlab-01 is operational (end of Phase 1.4), state is migrated to GitLab's
built-in HTTP backend via `tofu init -migrate-state`. This backend supports locking
and versioning; state lives alongside the code in GitLab under the same access control.

```hcl
terraform {
  backend "http" {
    address        = "https://gitlab.dream.lab/api/v4/projects/<id>/terraform/state/<name>"
    lock_address   = "https://gitlab.dream.lab/api/v4/projects/<id>/terraform/state/<name>/lock"
    unlock_address = "https://gitlab.dream.lab/api/v4/projects/<id>/terraform/state/<name>/lock"
  }
}
```

### Operational secrets

Durable operational secrets are managed in OpenBao. No secrets are stored in Git.
On-disk exceptions with a documented lifecycle:

| Item                       | Location                      | Lifecycle                                             |
| -------------------------- | ----------------------------- | ----------------------------------------------------- |
| OpenTofu state (bootstrap) | host filesystem               | Removed after `tofu init -migrate-state` in Phase 1.4 |
| OpenBao backup token       | `/root/.openbao-backup-token` | Least-privilege read-only token; renewed periodically |

| Data                                 | Storage |
| ------------------------------------ | ------- |
| Talos secrets (PKI, machine configs) | OpenBao |
| kubeconfig                           | OpenBao |
| Ansible sensitive variables          | OpenBao |
| GitLab tokens, API keys              | OpenBao |

## Automation Model

| Layer                 | Tool            | Scope                                                      |
| --------------------- | --------------- | ---------------------------------------------------------- |
| Host OS               | Manual          | Ubuntu install, SSH keys, base user setup                  |
| Host configuration    | Ansible         | Incus install, ZFS pool, bridge network, autossh service   |
| VM provisioning       | OpenTofu        | Incus VMs, Talos machine configs                           |
| GitLab configuration  | Ansible         | GitLab CE install and configuration inside the GitLab VM   |
| step-ca configuration | Ansible         | step-ca install and configuration inside the step-ca-01 VM |
| OpenBao configuration | Ansible         | OpenBao install and configuration inside the openbao-01 VM |
| Kubernetes workloads  | ArgoCD (GitOps) | Platform services, applications                            |
