# dream-ops-lab

A modern DevSecOps platform designed for hands-on experimentation and engineering solution exploration.

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
| Hypervisor | Incus (ZFS storage, NAT bridge) |
| VM OS | Talos Linux |
| Kubernetes | v1.35.x |
| CNI | Cilium (eBPF, kube-proxy replacement) |
| Provisioning | OpenTofu + Ansible |
| Source control / CI | GitLab CE |
| GitOps | ArgoCD |
| Container registry | GitLab Container Registry |
| State backend | GitLab HTTP backend (OpenTofu state) |
| Internal domain | dream.lab |
| PKI | step-ca (internal CA, ACME) |
| Secrets | OpenBao |
| Policy | Kyverno |
| Runtime security | Tetragon |
| Image scanning | Trivy |
| Observability | kube-prometheus-stack + Loki + Tempo + OTel Collector + Hubble |
| Object storage | MinIO |
| Databases | CloudNativePG (PostgreSQL) · Altinity (ClickHouse) |
| Streaming | Strimzi (Kafka) |
| Batch processing | Spark Operator |
| GPU | NVIDIA GPU Operator (RTX 3070 Ti) |

## Documentation

- [Architecture](docs/architecture.md) — components, topology, networking model
- [Network Diagram](docs/network-diagram.md) — DNS resolution flow, PKI, topology
- [Roadmap](docs/roadmap.md) — implementation phases and sequencing
- [Decisions](docs/decisions.md) — design decisions and trade-offs
- [Handoff Context](docs/handoff-context.md) — current state and next steps

## Status

> Architecture and stack discussion in progress. No infrastructure deployed yet.
