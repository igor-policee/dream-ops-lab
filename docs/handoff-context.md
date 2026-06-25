# Handoff Context

## Current State

Architecture and stack discussion complete. No infrastructure has been deployed.
The repository contains documentation and Go tooling source code.

**Physical host status:** Ubuntu 24.04.4 LTS (kernel 6.8.0-110-generic), libvirt fully
installed and active. 4 VMs currently running on libvirt — must be stopped and removed
before Incus installation. ~828 GB free in the Ubuntu LVM VG — to be used as Incus
storage pool. Windows dual-boot on nvme0n1 — do not touch.

## What Was Decided

Bottom-up architecture discussion completed. See [decisions.md](decisions.md) for full rationale.

**Networking layer (2026-06-23):**
1. **Host OS** — Ubuntu 24.04 LTS retained, no reinstall
2. **Hypervisor** — Incus replaces libvirt stack
3. **VM networking** — incusbr0 bridge, 10.10.0.0/24, NAT to wlp5s0
4. **Remote access** — reverse SSH tunnel to dev-ubuntu-01 via autossh + systemd

**Incus layer (2026-06-24):**
5. **Incus install** — Zabbly repository
6. **Storage** — ZFS pool on LVM LV (~828 GB from ubuntu-vg)
7. **OpenTofu integration** — local Unix socket, no remote API
8. **Automation model** — manual → Ansible → OpenTofu → ArgoCD
9. **GitLab** — CE, standalone Incus VM outside Kubernetes
10. **GitLab Runner** — inside Kubernetes, Kubernetes executor

**DNS and PKI layer (2026-06-24):**
11. **Domain** — `dream.lab` (internal only)
12. **DNS** — Incus dnsmasq (VM names + aliases) + CoreDNS + k8s_gateway (platform services)
13. **DNS naming** — service DNS names carry no numeric suffix (gitlab.dream.lab, not gitlab-01.dream.lab)
14. **PKI** — step-ca as dedicated Incus VM (`step-ca-01`), ACME issuer for cert-manager
15. **Certificates** — wildcard `*.dream.lab` from step-ca
16. **VM naming** — all hostnames numbered (step-ca-01, gitlab-01, talos-cp-01, etc.)

**Talos and K8s layer (2026-06-24):**
17. **K8s topology** — single control plane (talos-cp-01), 1 general worker + 1 GPU worker
18. **VM resources** — step-ca-01 (1/1GB/10GB), openbao-01 (1/2GB/20GB), gitlab-01 (4/6GB/200GB), talos-cp-01 (2/4GB/100GB), talos-worker-01 (6/20GB/200GB), talos-worker-gpu-01 (6/20GB/200GB). Total: 53 GB RAM, 11 GB reserve.

**Supporting infrastructure (2026-06-24):**
19. **dev-ubuntu-01** — VPS with fixed public IP, online 24/7; reverse SSH tunnel endpoint + encrypted off-site backup storage (age + Bitwarden)
20. **Backup strategy** — age asymmetric encryption, systemd event triggers, dev-ubuntu-01 as destination, Bitwarden for keys and unseal shards

**Platform services layer (2026-06-24):**
21. **OpenBao** — standalone Incus VM (openbao-01), outside K8s, stores all operational secrets
22. **Container registry** — GitLab Container Registry (built into gitlab-01)
23. **OpenTofu state** — local backend during bootstrap → migrate to GitLab HTTP backend after Phase 1.4
24. **GPU** — NVIDIA RTX 3070 Ti, PCI passthrough to talos-worker-gpu-01, NVIDIA GPU Operator in K8s
25. **Policy** — Kyverno (extended: CIS benchmark policies, image signing enforcement)
26. **Runtime security** — Tetragon (custom TracingPolicies, Loki integration)
27. **Image scanning** — Trivy (CI + IaC + secret scan, SARIF output)
28. **Observability** — kube-prometheus-stack + Loki + Tempo + OpenTelemetry Collector + Hubble
29. **Object storage** — MinIO (deployed in Phase 5, before Loki/Tempo)
30. **Streaming** — Strimzi (Kafka, KRaft mode)
31. **Batch processing** — Spark Operator
32. **Databases** — CloudNativePG (PostgreSQL), Altinity clickhouse-operator (ClickHouse)

**Security and supply chain layer (2026-06-25):**
33. **Secret scanning** — Gitleaks (pre-commit + CI), Checkov (IaC), baseline in Phase 0.0
34. **Posture management** — Kubescape (Phase 4.1, NSA/CISA + CIS; Grafana dashboard in Phase 5.8)
35. **Image signing** — Cosign (Phase 4.4), key in OpenBao, enforced via Kyverno verifyImages
36. **SBOM + SCA** — Syft (CycloneDX) + Dependency-Track (Phase 4.6, NVD/OSV feeds, CI gate)
37. **Threat model** — living document updated at each phase security checkpoint
38. **Supply chain** — documented in supply-chain-security.md (SLSA Level 2 target)

**Go security tooling (2026-06-25):**
39. **dream-checker** — custom Go CLI (`tools/dream-checker/`); 4 modules: k8s (K8S-001..007), vault (VAULT-001..005), pki (PKI-001..004), supply (SUPPLY-001..005); JSON/table output; bootstrapped in Phase 3.9, CI integration in Phase 4.8/4.9; CronJob in `k8s/tools/`
40. **bao-rotator** — custom Go CLI (`tools/bao-rotator/`); list/rotate/audit commands for KV v2; 90-day rotation threshold; deployed as CronJob in Phase 4.9; slog-structured output
41. **Security posture dashboard** — Phase 5.9; Grafana dashboard aggregating dream-checker, Kubescape, Tetragon, Dependency-Track findings from Loki/Prometheus

## Next Steps

Architecture discussion complete. Implementation begins with Phase 0.
See [roadmap.md](roadmap.md) for full phase breakdown.

- [ ] Phase 0 — Host preparation (Ansible: Incus, ZFS, networking, autossh)
- [ ] Phase 1 — Pre-K8s VMs (step-ca-01, openbao-01, gitlab-01)
- [ ] Phase 2 — Kubernetes cluster (talos nodes, bootstrap)
- [ ] Phase 3 — K8s core (Cilium, CoreDNS, cert-manager, ArgoCD)
- [ ] Phase 4 — Security and policy
- [ ] Phase 5 — Observability
- [ ] Phase 6 — Data platform
- [ ] Phase 7 — GPU workloads
- [ ] Phase 8 — Optional (AmneziaWG, bao-rotator enhancements, SonarQube, Keycloak, Velero, Chaos Mesh, Backstage)

## Documentation Status

| Document | Status |
|----------|--------|
| README.md | Current |
| architecture.md | Current — dev-ubuntu-01 added as named system element |
| network-diagram.md | Current — dev-ubuntu-01 referenced by hostname |
| roadmap.md | Current — Phase 4 expanded, security checkpoints, ESO Phase 3.5, GPU Phase 7, Phase 3.9/4.8/4.9/5.9 added for Go tools |
| decisions.md | Current — all major architectural decisions recorded (Talos, Cilium, ArgoCD, ESO, cert-manager, DevSecOps tools, Gateway API, Go tooling) |
| runbooks.md | Current — GitLab backup added, retention 3 days everywhere, full recovery procedures |
| threat-model.md | Current — skeleton with assets, trust boundaries, 4 threat scenarios; updated per phase |
| supply-chain-security.md | Current — SLSA L2 target, Gitleaks/Checkov/Trivy/Syft/Cosign/Dependency-Track pipeline |
| docs/tools/dream-checker.md | Current — full module reference, CI integration, CronJob setup |
| docs/tools/bao-rotator.md | Current — commands, rotation policy, CronJob setup |
| handoff-context.md | Current |

## Risks and Constraints

| Risk | Notes |
|------|-------|
| WiFi-only networking | NAT bridge mitigates; no L2 features available on uplink |
| Single physical host | No hardware redundancy; acceptable for training environment |
| DPI filtering (RU) | WireGuard may be blocked; reverse SSH used instead; AmneziaWG deferred |
| KVM stack migration | libvirt removal must not break existing qemu-kvm before Incus is ready |
| SSD failure | GitLab data backed up to dev-ubuntu-01 (automated, 3-day retention). All other VM data (PostgreSQL, ClickHouse, MinIO, Kafka) is synthetic/educational and recreatable from scratch. Talos OS layer is stateless and rebuilt from configs stored in OpenBao. Risk accepted for training environment. |

## Validation Status

> Not validated — no infrastructure deployed yet.
