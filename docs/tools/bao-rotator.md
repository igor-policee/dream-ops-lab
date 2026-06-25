# bao-rotator

Secret rotation CLI for OpenBao / Vault. Supports listing, rotating, and auditing
KV v2 secrets with configurable rotation policy enforcement.

## Commands

### `list <mount>`

List all secret keys under a KV v2 mount path.

```bash
bao-rotator list kv
```

### `rotate <mount> <path>`

Rotate a secret at the given path. Generates a cryptographically random 32-byte
value (base64url-encoded) and writes it as the new secret version. The previous
version is retained by Vault's versioning mechanism.

```bash
bao-rotator rotate kv gitlab/runner-token
```

After rotation, the old value remains accessible as a previous version in Vault
until explicitly destroyed.

### `audit <mount>`

Check all secrets under a mount for rotation policy compliance. Reports secrets
90 days old or older as needing rotation.

```bash
bao-rotator audit kv
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `VAULT_ADDR` | OpenBao/Vault address (e.g. `https://openbao.dream.lab:8200`) |
| `VAULT_TOKEN` | Token with KV read/write and metadata access |

Both can be overridden with `--addr` and `--token` flags.

## Rotation Policy

| Threshold | Behavior |
|-----------|----------|
| < 90 days | `audit`: reports OK |
| ≥ 90 days | `audit`: reports WARN — needs rotation |

`audit` traverses the KV v2 path tree recursively; keys at all nesting levels
(e.g. `gitlab/runner-token`) are included in the report.

## Build

```bash
cd tools/bao-rotator
go build -o bao-rotator ./cmd
```

## CronJob Deployment

Runs weekly on Sundays at 02:00 to audit secrets for stale rotation:

```bash
kubectl apply -f k8s/tools/bao-rotator-cronjob.yaml
```

Required secret:

```bash
kubectl create secret generic bao-rotator-vault-token \
  --from-literal=token=<VAULT_TOKEN> \
  -n security
```

The token requires KV v2 metadata read and data write permissions:

```hcl
path "kv/metadata/*" {
  capabilities = ["read", "list"]
}
path "kv/data/*" {
  capabilities = ["create", "update"]
}
```

## Notes

- `rotate` generates a random value. For secrets where the value is externally
  imposed (e.g. a third-party API key), rotate manually and update Vault directly.
- Version history is preserved in Vault. Destroying old versions requires an
  explicit `vault kv destroy` or equivalent API call.
- The audit command checks `created_time` of the current version from KV metadata.
