# Threat Model

## Purpose

This document tracks the security posture of dream-ops-lab from an attacker's
perspective. Updated at the end of each phase checkpoint.

## Scope

Single physical host, single Kubernetes cluster, internal network only
(10.10.0.0/24). No external services exposed except the reverse SSH tunnel
via dev-ubuntu-01.

## Assets

| Asset | Sensitivity | Location |
|-------|-------------|----------|
| CA root key | CRITICAL | step-ca-01 `/etc/step-ca/secrets/` |
| OpenBao unseal keys | CRITICAL | Bitwarden (offline, 5 shards, threshold 3-of-5) |
| Talos secrets bundle | HIGH | OpenBao `talos/secrets` |
| kubeconfig | HIGH | OpenBao `k8s/kubeconfig` |
| GitLab root token | HIGH | OpenBao `gitlab/root-token` |
| Cosign private key | HIGH | OpenBao `supply-chain/cosign-key` |
| OpenBao backup token | LOW | Host `/root/.openbao-backup-token` (read-only, raft snapshot only) |

## Trust boundaries

| Boundary | Control |
|----------|---------|
| Internet → host | Reverse SSH tunnel only; key-based auth; no password auth |
| Host → VMs | incusbr0 bridge; NAT outbound; no inbound from VMs to host by default |
| VM → VM | No explicit firewall (lab); mitigated by K8s NetworkPolicy for workloads |
| Pod → Pod (K8s) | Cilium NetworkPolicy; default-deny baseline |
| Pod → OpenBao | AppRole authentication; least-privilege policies |
| GitLab CI → K8s | ServiceAccount with scoped RBAC; no cluster-admin |

## Threat scenarios

### T1: Compromised GitLab runner pod
**Attack path:** Malicious dependency in build → RCE in runner pod → access to K8s
ServiceAccount token → lateral movement.

**Controls:** Kyverno (restricted pod spec), Tetragon (unexpected process/network),
RBAC (scoped ServiceAccount).

**Blast radius:** Build namespace only; no access to production secrets.

---

### T2: Compromised container image
**Attack path:** Supply chain attack on base image → deployed to cluster → runtime exploit.

**Controls:** Trivy scan in CI (known CVEs), Cosign signing (integrity), Kyverno
(unsigned image blocked), Dependency-Track (SBOM vulnerability tracking), Tetragon
(runtime anomaly detection).

**Blast radius:** Single pod namespace; limited by NetworkPolicy.

---

### T3: Exposed OpenBao credentials
**Attack path:** Secret leaked in Git or logs → attacker authenticates to OpenBao
→ reads all secrets.

**Controls:** Gitleaks (pre-commit + CI), AppRole least-privilege policies, OpenBao
audit logging, secret rotation via bao-rotator (Phase 8.2).

**Blast radius:** Depends on which AppRole is compromised; `k8s-app` policy cannot
read CA material.

---

### T4: Compromised step-ca
**Attack path:** RCE on step-ca-01 VM → access to root CA key → forge certificates
for any `dream.lab` domain.

**Controls:** Dedicated VM (minimal attack surface); no K8s workloads on this VM;
CA key passphrase stored in Bitwarden only.

**Blast radius:** All TLS within dream.lab is untrusted; requires full PKI rebuild.

---

## Phase completion status

| Phase | Checkpoint | Status |
|-------|-----------|--------|
| Phase 1 | 1.7 security checkpoint | Not started |
| Phase 2 | 2.4 kube-bench baseline | Not started |
| Phase 3 | 3.8 Kubescape baseline, network model | Not started |
| Phase 4 | 4.7 runtime security coverage | Not started |

## Residual risks

| Risk | Accepted? | Reason |
|------|-----------|--------|
| No VM-to-VM firewall rules on incusbr0 | Yes | Lab environment; K8s NetworkPolicy covers workloads |
| Single control plane node | Yes | Lab environment; etcd backup via Velero (Phase 8.6) |
| WiFi uplink (no redundancy) | Yes | Lab environment; physical access available |
| OpenBao auto-unseal not configured | Yes | Lab environment; manual unseal on restart is acceptable |
