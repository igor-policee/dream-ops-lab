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
7. **Terraform integration** — local Unix socket, no remote API
8. **Automation model** — manual → Ansible → Terraform → ArgoCD
9. **GitLab** — CE, standalone Incus VM outside Kubernetes
10. **GitLab Runner** — inside Kubernetes, Kubernetes executor

**DNS and PKI layer (2026-06-24):**
11. **Domain** — `dream.lab` (internal only)
12. **DNS** — Incus dnsmasq (VM names) + CoreDNS + k8s_gateway (platform services)
13. **PKI** — step-ca as dedicated Incus VM (`step-ca-01`), ACME issuer for cert-manager
14. **Certificates** — wildcard `*.dream.lab` from step-ca
15. **VM naming** — all hostnames numbered (step-ca-01, gitlab-01, talos-cp-01, etc.)

See [decisions.md](decisions.md) for full rationale on each choice.

## Next Steps

Continue architecture discussion bottom-up:

- [ ] Talos layer — VM topology, resource allocation, disk layout, cluster config
- [ ] Kubernetes layer — Cilium config, Gateway API, add-ons
- [ ] Platform services — storage, secrets, policy, observability, security scanning

## Risks and Constraints

| Risk | Notes |
|------|-------|
| WiFi-only networking | NAT bridge mitigates; no L2 features available on uplink |
| Single physical host | No hardware redundancy; acceptable for training environment |
| DPI filtering (RU) | WireGuard may be blocked; reverse SSH used instead; AmneziaWG deferred |
| KVM stack migration | libvirt removal must not break existing qemu-kvm before Incus is ready |

## Validation Status

> Not validated — no infrastructure deployed yet.
