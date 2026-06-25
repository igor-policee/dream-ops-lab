package vault

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/igor-policee/dream-ops-lab/tools/dream-checker/internal/report"
)

func TestRunChecks_SkipOnUnreachable(t *testing.T) {
	results, err := RunChecks("http://127.0.0.1:19999", "test-token")
	if err != nil {
		t.Fatalf("RunChecks should not return error when vault is unreachable, got: %v", err)
	}
	for _, r := range results {
		if r.Status != report.StatusSkip {
			t.Errorf("check %s: expected SKIP when vault unreachable, got %s", r.ID, r.Status)
		}
	}
}

func TestCheckVaultSealed_Pass(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"sealed":false,"version":"1.15.0","cluster_name":"test"}`))
		}
	}))
	defer srv.Close()

	client, err := newClient(srv.URL, "test-token")
	if err != nil {
		t.Fatal(err)
	}

	r := checkVaultSealed(client, time.Now())
	if r.Status != report.StatusPass {
		t.Errorf("expected PASS, got %s: %s", r.Status, r.Message)
	}
}

func TestCheckVaultSealed_Fail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/sys/health" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte(`{"sealed":true,"version":"1.15.0"}`))
		}
	}))
	defer srv.Close()

	client, err := newClient(srv.URL, "test-token")
	if err != nil {
		t.Fatal(err)
	}

	r := checkVaultSealed(client, time.Now())
	if r.Status != report.StatusFail {
		t.Errorf("expected FAIL, got %s: %s", r.Status, r.Message)
	}
}
