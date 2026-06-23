# Architecture

## Overview

Single physical host running Incus as a hypervisor. Talos Linux VMs form a
Kubernetes cluster on top. All layers follow an immutable, API-driven approach.

## Physical Host

| Component | Detail |
|-----------|--------|
| Hostname | homelab-ubuntu |
| Hardware | MSI MAG Z590 Codex X5 |
| CPU | Intel Core i7-11700KF @ 3.60 GHz (8 cores / 16 threads) |
| RAM | 32 GB installed / 64 GB planned (additional DIMMs purchased, not yet installed) |
| OS | Ubuntu 24.04.4 LTS |
| Kernel | 6.8.0-110-generic |

### Storage

| Device | Size | Role |
|--------|------|------|
| sda | 931.5 GB | Ubuntu (LVM). Only 100 GB allocated to root LV; ~828 GB free in VG — reserved for Incus storage pool |
| nvme0n1 | 953.9 GB | Windows (dual boot, NTFS) — do not modify |

### Hypervisor stack

Incus replaces libvirt. The kvm kernel module and qemu-kvm are retained — used by Incus directly.

Current state: libvirt fully installed and running, 4 VMs active. Must be shut down and removed before Incus installation.

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

### Terraform integration

Terraform manages Incus resources via the incus provider using a local Unix socket.
No remote API or TLS configuration required.

### VM inventory

| VM | Role |
|----|------|
| step-ca-01 | Internal PKI / Certificate Authority — first to be provisioned |
| gitlab-01 | GitLab CE — source control and CI/CD, outside Kubernetes |
| talos-cp-01, talos-cp-02, talos-cp-03 | Talos control plane nodes |
| talos-worker-01, talos-worker-N | Talos worker nodes |

All VMs use numbered hostnames regardless of expected replica count.
VM resource allocation to be decided in the Talos layer.

### GitLab Runner

Runs inside Kubernetes as a pod (Kubernetes executor). Handles application
pipelines: image builds, tests, deployments. Cluster infrastructure (Terraform,
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
  └── VPS (public IP)
        ← reverse SSH tunnel (outbound from host, persistent via autossh + systemd)
              └── Physical Host (:22)
                    └── incusbr0 (10.10.0.0/24)
                          └── Incus VMs
```

Access pattern: SSH into the host via VPS reverse tunnel, then interact with all
components (kubectl, talosctl, incus CLI) directly from the host.

Ad-hoc local port forwarding is used when browser access to internal UIs is needed.

## DNS and PKI

### Internal domain

All platform services use the internal domain `dream.lab`.

### DNS architecture

Two servers with distinct roles. See [network-diagram.md](network-diagram.md) for the
full resolution flow.

**Incus dnsmasq** (10.10.0.1, on incusbr0):
- Authoritative for VM hostnames within `dream.lab` (auto-registered on VM start)
- Forwards unresolved `dream.lab` queries to CoreDNS
- Forwards all other queries upstream (router / internet)

**CoreDNS** (stable IP via Cilium LoadBalancer, reachable from incusbr0):
- Authoritative for platform service names in `dream.lab` via the `k8s_gateway` plugin
- `k8s_gateway` watches Gateway API resources and auto-generates DNS records
- Handles `*.cluster.local` for K8s internal service discovery
- Forwards unresolved `dream.lab` queries to Incus dnsmasq (for VM names)

### PKI

step-ca runs as a dedicated Incus VM (`step-ca-01`). It is the root CA for the entire
platform and is provisioned before any other VM.

- Issues certificates via ACME protocol
- cert-manager in Kubernetes uses step-ca as its ACME issuer
- All platform services (GitLab, Talos, K8s ingress) receive certificates from step-ca
- Wildcard certificate `*.dream.lab` used for platform UIs

step-ca is independent of Kubernetes — certificates can be issued before the cluster
exists and during cluster rebuilds.

## Automation Model

| Layer | Tool | Scope |
|-------|------|-------|
| Host OS | Manual | Ubuntu install, SSH keys, base user setup |
| Host configuration | Ansible | Incus install, ZFS pool, bridge network, autossh service |
| VM provisioning | Terraform | Incus VMs, Talos machine configs |
| GitLab configuration | Ansible | GitLab CE install and configuration inside the GitLab VM |
| step-ca configuration | Ansible | step-ca install and configuration inside the step-ca-01 VM |
| Kubernetes workloads | ArgoCD (GitOps) | Platform services, applications |

