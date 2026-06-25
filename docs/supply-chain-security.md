# Supply Chain Security

## Overview

This document describes the software supply chain security controls in dream-ops-lab.
The goal is SLSA Level 2 compliance for all platform images.

## Pipeline security controls

### Source (GitLab)

| Control                    | Tool                  | Stage                  |
| -------------------------- | --------------------- | ---------------------- |
| Secret detection           | Gitleaks (pre-commit) | Developer workstation  |
| Secret detection           | Gitleaks (CI)         | GitLab CI: every push  |
| IaC misconfiguration       | Checkov               | GitLab CI: every push  |
| Dependency vulnerabilities | Dependency-Track      | GitLab CI: every build |

### Build (GitLab CI)

| Control                  | Tool                    | Stage                         |
| ------------------------ | ----------------------- | ----------------------------- |
| Image vulnerability scan | Trivy                   | After docker build            |
| IaC scan                 | Trivy config            | Every push with K8s manifests |
| SBOM generation          | Syft (CycloneDX format) | After docker build            |
| SBOM upload              | Dependency-Track API    | After Syft                    |
| Image signing            | Cosign                  | After Trivy passes            |

### Deploy (ArgoCD + Kubernetes)

| Control                       | Tool                          | Stage             |
| ----------------------------- | ----------------------------- | ----------------- |
| Signature verification        | Kyverno (verifyImages policy) | Admission control |
| Policy compliance             | Kyverno                       | Admission control |
| Runtime anomaly detection     | Tetragon                      | Runtime           |
| Continuous vulnerability scan | Trivy Operator                | Continuous        |

## Signing key management

- Cosign key pair generated once, stored in OpenBao at `supply-chain/cosign-key`
- Public key distributed as Kyverno policy ConfigMap
- Key rotation: manual, documented in [runbooks.md](runbooks.md)
- CI runner retrieves private key from OpenBao via AppRole at build time

## SBOM workflow

1. GitLab CI builds image → pushes to `registry.dream.lab`
2. Syft scans the pushed image → generates CycloneDX JSON SBOM
3. CI uploads SBOM to Dependency-Track via API
4. Dependency-Track correlates against NVD + OSV feeds
5. If CRITICAL vulnerability found → CI fails, merge blocked
6. SBOM stored as CI artifact for audit trail

## SLSA compliance status

| Requirement                | Level | Status                                                |
| -------------------------- | ----- | ----------------------------------------------------- |
| Version controlled source  | L1    | GitLab                                                |
| Scripted build             | L1    | GitLab CI                                             |
| Build service              | L2    | GitLab Runner in K8s                                  |
| Provenance available       | L2    | Cosign attestation                                    |
| Provenance authenticated   | L2    | Cosign signature                                      |
| Non-falsifiable provenance | L3    | Not implemented (would require SLSA generator action) |
