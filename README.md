# dream-ops-lab

A modern DevSecOps platform built for hands-on training and experimentation.

## Goal

Build a fully functional, production-grade DevSecOps platform on a single physical
machine using an immutable, API-driven infrastructure philosophy.

## Hardware

| Component | Spec |
|-----------|------|
| CPU | Intel Core i7 |
| RAM | 64 GB |
| Storage | 900 GB SSD |
| Network | WiFi (802.11, no ethernet) |

## Core Stack

| Layer | Technology |
|-------|------------|
| Host OS | Ubuntu 24.04 LTS |
| Hypervisor | Incus |
| VM OS | Talos Linux |
| Kubernetes | v1.35.x |
| CNI | Cilium (eBPF, kube-proxy replacement) |
| Provisioning | Terraform |

## Documentation

- [Architecture](docs/architecture.md) — components, topology, networking model
- [Decisions](docs/decisions.md) — design decisions and trade-offs
- [Handoff Context](docs/handoff-context.md) — current state and next steps

## Status

> Architecture and stack discussion in progress. No infrastructure deployed yet.
