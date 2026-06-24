# Runbooks

Operational procedures for the platform. See [architecture.md](architecture.md) for
system context and [decisions.md](decisions.md) for rationale behind each choice.

---

## Backup and Recovery

### Overview

Critical data is backed up from the physical host to dev-ubuntu-01 automatically.
The physical host is frequently off; backups are triggered by system events, not
by a fixed schedule.

```
Physical Host (when running)
  ├── step-ca-01 VM  → /etc/step-ca tarball
  ├── openbao-01 VM  → Raft snapshot (only when unsealed)
  └── OpenTofu state → terraform.tfstate (bootstrap phase only)
        ↓ encrypted with age (asymmetric, public key on host)
dev-ubuntu-01 ~/backups/dream-ops-lab/
  ├── step-ca/    ← .tar.gz.age files, last 3 retained
  ├── openbao/    ← .snap.age files, last 3 retained
  ├── gitlab/     ← .tar.gz.age files, last 3 retained
  └── tfstate/    ← .tfstate.age files, last 3 retained

Bitwarden (recovery credentials, stored once, do not change)
  ├── "OpenBao unseal keys"          ← 5 Shamir shards (threshold 3-of-5)
  ├── "step-ca CA password"          ← encrypts CA private keys inside the VM
  └── "dream-ops-lab age backup key" ← age private key for decrypting VPS backups
```

**Trigger model:** backup runs on host startup (5 min after boot) and every hour
while the host is running. No fixed wall-clock schedule — the host is mostly off.

---

### Initial Setup (split across phases)

**Phase 1.1 — On the host (before first `tofu apply`):**

```bash
# Install age
apt-get install -y age

# Generate key pair — private key is written to file only temporarily
age-keygen -o /root/.age-backup.key   # prints the public key to stdout
chmod 400 /root/.age-backup.key
```

Copy the printed public key (starts with `age1...`) — it goes into the backup script
(public key is not a secret).

**Immediately** copy the private key content to Bitwarden as secure note
"dream-ops-lab age backup key", then shred the local file:

```bash
shred -u /root/.age-backup.key
```

The private key must not persist on the host. Only the public key (embedded in the
backup script) lives on the host — it cannot be used to decrypt.

**Phase 1.1 — On dev-ubuntu-01:**

```bash
mkdir -p ~/backups/dream-ops-lab/tfstate
```

**Phase 1.5 — On dev-ubuntu-01 (after state migrated to GitLab in Phase 1.4):**

```bash
mkdir -p ~/backups/dream-ops-lab/{step-ca,openbao,gitlab}
```

---

### Backup Script

Deploy to `/usr/local/bin/dream-ops-backup.sh` (mode 0700, owner root):

```bash
#!/usr/bin/env bash
set -euo pipefail

BACKUP_HOST="dev-ubuntu-01"
BACKUP_BASE="~/backups/dream-ops-lab"
# Public key — not a secret, safe to hardcode
AGE_PUBKEY="age1..."   # replace with actual public key from age-keygen output
DATE=$(date +%Y%m%d-%H%M%S)
TMPDIR=$(mktemp -d)
trap 'rm -rf "${TMPDIR}"' EXIT

log() { echo "[$(date -Iseconds)] $*"; }

# --- step-ca ---
if incus exec step-ca-01 -- test -d /etc/step-ca 2>/dev/null; then
    log "Backing up step-ca-01..."
    incus file pull --recursive step-ca-01/etc/step-ca "${TMPDIR}/step-ca"
    tar -czf "${TMPDIR}/step-ca.tar.gz" -C "${TMPDIR}" step-ca
    age -r "${AGE_PUBKEY}" -o "${TMPDIR}/step-ca-${DATE}.tar.gz.age" \
        "${TMPDIR}/step-ca.tar.gz"
    rsync "${TMPDIR}/step-ca-${DATE}.tar.gz.age" \
        "${BACKUP_HOST}:${BACKUP_BASE}/step-ca/"
    log "step-ca backup done."
else
    log "step-ca-01 not running — skipping."
fi

# --- OpenBao ---
OPENBAO_ADDR="http://10.10.0.11:8200"
OPENBAO_TOKEN_FILE="/root/.openbao-backup-token"
if curl -sf "${OPENBAO_ADDR}/v1/sys/health" | grep -q '"sealed":false'; then
    log "Backing up OpenBao..."
    VAULT_ADDR="${OPENBAO_ADDR}" VAULT_TOKEN="$(cat ${OPENBAO_TOKEN_FILE})" \
        bao operator raft snapshot save "${TMPDIR}/openbao-snapshot.snap"
    age -r "${AGE_PUBKEY}" -o "${TMPDIR}/openbao-${DATE}.snap.age" \
        "${TMPDIR}/openbao-snapshot.snap"
    rsync "${TMPDIR}/openbao-${DATE}.snap.age" \
        "${BACKUP_HOST}:${BACKUP_BASE}/openbao/"
    log "OpenBao backup done."
else
    log "OpenBao is sealed or unreachable — skipping snapshot."
fi

# --- GitLab ---
if incus exec gitlab-01 -- test -f /etc/gitlab/gitlab.rb 2>/dev/null; then
    log "Backing up GitLab..."
    incus exec gitlab-01 -- gitlab-backup create STRATEGY=copy 2>/dev/null
    GITLAB_BACKUP=$(incus exec gitlab-01 -- \
        ls -t /var/opt/gitlab/backups/*.tar 2>/dev/null | head -1)
    if [ -n "${GITLAB_BACKUP}" ]; then
        GITLAB_BACKUP_FILE=$(basename "${GITLAB_BACKUP}")
        incus file pull "gitlab-01${GITLAB_BACKUP}" "${TMPDIR}/${GITLAB_BACKUP_FILE}"
        incus file pull gitlab-01/etc/gitlab/gitlab-secrets.json \
            "${TMPDIR}/gitlab-secrets.json"
        incus file pull gitlab-01/etc/gitlab/gitlab.rb "${TMPDIR}/gitlab.rb"
        tar -czf "${TMPDIR}/gitlab-bundle.tar.gz" -C "${TMPDIR}" \
            "${GITLAB_BACKUP_FILE}" gitlab-secrets.json gitlab.rb
        age -r "${AGE_PUBKEY}" -o "${TMPDIR}/gitlab-${DATE}.tar.gz.age" \
            "${TMPDIR}/gitlab-bundle.tar.gz"
        rsync "${TMPDIR}/gitlab-${DATE}.tar.gz.age" \
            "${BACKUP_HOST}:${BACKUP_BASE}/gitlab/"
        incus exec gitlab-01 -- rm -f "${GITLAB_BACKUP}"
        log "GitLab backup done."
    fi
else
    log "gitlab-01 not running — skipping."
fi

# --- OpenTofu state (bootstrap phase only) ---
TFSTATE_PATH="/opt/infra/terraform.tfstate"
if [ -f "${TFSTATE_PATH}" ]; then
    log "Backing up terraform.tfstate..."
    age -r "${AGE_PUBKEY}" -o "${TMPDIR}/terraform-${DATE}.tfstate.age" \
        "${TFSTATE_PATH}"
    rsync "${TMPDIR}/terraform-${DATE}.tfstate.age" \
        "${BACKUP_HOST}:${BACKUP_BASE}/tfstate/"
    log "tfstate backup done."
fi

# --- Retention on dev-ubuntu-01 (keep 3 most recent per type) ---
ssh "${BACKUP_HOST}" \
    "ls -t ${BACKUP_BASE}/step-ca/*.age 2>/dev/null | tail -n +4 | xargs -r rm -f"
ssh "${BACKUP_HOST}" \
    "ls -t ${BACKUP_BASE}/openbao/*.age 2>/dev/null | tail -n +4 | xargs -r rm -f"
ssh "${BACKUP_HOST}" \
    "ls -t ${BACKUP_BASE}/gitlab/*.age 2>/dev/null | tail -n +4 | xargs -r rm -f"
ssh "${BACKUP_HOST}" \
    "ls -t ${BACKUP_BASE}/tfstate/*.age 2>/dev/null | tail -n +4 | xargs -r rm -f"

log "Backup complete."
```

---

### Systemd Service and Timer

**`/etc/systemd/system/dream-ops-backup.service`:**

```ini
[Unit]
Description=dream-ops-lab off-site backup
After=network-online.target incus.service
Wants=network-online.target

[Service]
Type=oneshot
ExecStart=/usr/local/bin/dream-ops-backup.sh
User=root
StandardOutput=journal
StandardError=journal
```

**`/etc/systemd/system/dream-ops-backup.timer`:**

```ini
[Unit]
Description=dream-ops-lab backup — on boot and hourly

[Timer]
OnBootSec=5min
OnUnitActiveSec=1h

[Install]
WantedBy=timers.target
```

Enable:

```bash
systemctl daemon-reload
systemctl enable --now dream-ops-backup.timer
```

Check last run: `journalctl -u dream-ops-backup.service -n 50`

Trigger manually: `systemctl start dream-ops-backup.service`

---

### OpenBao — Initialization and Unseal Key Management

OpenBao is initialized once after first boot of openbao-01. Key shards and the
root token are shown in plaintext only at this moment.

```bash
# On openbao-01
bao operator init -key-shares=5 -key-threshold=3
```

**Immediately after init:**

1. Save the 5 unseal key shards and root token to a temporary file.
2. Store them in Bitwarden as secure note "OpenBao unseal keys".
3. Shred the local file: `shred -u openbao-init.txt`
4. Revoke the root token after initial setup: `bao token revoke <root-token>`

Create a dedicated backup token (used by the backup script):

```bash
bao policy write backup - <<'EOF'
path "sys/storage/raft/snapshot" { capabilities = ["read"] }
EOF
bao token create -policy=backup -period=768h -display-name=backup \
    | grep "^token " | awk '{print $2}' > /root/.openbao-backup-token
chmod 400 /root/.openbao-backup-token
```

**Unsealing after reboot** (OpenBao seals on every restart):

```bash
bao operator unseal <shard-1>
bao operator unseal <shard-2>
bao operator unseal <shard-3>
# Status check:
bao status
```

Retrieve shards from Bitwarden when needed.

---

### Recovery Procedures

#### Decrypt a backup file

```bash
# Retrieve age private key from Bitwarden → save to /tmp/age-backup.key
age -d -i /tmp/age-backup.key -o output.tar.gz input.tar.gz.age
shred -u /tmp/age-backup.key
```

#### Restore step-ca from backup

```bash
# Decrypt
age -d -i /tmp/age-backup.key -o step-ca.tar.gz step-ca-YYYYMMDD.tar.gz.age

# Push into a freshly provisioned step-ca-01 VM
tar -xzf step-ca.tar.gz
incus file push --recursive step-ca/ step-ca-01/etc/step-ca/
incus exec step-ca-01 -- chown -R step:step /etc/step-ca
incus exec step-ca-01 -- systemctl restart step-ca

# Verify
incus exec step-ca-01 -- step ca health
```

#### Restore OpenBao from snapshot

```bash
# Decrypt
age -d -i /tmp/age-backup.key -o openbao-snapshot.snap openbao-YYYYMMDD.snap.age

# Restore — OpenBao must be initialized but can be sealed
bao operator raft snapshot restore openbao-snapshot.snap

# Unseal with shards from Bitwarden
bao operator unseal <shard-1>
bao operator unseal <shard-2>
bao operator unseal <shard-3>
```

#### Restore OpenTofu state (bootstrap phase)

```bash
age -d -i /tmp/age-backup.key -o terraform.tfstate terraform-YYYYMMDD.tfstate.age
# Place at the expected path and run: tofu init && tofu plan
```

#### Restore GitLab from backup

```bash
# Decrypt the bundle
age -d -i /tmp/age-backup.key -o gitlab-bundle.tar.gz gitlab-YYYYMMDD.tar.gz.age

# Unpack
mkdir -p /tmp/gitlab-restore
tar -xzf gitlab-bundle.tar.gz -C /tmp/gitlab-restore

# Derive backup prefix — GitLab restore expects BACKUP=<prefix> where the file
# on disk is named <prefix>_gitlab_backup.tar
BACKUP_FILE=$(ls /tmp/gitlab-restore/*_gitlab_backup.tar 2>/dev/null | head -1)
BACKUP_PREFIX=$(basename "${BACKUP_FILE}" _gitlab_backup.tar)

# Push data backup into a freshly provisioned gitlab-01 VM (keep original filename)
incus file push "${BACKUP_FILE}" \
    "gitlab-01/var/opt/gitlab/backups/$(basename "${BACKUP_FILE}")"
incus exec gitlab-01 -- chown git:git \
    "/var/opt/gitlab/backups/$(basename "${BACKUP_FILE}")"

# Restore config files (required before running gitlab-restore)
incus file push /tmp/gitlab-restore/gitlab-secrets.json \
    gitlab-01/etc/gitlab/gitlab-secrets.json
incus file push /tmp/gitlab-restore/gitlab.rb \
    gitlab-01/etc/gitlab/gitlab.rb

# Run restore (GitLab service must be stopped first)
incus exec gitlab-01 -- gitlab-ctl stop puma
incus exec gitlab-01 -- gitlab-ctl stop sidekiq
incus exec gitlab-01 -- gitlab-backup restore "BACKUP=${BACKUP_PREFIX}"
incus exec gitlab-01 -- gitlab-ctl reconfigure
incus exec gitlab-01 -- gitlab-ctl start
incus exec gitlab-01 -- gitlab-rake gitlab:check SANITIZE=true
```

---

### Recovery Priority Order

Restore components in this sequence after a catastrophic host failure:

| Order | Component | Reason |
|-------|-----------|--------|
| 1 | step-ca-01 | All TLS certificates depend on it; must exist before anything requests a cert |
| 2 | openbao-01 | Holds Talos configs, kubeconfig, tokens required to rebuild K8s |
| 3 | gitlab-01 | GitOps source; cluster self-heals once ArgoCD reconnects |
| 4 | Kubernetes nodes | Re-provision with `talosctl` using configs retrieved from OpenBao |

---

## Incus — VM Snapshots

ZFS-backed Incus provides instant VM snapshots. Use for pre-change checkpoints
and single-VM rollbacks.

```bash
# Create
incus snapshot create <vm-name> <snapshot-name>

# List
incus snapshot list <vm-name>

# Restore (VM must be stopped)
incus stop <vm-name>
incus snapshot restore <vm-name> <snapshot-name>
incus start <vm-name>
```

Automated daily snapshots for critical VMs:

```bash
for vm in step-ca-01 openbao-01; do
    incus config set $vm snapshots.schedule "0 2 * * *"
    incus config set $vm snapshots.expiry 3d
done
```
