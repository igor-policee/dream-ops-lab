# Network Diagram

## Physical and VM topology

```
Physical Host (homelab-ubuntu)
│
│  wlp5s0 (WiFi, 192.168.1.100/24)
│    └── NAT → Router → Internet
│
└── incusbr0 (10.10.0.0/24)
      │
      ├── 10.10.0.1    host bridge IP / Incus dnsmasq
      ├── 10.10.0.10   step-ca-01
      ├── 10.10.0.11   openbao-01
      ├── 10.10.0.12   gitlab-01
      ├── 10.10.0.20   talos-cp-01
      ├── 10.10.0.30   talos-worker-01
      ├── 10.10.0.31   talos-worker-02
      ├── 10.10.0.40   talos-worker-gpu-01
      │
      └── 10.10.0.53   CoreDNS (Cilium LoadBalancer IP, stable)
```

> IP assignments are illustrative. Final allocation decided in the Talos layer.

---

## Remote access

```
Internet
  └── VPS (public IP)
        ← autossh reverse tunnel (from host, persistent via systemd)
              └── Physical Host :22
                    └── incusbr0 (all VMs reachable from host)
```

Access pattern: SSH to host via VPS, then use kubectl / talosctl / incus / curl
directly from the host. Ad-hoc port forwarding for browser access to platform UIs.

---

## DNS resolution flow

### From the host or any VM (non-K8s)

```
Client (host or VM)
  └── resolv.conf: 10.10.0.1
        │
        ├── step-ca-01.dream.lab?   → Incus dnsmasq knows → 10.10.0.10
        ├── openbao-01.dream.lab?  → Incus dnsmasq knows → 10.10.0.11
        ├── gitlab-01.dream.lab?   → Incus dnsmasq knows → 10.10.0.12
        ├── talos-cp-01.dream.lab?  → Incus dnsmasq knows → 10.10.0.20
        │
        ├── argocd.dream.lab?      → dnsmasq forwards → CoreDNS (10.10.0.53)
        │     └── k8s_gateway      → looks up Gateway resource → LB IP
        │
        └── google.com?            → dnsmasq forwards → router → internet
```

### From a pod inside Kubernetes

```
Pod
  └── resolv.conf: CoreDNS (cluster IP, e.g. 10.96.0.10)
        │
        ├── svc.cluster.local?     → CoreDNS handles natively (K8s service discovery)
        │
        ├── argocd.dream.lab?      → k8s_gateway → Gateway LB IP
        ├── grafana.dream.lab?     → k8s_gateway → Gateway LB IP
        │
        ├── gitlab-01.dream.lab?   → CoreDNS forwards → Incus dnsmasq (10.10.0.1)
        │     └── dnsmasq knows   → 10.10.0.12
        │
        └── google.com?            → CoreDNS forwards upstream → internet
```

---

## DNS server responsibilities

| Server | Address | Authoritative for | Forwards to |
|--------|---------|-------------------|-------------|
| Incus dnsmasq | 10.10.0.1 | VM hostnames in `dream.lab` | CoreDNS (dream.lab platform names), router (internet) |
| CoreDNS + k8s_gateway | 10.10.0.53 | Platform service names in `dream.lab` | Incus dnsmasq (VM names), upstream (internet) |

---

## PKI and certificate flow

```
step-ca-01 (10.10.0.10)
  └── Root CA for dream.lab
        │
        ├── cert-manager (in K8s)
        │     └── ACME → step-ca-01
        │           └── issues certs for platform UIs (*.dream.lab)
        │
        ├── openbao-01
        │     └── ACME → step-ca-01 → openbao-01.dream.lab cert
        │
        ├── gitlab-01
        │     └── ACME → step-ca-01 → gitlab-01.dream.lab cert
        │
        └── Talos nodes (if TLS required)
              └── ACME → step-ca-01
```

All clients (browser, kubectl, curl) must trust the step-ca root certificate.
The root cert is added to the trust store once per client machine.

---

## Platform service DNS names (examples)

| Service | DNS name | Resolved by |
|---------|----------|-------------|
| GitLab | gitlab-01.dream.lab | Incus dnsmasq |
| OpenBao | openbao-01.dream.lab | Incus dnsmasq |
| ArgoCD | argocd.dream.lab | CoreDNS / k8s_gateway |
| Grafana | grafana.dream.lab | CoreDNS / k8s_gateway |
| step-ca | step-ca-01.dream.lab | Incus dnsmasq |
| K8s API | talos-cp-01.dream.lab | Incus dnsmasq |
