# Handoff Context

## Current State

Architecture and stack discussion in progress. No infrastructure has been deployed.
The repository contains documentation only.

**Physical host status:** Ubuntu 24.04.4 LTS (kernel 6.8.0-110-generic), libvirt fully
installed and active. 4 VMs currently running on libvirt — must be stopped and removed
before Incus installation. ~828 GB free in the Ubuntu LVM VG — to be used as Incus
storage pool. Windows dual-boot on nvme0n1 — do not touch.

## What Was Decided

Bottom-up architecture discussion completed through the Incus layer.

**Networking layer (2026-06-23):**
1. **Host OS** — Ubuntu 24.04 LTS retained, no reinstall
2. **Hypervisor** — Incus replaces libvirt stack
3. **VM networking** — incusbr0 bridge, 10.10.0.0/24, NAT to wlan0
4. **Remote access** — reverse SSH tunnel to VPS via autossh + systemd

**Incus layer (2026-06-24):**
5. **Incus install** — Zabbly repository
6. **Storage** — ZFS pool on LVM LV (~828 GB from ubuntu-vg)
7. **OpenTofu integration** — local Unix socket, no remote API
8. **Automation model** — manual → Ansible → OpenTofu → ArgoCD
9. **GitLab** — CE, standalone Incus VM outside Kubernetes
10. **GitLab Runner** — inside Kubernetes, Kubernetes executor

**DNS and PKI layer (2026-06-24):**
11. **Domain** — `dream.lab` (internal only)
12. **DNS** — Incus dnsmasq (VM names) + CoreDNS + k8s_gateway (platform services)
13. **PKI** — step-ca as dedicated Incus VM (`step-ca-01`), ACME issuer for cert-manager
14. **Certificates** — wildcard `*.dream.lab` from step-ca
15. **VM naming** — all hostnames numbered (step-ca-01, gitlab-01, talos-cp-01, etc.)

**Talos layer (2026-06-24):**
16. **K8s topology** — single control plane (talos-cp-01), 2 general workers + 1 GPU worker
17. **VM resources** — step-ca-01 (1/1GB/10GB), openbao-01 (1/2GB/20GB), gitlab-01 (4/6GB/200GB), talos-cp-01 (2/4GB/100GB), talos-worker-01/02 (6/18GB/200GB), talos-worker-gpu-01 (2/6GB/50GB). Total: 55 GB RAM, 9 GB reserve.

**Platform services layer (2026-06-24):**
18. **OpenBao** — standalone Incus VM (openbao-01), outside K8s, pre-K8s infrastructure
19. **Container registry** — GitLab Container Registry (built into gitlab-01)
20. **OpenTofu state** — GitLab HTTP backend (built-in, locking + versioning)
21. **Operational secrets** — OpenBao (Talos configs, kubeconfig, tokens)
21. **GPU** — NVIDIA RTX 3070 Ti, NVIDIA GPU Operator in K8s
19. **Secrets** — OpenBao
20. **Policy** — Kyverno
21. **Runtime security** — Tetragon
22. **Image scanning** — Trivy
23. **Observability** — kube-prometheus-stack + Loki + Tempo + OpenTelemetry Collector + Hubble
24. **Object storage** — MinIO
25. **Streaming** — Strimzi (Kafka)
26. **Batch processing** — Spark Operator
27. **Databases** — CloudNativePG (PostgreSQL), Altinity clickhouse-operator (ClickHouse)

See [decisions.md](decisions.md) for full rationale on each choice.

## Next Steps

- [ ] Roadmap — implementation phases and sequencing
- [ ] Kubernetes layer — Cilium config, Gateway API, cert-manager, ArgoCD bootstrap
- [ ] Infrastructure implementation — Ansible playbooks, OpenTofu modules

## Risks and Constraints

| Risk | Notes |
|------|-------|
| WiFi-only networking | NAT bridge mitigates; no L2 features available on uplink |
| Single physical host | No hardware redundancy; acceptable for training environment |
| DPI filtering (RU) | WireGuard may be blocked; reverse SSH used instead; AmneziaWG deferred |
| KVM stack migration | libvirt removal must not break existing qemu-kvm before Incus is ready |

## Validation Status

> Not validated — no infrastructure deployed yet.
