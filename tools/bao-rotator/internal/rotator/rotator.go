package rotator

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	vaultapi "github.com/hashicorp/vault/api"
)

const (
	rotationThresholdDays = 90
	randomBytesLen        = 32
)

type Rotator struct {
	client *vaultapi.Client
}

type RotateResult struct {
	Path    string
	Version int
}

type AuditEntry struct {
	Path          string
	Version       int
	AgeDays       int
	NeedsRotation bool
	CreatedAt     time.Time
}

func New(addr, token string) (*Rotator, error) {
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
	return &Rotator{client: client}, nil
}

// List returns all secret paths recursively under the given KV v2 mount path.
// Keys ending in "/" in the Vault response are directory markers and are traversed recursively.
func (r *Rotator) List(mount string) ([]string, error) {
	return r.listRecursive(mount, "")
}

func (r *Rotator) listRecursive(mount, prefix string) ([]string, error) {
	listPath := fmt.Sprintf("%s/metadata/%s", mount, prefix)

	secret, err := r.client.Logical().List(listPath)
	if err != nil {
		return nil, fmt.Errorf("list %s: %w", listPath, err)
	}
	if secret == nil || secret.Data == nil {
		return nil, nil
	}

	rawKeys, ok := secret.Data["keys"].([]interface{})
	if !ok {
		return nil, nil
	}

	var paths []string
	for _, k := range rawKeys {
		s, ok := k.(string)
		if !ok {
			continue
		}
		if strings.HasSuffix(s, "/") {
			sub, err := r.listRecursive(mount, prefix+s)
			if err != nil {
				return nil, err
			}
			paths = append(paths, sub...)
		} else {
			paths = append(paths, prefix+s)
		}
	}
	return paths, nil
}

// Rotate generates a new random value for the secret at mount/path and writes it.
func (r *Rotator) Rotate(mount, path string) (*RotateResult, error) {
	newValue, err := generateSecret()
	if err != nil {
		return nil, fmt.Errorf("generate secret: %w", err)
	}

	writePath := fmt.Sprintf("%s/data/%s", mount, path)
	secret, err := r.client.Logical().Write(writePath, map[string]interface{}{
		"data": map[string]interface{}{
			"value":      newValue,
			"rotated_at": time.Now().UTC().Format(time.RFC3339),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("write secret %s: %w", writePath, err)
	}

	version := 0
	if secret != nil && secret.Data != nil {
		if meta, ok := secret.Data["version"].(float64); ok {
			version = int(meta)
		}
	}

	return &RotateResult{Path: path, Version: version}, nil
}

// Audit checks all secrets under the mount (recursively) for rotation policy compliance.
func (r *Rotator) Audit(mount string) ([]AuditEntry, error) {
	keys, err := r.List(mount)
	if err != nil {
		return nil, err
	}

	var entries []AuditEntry
	for _, key := range keys {
		entry, err := r.auditSecret(mount, key)
		if err != nil {
			entry = AuditEntry{Path: key, NeedsRotation: true}
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (r *Rotator) auditSecret(mount, path string) (AuditEntry, error) {
	metaPath := fmt.Sprintf("%s/metadata/%s", mount, path)
	secret, err := r.client.Logical().Read(metaPath)
	if err != nil {
		return AuditEntry{}, fmt.Errorf("read metadata %s: %w", metaPath, err)
	}

	entry := AuditEntry{Path: path}

	if secret == nil || secret.Data == nil {
		entry.NeedsRotation = true
		return entry, nil
	}

	if versions, ok := secret.Data["versions"].(map[string]interface{}); ok {
		if v, ok := secret.Data["current_version"].(float64); ok {
			entry.Version = int(v)
			vKey := fmt.Sprintf("%d", entry.Version)
			if vInfo, ok := versions[vKey].(map[string]interface{}); ok {
				if createdRaw, ok := vInfo["created_time"].(string); ok {
					if t, err := time.Parse(time.RFC3339, createdRaw); err == nil {
						entry.CreatedAt = t
						entry.AgeDays = int(time.Since(t).Hours() / 24)
					}
				}
			}
		}
	}

	entry.NeedsRotation = entry.AgeDays >= rotationThresholdDays
	return entry, nil
}

func generateSecret() (string, error) {
	b := make([]byte, randomBytesLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
