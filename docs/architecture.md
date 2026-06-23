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
        └── Talos VMs (static IPs within 10.10.0.0/24)
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
                          └── Talos VMs
```

Access pattern: SSH into the host via VPS reverse tunnel, then interact with all
components (kubectl, talosctl, incus CLI) directly from the host.

Ad-hoc local port forwarding is used when browser access to internal UIs is needed.

