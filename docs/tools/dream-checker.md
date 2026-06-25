# dream-checker

Security posture checker for the dream-ops-lab platform. Runs structured checks
against Kubernetes, OpenBao, PKI (step-ca), and supply-chain artefacts.

## Check Modules

| Module | ID         | Description                                        |
| ------ | ---------- | -------------------------------------------------- |
| k8s    | K8S-001    | No privileged containers                           |
| k8s    | K8S-002    | No hostNetwork pods                                |
| k8s    | K8S-003    | No containers running as root                      |
| k8s    | K8S-004    | All containers have resource limits                |
| k8s    | K8S-005    | Default ServiceAccount not automounted             |
| k8s    | K8S-006    | NetworkPolicy present per namespace                |
| k8s    | K8S-007    | Pod security admission labels set                  |
| vault  | VAULT-001  | Vault/Bao is unsealed                              |
| vault  | VAULT-002  | Root token revoked                                 |
| vault  | VAULT-003  | Audit device enabled                               |
| vault  | VAULT-004  | Required secret engines mounted (kv/, pki/)        |
| vault  | VAULT-005  | Token TTL bounded                                  |
| pki    | PKI-001    | step-ca ACME endpoint reachable                    |
| pki    | PKI-002    | No expired or near-expiry TLS certificates         |
| pki    | PKI-003    | TLS secrets managed by cert-manager                |
| pki    | PKI-004    | CRL endpoint available                             |
| supply | SUPPLY-001 | gitleaks available in PATH                         |
| supply | SUPPLY-002 | trivy available in PATH                            |
| supply | SUPPLY-003 | Non-system images have cosign signature annotation |
| supply | SUPPLY-004 | Non-system pods have SBOM annotation               |
| supply | SUPPLY-005 | No mutable image tags (:latest or untagged)        |

## Status Values

- `PASS` — check passed
- `WARN` — potential issue; does not cause non-zero exit
- `FAIL` — check failed; causes exit code 1
- `SKIP` — check skipped (missing configuration or unreachable dependency)

## Usage

```bash
# Run all modules
dream-checker all

# Run a specific module
dream-checker k8s --namespace production
dream-checker vault
dream-checker pki
dream-checker supply

# JSON output (for GitLab CI artifact or Loki ingestion)
dream-checker all --output json

# Check all namespaces
dream-checker k8s --all-namespaces
```

## Environment Variables

| Variable       | Description                                                         |
| -------------- | ------------------------------------------------------------------- |
| `VAULT_ADDR`   | OpenBao/Vault address (e.g. `https://openbao.dream.lab:8200`)       |
| `VAULT_TOKEN`  | Token for Vault API access                                          |
| `STEP_CA_ADDR` | step-ca host:port or full URL (e.g. `step-ca.dream.lab:9000`)       |
| `KUBECONFIG`   | Path to kubeconfig; not needed when running in-cluster as a CronJob |

## Build

```bash
cd tools/dream-checker
go build -o dream-checker .
```

## Test

```bash
cd tools/dream-checker
go test ./...
```

## CronJob Deployment

The tool runs as a Kubernetes CronJob every hour:

```bash
kubectl apply -f k8s/tools/dream-checker-cronjob.yaml
```

Required secrets (create before deploying):

```bash
# Vault token secret
kubectl create secret generic dream-checker-vault-token \
  --from-literal=token=<VAULT_TOKEN> \
  -n security

# The CronJob uses in-cluster ServiceAccount — no kubeconfig secret needed
```

## GitLab CI Integration

```yaml
dream-checker:
  stage: security
  image: registry.dream.lab/dream-checker:v0.1.0
  script:
    - dream-checker all --output json > dream-checker-report.json
  artifacts:
    paths:
      - dream-checker-report.json
    expire_in: 7 days
  allow_failure: false
```

## Exit Codes

| Code | Meaning                 |
| ---- | ----------------------- |
| 0    | All checks PASS or WARN |
| 1    | One or more checks FAIL |
