# Handoff Context

## Current State

Architecture and stack discussion in progress. No infrastructure has been deployed.
The repository contains documentation only.

**Physical host status:** Ubuntu 24.04.4 LTS (kernel 6.8.0-110-generic), libvirt fully
installed and active. 4 VMs currently running on libvirt — must be stopped and removed
before Incus installation. ~828 GB free in the Ubuntu LVM VG — to be used as Incus
storage pool. Windows dual-boot on nvme0n1 — do not touch.

## What Was Decided (2026-06-23)

Bottom-up architecture discussion completed through the networking layer:

1. **Host OS** — Ubuntu 24.04 LTS retained, no reinstall
2. **Hypervisor** — Incus replaces libvirt stack
3. **VM networking** — incusbr0 bridge, 10.10.0.0/24, NAT to wlan0
4. **Remote access** — reverse SSH tunnel to VPS via autossh + systemd

See [decisions.md](decisions.md) for full rationale on each choice.

## Next Steps

Continue architecture discussion bottom-up:

- [ ] Incus layer — network config details, storage pools, VM templates
- [ ] Talos layer — VM topology, disk layout, cluster config
- [ ] Kubernetes layer — Cilium config, Gateway API, add-ons
- [ ] Platform services — storage, GitOps, CI/CD, secrets, policy, observability, security

## Risks and Constraints

| Risk | Notes |
|------|-------|
| WiFi-only networking | NAT bridge mitigates; no L2 features available on uplink |
| Single physical host | No hardware redundancy; acceptable for training environment |
| DPI filtering (RU) | WireGuard may be blocked; reverse SSH used instead; AmneziaWG deferred |
| KVM stack migration | libvirt removal must not break existing qemu-kvm before Incus is ready |

## Validation Status

> Not validated — no infrastructure deployed yet.
