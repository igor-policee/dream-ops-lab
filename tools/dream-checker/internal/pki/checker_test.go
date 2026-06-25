package pki

import (
	"testing"
	"time"

	"github.com/igor-policee/dream-ops-lab/tools/dream-checker/internal/report"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func makeCertificate(name, namespace string, notAfter time.Time, ready bool) *unstructured.Unstructured {
	readyStatus := "False"
	if ready {
		readyStatus = "True"
	}
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cert-manager.io/v1",
			"kind":       "Certificate",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"status": map[string]interface{}{
				"notAfter": notAfter.Format(time.RFC3339),
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": readyStatus,
					},
				},
			},
		},
	}
}

func newFakeDynamic(objs ...*unstructured.Unstructured) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	runtimeObjs := make([]runtime.Object, len(objs))
	for i, o := range objs {
		runtimeObjs[i] = o
	}
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		map[schema.GroupVersionResource]string{certificateGVR: "CertificateList"},
		runtimeObjs...,
	)
}

func TestCheckCertExpiry_Pass(t *testing.T) {
	cert := makeCertificate("valid", "default", time.Now().Add(90*24*time.Hour), true)
	client := newFakeDynamic(cert)
	r := checkCertExpiry(client, "default", time.Now())
	if r.Status != report.StatusPass {
		t.Errorf("expected PASS, got %s: %s", r.Status, r.Message)
	}
}

func TestCheckCertExpiry_FailExpired(t *testing.T) {
	cert := makeCertificate("expired", "default", time.Now().Add(-24*time.Hour), false)
	client := newFakeDynamic(cert)
	r := checkCertExpiry(client, "default", time.Now())
	if r.Status != report.StatusFail {
		t.Errorf("expected FAIL, got %s: %s", r.Status, r.Message)
	}
}

func TestCheckCertExpiry_FailExpiringSoon(t *testing.T) {
	cert := makeCertificate("expiring", "default", time.Now().Add(2*24*time.Hour), true)
	client := newFakeDynamic(cert)
	r := checkCertExpiry(client, "default", time.Now())
	if r.Status != report.StatusFail {
		t.Errorf("expected FAIL for cert expiring in 2 days, got %s: %s", r.Status, r.Message)
	}
}

func TestCheckCertRenewalStatus_Pass(t *testing.T) {
	cert := makeCertificate("ready", "default", time.Now().Add(90*24*time.Hour), true)
	client := newFakeDynamic(cert)
	r := checkCertRenewalStatus(client, "default", time.Now())
	if r.Status != report.StatusPass {
		t.Errorf("expected PASS, got %s: %s", r.Status, r.Message)
	}
}

func TestCheckCertRenewalStatus_Fail(t *testing.T) {
	cert := makeCertificate("notready", "default", time.Now().Add(90*24*time.Hour), false)
	client := newFakeDynamic(cert)
	r := checkCertRenewalStatus(client, "default", time.Now())
	if r.Status != report.StatusFail {
		t.Errorf("expected FAIL, got %s: %s", r.Status, r.Message)
	}
}

func TestCheckCertRenewalStatus_SkipNoCerts(t *testing.T) {
	client := newFakeDynamic()
	r := checkCertRenewalStatus(client, "default", time.Now())
	if r.Status != report.StatusSkip {
		t.Errorf("expected SKIP when no certs, got %s: %s", r.Status, r.Message)
	}
}

func TestCheckCAReachable_SkipWhenEmpty(t *testing.T) {
	r := checkCAReachable("", time.Now())
	if r.Status != report.StatusSkip {
		t.Errorf("expected SKIP, got %s: %s", r.Status, r.Message)
	}
}

func TestCheckCAReachable_FailUnreachable(t *testing.T) {
	r := checkCAReachable("step-ca.dream.lab:19999", time.Now())
	if r.Status != report.StatusFail {
		t.Errorf("expected FAIL for unreachable addr, got %s: %s", r.Status, r.Message)
	}
}

func TestIsCertificateReady_True(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "True"},
			},
		},
	}
	if !isCertificateReady(obj) {
		t.Error("expected ready=true")
	}
}

func TestIsCertificateReady_False(t *testing.T) {
	obj := map[string]interface{}{
		"status": map[string]interface{}{
			"conditions": []interface{}{
				map[string]interface{}{"type": "Ready", "status": "False"},
			},
		},
	}
	if isCertificateReady(obj) {
		t.Error("expected ready=false")
	}
}
