# Roadmap

Implementation phases ordered by dependency. Each phase must be completed before
the next begins. Order within a phase is sequential where noted.

---

## Phase 0 — Host preparation

**Tooling:** Ansible

### 0.0 Pre-flight: confirm backup boundary
> **Must complete before touching any host configuration.**

- [ ] Confirm backup boundary and risk acceptance: critical secrets and GitLab are backed up; synthetic K8s data is not; risk accepted for lab environment (see Risks in [handoff-context.md](handoff-context.md))
- [ ] Verify Windows dual-boot (nvme0n1) is not affected by planned changes
- [ ] Document rollback plan for libvirt removal (snapshot or note current VM state)
- [ ] Confirm ~828 GB LVM free space is available: `vgdisplay ubuntu-vg`

### 0.6 Security baseline
> **Must complete before any host configuration changes.**

- [ ] Install pre-commit framework on host: `pip install pre-commit`
- [ ] Create `.pre-commit-config.yaml` in infra repo with Gitleaks hook
- [ ] Run `pre-commit install` — all future commits checked for secrets automatically
- [ ] Install Checkov: `pip install checkov`
- [ ] Run initial Checkov scan on existing IaC files (if any): `checkov -d .`
- [ ] Document findings in `docs/security-baseline.md` (even if zero findings — establishes a baseline)
- [ ] Add `.gitleaks.toml` allowlist for known false positives (age key references, example tokens)

### 0.1 Remove libvirt stack
- [ ] Stop and destroy all 4 running libvirt VMs
- [ ] Purge libvirt, libvirtd, virt-manager, virtinst packages
- [ ] Verify kvm kernel module and qemu-kvm are still present

### 0.2 Install Incus
- [ ] Add Zabbly apt repository
- [ ] Install incus, incus-base packages
- [ ] Run `incus admin init` (non-interactive, via Ansible)

### 0.3 Configure ZFS storage pool
- [ ] Create LVM logical volume in ubuntu-vg (~800 GB)
- [ ] Create ZFS pool: `zpool create incus-pool /dev/ubuntu-vg/incus-zfs`
- [ ] Register pool as Incus storage backend

### 0.4 Configure networking
- [ ] Create incusbr0 bridge (10.10.0.0/24)
- [ ] Set Incus bridge DNS domain to `dream.lab`
- [ ] Enable IP forwarding
- [ ] Configure NAT (nftables/iptables) from incusbr0 → wlp5s0

### 0.5 Configure remote access
- [ ] Install autossh
- [ ] Create systemd service for reverse SSH tunnel to dev-ubuntu-01
- [ ] Enable and start service
- [ ] Verify SSH access through dev-ubuntu-01 → host

---

## Phase 1 — Pre-Kubernetes infrastructure

**Tooling:** OpenTofu (VM provisioning) + Ansible (configuration)

### 1.1 OpenTofu base module
- [ ] Install `age` on host
- [ ] Generate age key pair: `age-keygen -o /root/.age-backup.key` (mode 0400)
- [ ] Store age private key in Bitwarden as secure note "dream-ops-lab age backup key"
- [ ] Shred the private key file: `shred -u /root/.age-backup.key`
- [ ] Create tfstate backup directory on dev-ubuntu-01: `mkdir -p ~/backups/dream-ops-lab/tfstate`
- [ ] Write reusable OpenTofu module for Incus VM (CPU, RAM, disk, network, cloud-init)
- [ ] Use local state backend — GitLab is not yet available at this stage
- [ ] Run a manual tfstate backup after each `tofu apply` until Phase 1.4 is complete (see "Manual tfstate Backup" in [runbooks.md](runbooks.md))

### 1.2 Provision and configure step-ca-01
- [ ] Provision VM via OpenTofu
- [ ] Install step-ca (Ansible)
- [ ] Initialize PKI: generate root CA and intermediate CA
- [ ] Configure ACME provisioner (HTTP challenge via internal network)
- [ ] Export root certificate → distribute to host trust store

### 1.3 Provision and configure openbao-01
- [ ] Provision VM via OpenTofu
- [ ] Install OpenBao (Ansible)
- [ ] Initialize and unseal OpenBao
- [ ] Configure AppRole auth method
- [ ] Create initial policies (admin, infra-read, k8s-app, backup)
- [ ] Store step-ca root certificate in OpenBao
- [ ] Store unseal key shards and CA password in Bitwarden (see [runbooks.md](runbooks.md))

### 1.4 Provision and configure gitlab-01
- [ ] Provision VM via OpenTofu
- [ ] Install GitLab CE via official package (Ansible)
- [ ] Obtain TLS certificate from step-ca via ACME
- [ ] Configure GitLab: domain (`gitlab.dream.lab`), registry, SSH
- [ ] Create GitLab groups and infrastructure repositories
- [ ] Enable GitLab Container Registry
- [ ] Create infrastructure project in GitLab and enable the Terraform/OpenTofu state backend
- [ ] Add `backend "http"` block to OpenTofu configuration pointing to GitLab
- [ ] Run `tofu init -migrate-state` to migrate local state to the GitLab HTTP backend
- [ ] Verify state appears in GitLab: project → Operate → Terraform states
- [ ] Remove local state files from host: `rm -f terraform.tfstate terraform.tfstate.backup`
- [ ] Archive or remove tfstate backups from dev-ubuntu-01 after confirming GitLab state is correct: `ssh dev-ubuntu-01 "rm -rf ~/backups/dream-ops-lab/tfstate"`

### 1.5 Configure backup automation
- [ ] Create backup directories on dev-ubuntu-01: `mkdir -p ~/backups/dream-ops-lab/{step-ca,openbao,gitlab}`
- [ ] Create dedicated OpenBao backup token with `sys/storage/raft/snapshot` policy
- [ ] Deploy backup script to host at `/usr/local/bin/dream-ops-backup.sh`
- [ ] Deploy systemd service (`dream-ops-backup.service`) and timer (`dream-ops-backup.timer`)
- [ ] Enable timer: `systemctl enable --now dream-ops-backup.timer`
- [ ] Trigger manual run and verify encrypted files appear on dev-ubuntu-01

### 1.6 Configure DNS
- [ ] Configure Incus dnsmasq to serve `dream.lab` for VM hostnames (auto-registered as `<hostname>.dream.lab`)
- [ ] Add static service aliases in dnsmasq — service DNS names use no numbers:
  - [ ] `gitlab.dream.lab` → `gitlab-01`
  - [ ] `step-ca.dream.lab` → `step-ca-01`
  - [ ] `openbao.dream.lab` → `openbao-01`
- [ ] Verify both resolve: `gitlab-01.dream.lab` (VM hostname) and `gitlab.dream.lab` (service name)

### 1.7 Phase 1 security checkpoint

- [ ] Run Checkov against all OpenTofu modules written in Phase 1: `checkov -d infra/`
- [ ] Verify no secrets committed to Git: `gitleaks detect --source=. --verbose`
- [ ] Verify OpenBao audit logging is enabled: `bao audit list`
- [ ] Document threat model entry for Phase 1 components in [docs/threat-model.md](threat-model.md):
  - step-ca-01: attack surface, blast radius if compromised
  - openbao-01: attack surface, blast radius if compromised
  - gitlab-01: attack surface, blast radius if compromised
- [ ] Verify TLS on all inter-VM communication (step-ca certificates in use)
- [ ] Verify OpenBao AppRole credentials are least-privilege (review policy scope)

---

## Phase 2 — Kubernetes cluster

**Tooling:** OpenTofu + talosctl

### 2.1 Prepare Talos configuration
- [ ] Generate Talos secrets: `talosctl gen secrets` → store in OpenBao
- [ ] Generate machine configs for control plane and workers
- [ ] Store machine configs in OpenBao

### 2.2 Provision Talos VMs
- [ ] Provision talos-cp-01 via OpenTofu (Talos ISO image)
- [ ] Provision talos-worker-01, talos-worker-gpu-01 via OpenTofu
- [ ] Apply machine configs via talosctl

### 2.3 Bootstrap cluster
- [ ] Run `talosctl bootstrap` on talos-cp-01
- [ ] Wait for control plane to be ready
- [ ] Generate kubeconfig → store in OpenBao
- [ ] Verify cluster: `kubectl get nodes`

### 2.4 Phase 2 security checkpoint

- [ ] Run kube-bench on talos-cp-01 immediately after bootstrap: `kubectl apply -f https://raw.githubusercontent.com/aquasecurity/kube-bench/main/job.yaml`
- [ ] Save kube-bench output as baseline: `kubectl logs job/kube-bench > docs/kube-bench-baseline.txt`
- [ ] Document cluster threat model entry in [docs/threat-model.md](threat-model.md):
  - K8s API server: who has access, from where
  - etcd: encryption at rest status
  - Node-to-node traffic: encrypted via Cilium WireGuard or not
- [ ] Verify Talos machine configs have no hardcoded secrets
- [ ] Verify kubeconfig stored in OpenBao, not on host filesystem

---

## Phase 3 — Kubernetes core

**Tooling:** Helm (Cilium bootstrap) → ArgoCD (everything else)

Order is strict within this phase.

### 3.1 Install Cilium
- [ ] Install Cilium via Helm (before ArgoCD — CNI must exist first)
- [ ] Enable eBPF dataplane, kube-proxy replacement
- [ ] Verify all nodes Ready

### 3.2 Configure DNS
- [ ] Install CoreDNS with k8s_gateway plugin (Helm)
- [ ] Configure k8s_gateway to serve Gateway API resources as DNS records
- [ ] Create `CiliumLoadBalancerIPPool` reserving 10.10.0.53 for CoreDNS
- [ ] Create `CiliumL2AnnouncementPolicy` to advertise LoadBalancer IPs on incusbr0
- [ ] Expose CoreDNS via Cilium LoadBalancer at stable IP (10.10.0.53)
- [ ] Update Incus dnsmasq: forward `dream.lab` → 10.10.0.53
- [ ] Verify 10.10.0.53 is reachable from host: `dig @10.10.0.53 argocd.dream.lab`
- [ ] Verify pod DNS resolution for `dream.lab` and `cluster.local`

### 3.3 Install cert-manager
- [ ] Deploy cert-manager (Helm)
- [ ] Create ClusterIssuer pointing to step-ca-01 ACME endpoint
- [ ] Verify certificate issuance with a test Certificate resource

### 3.4 Bootstrap ArgoCD
- [ ] Deploy ArgoCD (Helm)
- [ ] Configure ArgoCD to authenticate with GitLab
- [ ] Create App-of-Apps root application pointing to infrastructure repo
- [ ] All subsequent deployments managed through ArgoCD

### 3.5 Configure Cilium Gateway API
- [ ] Enable Gateway API CRDs
- [ ] Create GatewayClass and default Gateway
- [ ] Verify platform service routing (test with a sample HTTPRoute)

### 3.6 Phase 3 security checkpoint

- [ ] Verify all platform services have TLS (cert-manager issued certificates from step-ca)
- [ ] Verify ArgoCD uses OIDC or SSO — no local admin password in use
- [ ] Verify Cilium NetworkPolicy denies all pod-to-pod traffic by default (default-deny baseline)
- [ ] Run Kubescape scan against NSA/CISA framework: `kubescape scan framework nsa`
- [ ] Save Kubescape output as baseline: `kubescape scan framework nsa --format json > docs/kubescape-baseline.json`
- [ ] Document network security model in [docs/threat-model.md](threat-model.md):
  - Which namespaces can talk to which
  - External ingress points
  - Egress policy

---

## Phase 4 — Security and policy

**Tooling:** ArgoCD

Order is strict within this phase.

### 4.1 Kubescape (posture baseline)

- [ ] Deploy Kubescape operator via ArgoCD
- [ ] Run NSA/CISA and CIS benchmark scans
- [ ] Configure continuous scanning with results stored in Prometheus metrics
- [ ] Expose Kubescape dashboard in Grafana (after Phase 5)
- [ ] Fix all CRITICAL findings before proceeding to 4.2

### 4.2 External Secrets Operator

- [ ] Retrieve ESO AppRole credentials from OpenBao (role_id + secret_id for `k8s-app` policy)
- [ ] Create bootstrap K8s secret with AppRole credentials:
  `kubectl create secret generic openbao-approle --from-literal=role-id=<id> --from-literal=secret-id=<id> -n external-secrets`
- [ ] Deploy External Secrets Operator
- [ ] Create ClusterSecretStore pointing to openbao-01 using the bootstrap secret
- [ ] Verify secret sync with a test ExternalSecret
- [ ] Delete bootstrap secret after ESO is operational (ESO manages its own auth from this point)

### 4.3 Trivy in CI pipeline

- [ ] Add Trivy scanning stage to GitLab CI pipeline template (`.gitlab-ci.yml` shared template)
- [ ] Configure to fail on CRITICAL and HIGH CVEs
- [ ] Configure SARIF output: `trivy image --format sarif --output trivy-results.sarif`
- [ ] Integrate scan results with GitLab Security Dashboard
- [ ] Add Trivy IaC scan for Kubernetes manifests: `trivy config k8s/`
- [ ] Add Trivy secret scan: `trivy fs --scanners secret .`

### 4.4 Kyverno

- [ ] Deploy Kyverno via ArgoCD
- [ ] Apply baseline policies:
  - [ ] Require resource limits on all pods
  - [ ] Disallow privileged containers
  - [ ] Require labels (app, environment, team)
  - [ ] Disallow `latest` image tag
  - [ ] Require read-only root filesystem
  - [ ] Disallow hostPath volumes
  - [ ] Require non-root user (runAsNonRoot: true)
- [ ] Apply CIS Kubernetes Benchmark policies via Kyverno (from kyverno/policies repo)
- [ ] Configure Kyverno in audit mode first, then enforce after validating no breakage

### 4.5 Image signing (supply chain security)

- [ ] Install Cosign in GitLab CI runner
- [ ] Generate Cosign key pair, store private key in OpenBao
- [ ] Add Cosign signing step to GitLab CI after successful build and Trivy scan:
  `cosign sign --key <key-from-openbao> registry.dream.lab/image:tag`
- [ ] Add Kyverno policy to require signed images in production namespace:
  - [ ] Verify signature against known public key
  - [ ] Block unsigned images from deploying to `production` namespace
- [ ] Test policy: verify unsigned image is blocked, signed image is allowed
- [ ] Document signing workflow in [docs/supply-chain-security.md](supply-chain-security.md)

### 4.6 Tetragon

- [ ] Deploy Tetragon via ArgoCD
- [ ] Configure TracingPolicies for:
  - [ ] Process execution events (unexpected shells, curl, wget in containers)
  - [ ] File access events (sensitive paths: /etc/passwd, /proc/*, /var/run/secrets)
  - [ ] Network events (unexpected outbound connections)
- [ ] Verify event stream: `tetra getevents`
- [ ] Configure Tetragon JSON output → OTel Collector → Loki (after Phase 5)
- [ ] Write at least one custom TracingPolicy for a known attack pattern (e.g., crypto miner detection)

### 4.7 Dependency-Track (SCA and SBOM)

- [ ] Deploy Dependency-Track (API server + frontend) via ArgoCD
- [ ] Generate SBOM on every build in GitLab CI using Syft:
  `syft packages registry.dream.lab/image:tag -o cyclonedx-json > sbom.json`
- [ ] Push SBOM to Dependency-Track from GitLab CI after each successful build
- [ ] Configure vulnerability feeds (NVD, OSV)
- [ ] Set alert thresholds: CRITICAL findings block merge via GitLab CI gate
- [ ] Document SBOM workflow in [docs/supply-chain-security.md](supply-chain-security.md)

### 4.8 Phase 4 security checkpoint

- [ ] All production-namespace workloads pass Kyverno policies without exceptions
- [ ] All images in Container Registry are Cosign-signed
- [ ] Tetragon events are flowing to Loki (verify after Phase 5)
- [ ] Dependency-Track shows zero CRITICAL vulnerabilities in platform images
- [ ] Kubescape score improved vs baseline from 4.1
- [ ] Update [docs/threat-model.md](threat-model.md) with runtime security coverage

---

## Phase 5 — Observability

**Tooling:** ArgoCD

### 5.1 MinIO
- [ ] Deploy MinIO (standalone mode)
- [ ] Create buckets: loki, tempo, spark, data
- [ ] Configure lifecycle policies

### 5.2 kube-prometheus-stack
- [ ] Deploy Prometheus + Alertmanager + Grafana
- [ ] Configure scraping for all platform components
- [ ] Import baseline K8s dashboards

### 5.3 Loki
- [ ] Deploy Loki
- [ ] Deploy Promtail / OTel Collector as log forwarder on each node
- [ ] Configure MinIO as Loki storage backend

### 5.4 Tempo
- [ ] Deploy Tempo
- [ ] Configure MinIO as Tempo storage backend

### 5.5 OpenTelemetry Collector
- [ ] Deploy OTel Collector as DaemonSet
- [ ] Configure pipelines: metrics → Prometheus, logs → Loki, traces → Tempo

### 5.6 Hubble
- [ ] Enable Hubble UI (included with Cilium)
- [ ] Expose via Gateway API
- [ ] Verify network flow visibility

### 5.7 Grafana datasources
- [ ] Configure Prometheus, Loki, Tempo datasources
- [ ] Create unified dashboards for platform health

---

## Phase 6 — Data platform

**Tooling:** ArgoCD

### 6.1 CloudNativePG
- [ ] Deploy CloudNativePG operator
- [ ] Create initial PostgreSQL cluster (3-instance HA)
- [ ] Configure backups to MinIO

### 6.2 ClickHouse
- [ ] Deploy Altinity clickhouse-operator
- [ ] Create initial ClickHouse cluster
- [ ] Configure backups to MinIO

### 6.3 Strimzi (Kafka)
- [ ] Deploy Strimzi operator
- [ ] Create Kafka cluster (3 brokers, KRaft mode)
- [ ] Create initial topics

### 6.4 Spark Operator
- [ ] Deploy Spark Operator
- [ ] Run test SparkApplication to verify cluster connectivity

---

## Phase 7 — GPU workloads

**Tooling:** Ansible (host) + OpenTofu (VM) + ArgoCD (operator)

### 7.1 Configure PCI passthrough on host
- [ ] Enable IOMMU in GRUB (intel_iommu=on)
- [ ] Bind RTX 3070 Ti to vfio-pci driver (Ansible)
- [ ] Verify GPU in VFIO group: `ls /sys/bus/pci/drivers/vfio-pci`

### 7.2 Provision talos-worker-gpu-01
- [ ] Provision VM via OpenTofu with GPU PCI passthrough config
- [ ] Apply Talos machine config
- [ ] Add node to cluster, verify it joins

### 7.3 NVIDIA GPU Operator
- [ ] Deploy NVIDIA GPU Operator via ArgoCD
- [ ] Verify GPU resource available: `kubectl describe node talos-worker-gpu-01`
- [ ] Run test GPU workload (CUDA vector-add)

---

## Phase 8 — Research and ideas

> This phase is not a planned implementation sequence. It is a backlog of tools and
> practices worth exploring when resources and motivation allow. Items may be
> adopted, dropped, or reprioritized as the platform evolves.

### 8.1 AmneziaWG (alternative remote access)
- [ ] Install AmneziaWG on host and dev-ubuntu-01
- [ ] Configure obfuscated WireGuard tunnel
- [ ] Test connectivity through RU ISP DPI

### 8.2 Go tool: OpenBao secret rotator

> A custom Go CLI tool that demonstrates ability to build security tooling, not just deploy it.

- [ ] Create subdirectory `tools/bao-rotator/`
- [ ] Implement CLI tool in Go with the following capabilities:
  - [ ] Connect to OpenBao via AppRole authentication (reads credentials from env or K8s secret)
  - [ ] List secrets at a given path with their metadata (created_time, last_rotated)
  - [ ] Rotate a secret at a given path (generate new value, write to OpenBao, log the event)
  - [ ] Output structured JSON logs compatible with Loki (timestamp, action, path, actor)
  - [ ] Dry-run mode: show what would be rotated without making changes
- [ ] Add Kubernetes CronJob manifest to run rotation on schedule
- [ ] Write unit tests (Go testing package, mock OpenBao client)
- [ ] Add to GitLab CI: build, test, Trivy scan, Cosign sign
- [ ] Document usage in `docs/bao-rotator.md`

**Stack:** Go + OpenBao API client + cobra CLI + structured logging (slog)

### 8.3 SonarQube (SAST and code quality)
- [ ] Deploy SonarQube Community Edition
- [ ] Integrate with GitLab CI (sonar-scanner stage in pipeline templates)
- [ ] Configure quality gates to block merge on critical issues
- [ ] Enable secrets detection and security hotspot review

### 8.4 Dependency-Track (extended SCA)
- [ ] Extend Phase 4.7 Dependency-Track with multi-project SBOM aggregation
- [ ] Configure integration with SonarQube findings
- [ ] Set up custom component license policies

### 8.5 Keycloak (SSO and OIDC)
- [ ] Deploy Keycloak
- [ ] Configure OIDC integration for: GitLab, ArgoCD, Grafana, OpenBao
- [ ] Set up roles and groups mirroring platform access model
- [ ] Enable MFA for admin accounts

### 8.6 Velero (cluster backup and restore)
- [ ] Deploy Velero with MinIO as S3 backend
- [ ] Configure scheduled backups (cluster state, PVCs)
- [ ] Document and test restore procedure
- [ ] Verify backup integrity with a test restore to a vCluster

### 8.7 Chaos Mesh (chaos engineering)
- [ ] Deploy Chaos Mesh
- [ ] Define baseline SLOs for platform services (Prometheus rules)
- [ ] Run initial chaos experiments: pod kill, network partition, disk pressure
- [ ] Verify observability stack captures events correctly

### 8.8 Backstage (internal developer portal)
- [ ] Deploy Backstage
- [ ] Integrate with GitLab as catalog source (GitLab discovery plugin)
- [ ] Register all platform services in the Software Catalog
- [ ] Add TechDocs integration for documentation-as-code
