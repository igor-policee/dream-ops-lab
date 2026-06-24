# Roadmap

Implementation phases ordered by dependency. Each phase must be completed before
the next begins. Order within a phase is sequential where noted.

---

## Phase 0 — Host preparation

**Tooling:** Ansible

### 0.0 Pre-flight: confirm backup boundary
> **Must complete before touching any host configuration.**

- Confirm full-disk backup strategy is planned and in place (see Risks in [handoff-context.md](handoff-context.md))
- Verify Windows dual-boot (nvme0n1) is not affected by planned changes
- Document rollback plan for libvirt removal (snapshot or note current VM state)
- Confirm ~828 GB LVM free space is available: `vgdisplay ubuntu-vg`

### 0.1 Remove libvirt stack
- Stop and destroy all 4 running libvirt VMs
- Purge libvirt, libvirtd, virt-manager, virtinst packages
- Verify kvm kernel module and qemu-kvm are still present

### 0.2 Install Incus
- Add Zabbly apt repository
- Install incus, incus-base packages
- Run `incus admin init` (non-interactive, via Ansible)

### 0.3 Configure ZFS storage pool
- Create LVM logical volume in ubuntu-vg (~800 GB)
- Create ZFS pool: `zpool create incus-pool /dev/ubuntu-vg/incus-zfs`
- Register pool as Incus storage backend

### 0.4 Configure networking
- Create incusbr0 bridge (10.10.0.0/24)
- Set Incus bridge DNS domain to `dream.lab`
- Enable IP forwarding
- Configure NAT (nftables/iptables) from incusbr0 → wlp5s0

### 0.5 Configure remote access
- Install autossh
- Create systemd service for reverse SSH tunnel to dev-ubuntu-01
- Enable and start service
- Verify SSH access through dev-ubuntu-01 → host

---

## Phase 1 — Pre-Kubernetes infrastructure

**Tooling:** OpenTofu (VM provisioning) + Ansible (configuration)

### 1.1 OpenTofu base module
- Write reusable OpenTofu module for Incus VM (CPU, RAM, disk, network, cloud-init)
- Use local state backend initially — GitLab is not yet available at this stage

### 1.2 Provision and configure step-ca-01
- Provision VM via OpenTofu
- Install step-ca (Ansible)
- Initialize PKI: generate root CA and intermediate CA
- Configure ACME provisioner (HTTP challenge via internal network)
- Export root certificate → distribute to host trust store

### 1.3 Provision and configure openbao-01
- Provision VM via OpenTofu
- Install OpenBao (Ansible)
- Initialize and unseal OpenBao
- Configure AppRole auth method
- Create initial policies (admin, infra-read, k8s-app, backup)
- Store step-ca root certificate in OpenBao
- Store unseal key shards and CA password in Bitwarden (see [runbooks.md](runbooks.md))

### 1.4 Provision and configure gitlab-01
- Provision VM via OpenTofu
- Install GitLab CE via official package (Ansible)
- Obtain TLS certificate from step-ca via ACME
- Configure GitLab: domain (`gitlab.dream.lab`), registry, SSH
- Create GitLab groups and infrastructure repositories
- Enable GitLab Container Registry
- Create infrastructure project in GitLab and enable the Terraform/OpenTofu state backend
- Add `backend "http"` block to OpenTofu configuration pointing to GitLab
- Run `tofu init -migrate-state` to migrate local state to the GitLab HTTP backend
- Verify state appears in GitLab: project → Operate → Terraform states
- Remove local state files from host: `rm -f terraform.tfstate terraform.tfstate.backup`
- Remove tfstate backups from dev-ubuntu-01: `ssh dev-ubuntu-01 "rm -rf ~/backups/dream-ops-lab/tfstate"`

### 1.5 Configure backup automation
- Install `age` on host
- Generate age key pair: `age-keygen -o /root/.age-backup.key` (mode 0400)
- Store age private key in Bitwarden as secure note "dream-ops-lab age backup key"
- Create backup directories on dev-ubuntu-01: `~/backups/dream-ops-lab/{step-ca,openbao,tfstate}`
- Create dedicated OpenBao backup token with `sys/storage/raft/snapshot` policy
- Deploy backup script to host at `/usr/local/bin/dream-ops-backup.sh`
- Deploy systemd service (`dream-ops-backup.service`) and timer (`dream-ops-backup.timer`)
- Enable timer: `systemctl enable --now dream-ops-backup.timer`
- Trigger manual run and verify encrypted files appear on dev-ubuntu-01

### 1.6 Configure DNS
- Configure Incus dnsmasq to serve `dream.lab` for VM hostnames (auto-registered as `<hostname>.dream.lab`)
- Add static service aliases in dnsmasq — service DNS names use no numbers:
  - `gitlab.dream.lab` → `gitlab-01`
  - `step-ca.dream.lab` → `step-ca-01`
  - `openbao.dream.lab` → `openbao-01`
- Verify both resolve: `gitlab-01.dream.lab` (VM hostname) and `gitlab.dream.lab` (service name)

---

## Phase 2 — Kubernetes cluster

**Tooling:** OpenTofu + talosctl

### 2.1 Prepare Talos configuration
- Generate Talos secrets: `talosctl gen secrets` → store in OpenBao
- Generate machine configs for control plane and workers
- Store machine configs in OpenBao

### 2.2 Provision Talos VMs
- Provision talos-cp-01 via OpenTofu (Talos ISO image)
- Provision talos-worker-01, talos-worker-gpu-01 via OpenTofu
- Apply machine configs via talosctl

### 2.3 Bootstrap cluster
- Run `talosctl bootstrap` on talos-cp-01
- Wait for control plane to be ready
- Generate kubeconfig → store in OpenBao
- Verify cluster: `kubectl get nodes`

---

## Phase 3 — Kubernetes core

**Tooling:** Helm (Cilium bootstrap) → ArgoCD (everything else)

Order is strict within this phase.

### 3.1 Install Cilium
- Install Cilium via Helm (before ArgoCD — CNI must exist first)
- Enable eBPF dataplane, kube-proxy replacement
- Verify all nodes Ready

### 3.2 Configure DNS
- Install CoreDNS with k8s_gateway plugin (Helm)
- Configure k8s_gateway to serve Gateway API resources as DNS records
- Create `CiliumLoadBalancerIPPool` reserving 10.10.0.53 for CoreDNS
- Create `CiliumL2AnnouncementPolicy` to advertise LoadBalancer IPs on incusbr0
- Expose CoreDNS via Cilium LoadBalancer at stable IP (10.10.0.53)
- Update Incus dnsmasq: forward `dream.lab` → 10.10.0.53
- Verify 10.10.0.53 is reachable from host: `dig @10.10.0.53 argocd.dream.lab`
- Verify pod DNS resolution for `dream.lab` and `cluster.local`

### 3.3 Install cert-manager
- Deploy cert-manager (Helm)
- Create ClusterIssuer pointing to step-ca-01 ACME endpoint
- Verify certificate issuance with a test Certificate resource

### 3.4 Bootstrap ArgoCD
- Deploy ArgoCD (Helm)
- Configure ArgoCD to authenticate with GitLab
- Create App-of-Apps root application pointing to infrastructure repo
- All subsequent deployments managed through ArgoCD

### 3.5 Configure Cilium Gateway API
- Enable Gateway API CRDs
- Create GatewayClass and default Gateway
- Verify platform service routing (test with a sample HTTPRoute)

---

## Phase 4 — Security and policy

**Tooling:** ArgoCD

### 4.1 External Secrets Operator
- Retrieve ESO AppRole credentials from OpenBao (role_id + secret_id for `k8s-app` policy)
- Create bootstrap K8s secret with AppRole credentials:
  `kubectl create secret generic openbao-approle --from-literal=role-id=<id> --from-literal=secret-id=<id> -n external-secrets`
- Deploy External Secrets Operator
- Create ClusterSecretStore pointing to openbao-01 using the bootstrap secret
- Verify secret sync with a test ExternalSecret

### 4.2 Kyverno
- Deploy Kyverno
- Apply baseline policies:
  - Require resource limits on all pods
  - Disallow privileged containers
  - Require labels (app, environment)
  - Disallow latest image tag

### 4.3 Tetragon
- Deploy Tetragon
- Configure TracingPolicies for process and network events
- Verify event stream via `tetra getevents`

### 4.4 Trivy
- Add Trivy scanning stage to GitLab CI pipeline template
- Configure to fail on Critical/High CVEs
- Integrate scan results with GitLab Security Dashboard

---

## Phase 5 — Observability

**Tooling:** ArgoCD

### 5.1 MinIO
- Deploy MinIO (standalone mode)
- Create buckets: loki, tempo, spark, data
- Configure lifecycle policies

### 5.2 kube-prometheus-stack
- Deploy Prometheus + Alertmanager + Grafana
- Configure scraping for all platform components
- Import baseline K8s dashboards

### 5.3 Loki
- Deploy Loki
- Deploy Promtail / OTel Collector as log forwarder on each node
- Configure MinIO as Loki storage backend

### 5.4 Tempo
- Deploy Tempo
- Configure MinIO as Tempo storage backend

### 5.5 OpenTelemetry Collector
- Deploy OTel Collector as DaemonSet
- Configure pipelines: metrics → Prometheus, logs → Loki, traces → Tempo

### 5.6 Hubble
- Enable Hubble UI (included with Cilium)
- Expose via Gateway API
- Verify network flow visibility

### 5.7 Grafana datasources
- Configure Prometheus, Loki, Tempo datasources
- Create unified dashboards for platform health

---

## Phase 6 — Data platform

**Tooling:** ArgoCD

### 6.1 CloudNativePG
- Deploy CloudNativePG operator
- Create initial PostgreSQL cluster (3-instance HA)
- Configure backups to MinIO

### 6.2 ClickHouse
- Deploy Altinity clickhouse-operator
- Create initial ClickHouse cluster
- Configure backups to MinIO

### 6.3 Strimzi (Kafka)
- Deploy Strimzi operator
- Create Kafka cluster (3 brokers, KRaft mode)
- Create initial topics

### 6.4 Spark Operator
- Deploy Spark Operator
- Run test SparkApplication to verify cluster connectivity

---

## Phase 7 — GPU workloads

**Tooling:** Ansible (host) + OpenTofu (VM) + ArgoCD (operator)

### 7.1 Configure PCI passthrough on host
- Enable IOMMU in GRUB (intel_iommu=on)
- Bind RTX 3070 Ti to vfio-pci driver (Ansible)
- Verify GPU in VFIO group: `ls /sys/bus/pci/drivers/vfio-pci`

### 7.2 Provision talos-worker-gpu-01
- Provision VM via OpenTofu with GPU PCI passthrough config
- Apply Talos machine config
- Add node to cluster, verify it joins

### 7.3 NVIDIA GPU Operator
- Deploy NVIDIA GPU Operator via ArgoCD
- Verify GPU resource available: `kubectl describe node talos-worker-gpu-01`
- Run test GPU workload (CUDA vector-add)

---

## Phase 8 — Research and ideas

> This phase is not a planned implementation sequence. It is a backlog of tools and
> practices worth exploring when resources and motivation allow. Items may be
> adopted, dropped, or reprioritized as the platform evolves.

### 8.1 AmneziaWG (alternative remote access)
- Install AmneziaWG on host and dev-ubuntu-01
- Configure obfuscated WireGuard tunnel
- Test connectivity through RU ISP DPI

### 8.2 Image signing (supply chain security)
- Configure Cosign signing in GitLab CI (sign on push)
- Add Kyverno policy to require signed images in production namespaces
- Verify policy blocks unsigned images

### 8.3 Additional hardening
- Apply CIS Kubernetes Benchmark policies via Kyverno
- Configure OpenBao audit logging
- Configure GitLab audit events → Loki
- Review and tighten Tetragon TracingPolicies

### 8.4 SonarQube (SAST and code quality)
- Deploy SonarQube Community Edition
- Integrate with GitLab CI (sonar-scanner stage in pipeline templates)
- Configure quality gates to block merge on critical issues
- Enable secrets detection and security hotspot review

### 8.5 Dependency-Track (SCA and SBOM)
- Deploy Dependency-Track (API server + frontend)
- Generate SBOM on every build (Syft or CycloneDX Gradle/Maven plugin)
- Push SBOM to Dependency-Track from GitLab CI
- Configure vulnerability feed (NVD, OSV) and alert thresholds

### 8.6 Keycloak (SSO and OIDC)
- Deploy Keycloak
- Configure OIDC integration for: GitLab, ArgoCD, Grafana, OpenBao
- Set up roles and groups mirroring platform access model
- Enable MFA for admin accounts

### 8.7 Velero (cluster backup and restore)
- Deploy Velero with MinIO as S3 backend
- Configure scheduled backups (cluster state, PVCs)
- Document and test restore procedure
- Verify backup integrity with a test restore to a vCluster

### 8.8 Chaos Mesh (chaos engineering)
- Deploy Chaos Mesh
- Define baseline SLOs for platform services (Prometheus rules)
- Run initial chaos experiments: pod kill, network partition, disk pressure
- Verify observability stack captures events correctly

### 8.9 Backstage (internal developer portal)
- Deploy Backstage
- Integrate with GitLab as catalog source (GitLab discovery plugin)
- Register all platform services in the Software Catalog
- Add TechDocs integration for documentation-as-code

### 8.10 Kubescape (K8s security posture)
- Deploy Kubescape operator
- Run NSA/CISA and CIS benchmark scans
- Configure continuous scanning and trend reporting
- Expose results in Grafana dashboard
