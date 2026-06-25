package vault

import (
	"fmt"
	"time"

	"github.com/igor-policee/dream-ops-lab/tools/dream-checker/internal/report"
	vaultapi "github.com/hashicorp/vault/api"
)

func newClient(addr, token string) (*vaultapi.Client, error) {
	cfg := vaultapi.DefaultConfig()
	if addr != "" {
		cfg.Address = addr
	}
	client, err := vaultapi.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("vault client: %w", err)
	}
	if token != "" {
		client.SetToken(token)
	}
	return client, nil
}

func RunChecks(addr, token string) ([]report.CheckResult, error) {
	client, err := newClient(addr, token)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var results []report.CheckResult

	results = append(results, checkVaultSealed(client, now))
	results = append(results, checkRootTokenRevoked(client, now))
	results = append(results, checkAuditEnabled(client, now))
	results = append(results, checkSecretEnginesEnabled(client, now))
	results = append(results, checkTokenTTL(client, now))

	return results, nil
}

// VAULT-001: Vault/Bao is unsealed
func checkVaultSealed(client *vaultapi.Client, now time.Time) report.CheckResult {
	r := report.CheckResult{ID: "VAULT-001", Name: "Vault unsealed", CheckedAt: now}

	health, err := client.Sys().Health()
	if err != nil {
		r.Status = report.StatusSkip
		r.Message = fmt.Sprintf("health check failed: %v", err)
		return r
	}

	if health.Sealed {
		r.Status = report.StatusFail
		r.Message = "Vault is sealed"
	} else {
		r.Status = report.StatusPass
		r.Message = fmt.Sprintf("Vault unsealed, version=%s", health.Version)
	}
	return r
}

// VAULT-002: Root token is not active (should be revoked after setup)
func checkRootTokenRevoked(client *vaultapi.Client, now time.Time) report.CheckResult {
	r := report.CheckResult{ID: "VAULT-002", Name: "Root token revoked", CheckedAt: now}

	secret, err := client.Auth().Token().LookupSelf()
	if err != nil {
		r.Status = report.StatusSkip
		r.Message = fmt.Sprintf("token lookup failed: %v", err)
		return r
	}

	if secret == nil {
		r.Status = report.StatusPass
		r.Message = "Token not found (expected)"
		return r
	}

	policies, ok := secret.Data["policies"].([]interface{})
	if !ok {
		r.Status = report.StatusPass
		r.Message = "Non-root token in use"
		return r
	}

	for _, p := range policies {
		if p == "root" {
			r.Status = report.StatusWarn
			r.Message = "Root token is still active — revoke after setup"
			return r
		}
	}

	r.Status = report.StatusPass
	r.Message = "Non-root token in use"
	return r
}

// VAULT-003: At least one audit device is enabled
func checkAuditEnabled(client *vaultapi.Client, now time.Time) report.CheckResult {
	r := report.CheckResult{ID: "VAULT-003", Name: "Audit device enabled", CheckedAt: now}

	audits, err := client.Sys().ListAudit()
	if err != nil {
		r.Status = report.StatusSkip
		r.Message = fmt.Sprintf("list audit failed: %v", err)
		return r
	}

	if len(audits) == 0 {
		r.Status = report.StatusFail
		r.Message = "No audit devices enabled"
	} else {
		r.Status = report.StatusPass
		r.Message = fmt.Sprintf("%d audit device(s) enabled", len(audits))
		for path := range audits {
			r.Details = append(r.Details, path)
		}
	}
	return r
}

// VAULT-004: Required secret engines are mounted (kv-v2, pki)
func checkSecretEnginesEnabled(client *vaultapi.Client, now time.Time) report.CheckResult {
	r := report.CheckResult{ID: "VAULT-004", Name: "Required secret engines mounted", CheckedAt: now}

	mounts, err := client.Sys().ListMounts()
	if err != nil {
		r.Status = report.StatusSkip
		r.Message = fmt.Sprintf("list mounts failed: %v", err)
		return r
	}

	required := map[string]bool{
		"kv/":  false,
		"pki/": false,
	}

	for path := range mounts {
		for req := range required {
			if path == req {
				required[req] = true
			}
		}
	}

	var missing []string
	for path, found := range required {
		if !found {
			missing = append(missing, path)
		}
	}

	if len(missing) == 0 {
		r.Status = report.StatusPass
		r.Message = "All required secret engines are mounted"
	} else {
		r.Status = report.StatusFail
		r.Message = fmt.Sprintf("Missing secret engines: %v", missing)
		r.Details = missing
	}
	return r
}

// VAULT-005: Current token has a TTL (not infinite)
func checkTokenTTL(client *vaultapi.Client, now time.Time) report.CheckResult {
	r := report.CheckResult{ID: "VAULT-005", Name: "Token TTL bounded", CheckedAt: now}

	secret, err := client.Auth().Token().LookupSelf()
	if err != nil {
		r.Status = report.StatusSkip
		r.Message = fmt.Sprintf("token lookup failed: %v", err)
		return r
	}

	if secret == nil {
		r.Status = report.StatusSkip
		r.Message = "No token info available"
		return r
	}

	ttl, _ := secret.Data["ttl"].(float64)
	if ttl == 0 {
		r.Status = report.StatusWarn
		r.Message = "Token has no TTL (infinite) — use short-lived tokens in production"
	} else {
		r.Status = report.StatusPass
		r.Message = fmt.Sprintf("Token TTL: %.0f seconds", ttl)
	}
	return r
}
