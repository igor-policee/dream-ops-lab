# Decisions

## 2026-06-23 — Incus as hypervisor over libvirt/KVM stack

**Decision:** Use Incus to manage VMs on the physical host. Remove libvirt, libvirtd,
and virt-manager. Retain kvm kernel module and qemu-kvm (Incus uses them directly).

**Reason:** Incus provides a modern, unified API for both VMs and containers, aligns
with the immutable/API-driven philosophy of the platform, and has cleaner integration
with Talos provisioning via Terraform.

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

## 2026-06-23 — Ubuntu 24.04 LTS as host OS

**Decision:** Retain existing Ubuntu 24.04 LTS installation. No reinstall needed.

**Reason:** Ubuntu 24.04 is well-supported for Incus (via Zabbly repository),
has good kernel support for eBPF and KVM, and is already installed.

**Alternatives considered:** Debian 12, NixOS.

**Trade-offs:** Ubuntu adds some default bloat vs Debian, but the difference is
negligible for a single-host lab. NixOS would be more declarative but adds
significant operational complexity for Incus.

