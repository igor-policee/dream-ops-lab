package supply

import (
	"testing"
	"time"

	"github.com/igor-policee/dream-ops-lab/tools/dream-checker/internal/report"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func makePod(name, namespace, image string, annotations map[string]string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: annotations,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{Name: "app", Image: image},
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

func TestCheckImageTags_Pass(t *testing.T) {
	pod := makePod("app", "production", "registry.example.com/app:v1.2.3", nil)
	client := fake.NewSimpleClientset(pod)
	r := checkImageTags(client, time.Now())
	if r.Status != report.StatusPass {
		t.Errorf("expected PASS, got %s: %s", r.Status, r.Message)
	}
}

func TestCheckImageTags_FailLatest(t *testing.T) {
	pod := makePod("app", "production", "registry.example.com/app:latest", nil)
	client := fake.NewSimpleClientset(pod)
	r := checkImageTags(client, time.Now())
	if r.Status != report.StatusFail {
		t.Errorf("expected FAIL, got %s: %s", r.Status, r.Message)
	}
}

func TestCheckImageTags_FailNoTag(t *testing.T) {
	pod := makePod("app", "production", "registry.example.com/app", nil)
	client := fake.NewSimpleClientset(pod)
	r := checkImageTags(client, time.Now())
	if r.Status != report.StatusFail {
		t.Errorf("expected FAIL, got %s: %s", r.Status, r.Message)
	}
}

func TestCheckImageTags_SkipSystemNamespace(t *testing.T) {
	pod := makePod("coredns", "kube-system", "registry.k8s.io/coredns:latest", nil)
	client := fake.NewSimpleClientset(pod)
	r := checkImageTags(client, time.Now())
	if r.Status != report.StatusPass {
		t.Errorf("kube-system pods should be ignored, got %s: %s", r.Status, r.Message)
	}
}

func TestCheckCosignImages_Pass(t *testing.T) {
	pod := makePod("app", "production", "registry.example.com/app:v1.0", map[string]string{
		"cosign.sigstore.dev/signed": "true",
	})
	client := fake.NewSimpleClientset(pod)
	r := checkCosignImages(client, time.Now())
	if r.Status != report.StatusPass {
		t.Errorf("expected PASS, got %s: %s", r.Status, r.Message)
	}
}

func TestCheckCosignImages_Warn(t *testing.T) {
	pod := makePod("app", "production", "registry.example.com/app:v1.0", nil)
	client := fake.NewSimpleClientset(pod)
	r := checkCosignImages(client, time.Now())
	if r.Status != report.StatusWarn {
		t.Errorf("expected WARN, got %s: %s", r.Status, r.Message)
	}
}

func TestIsMutableTag(t *testing.T) {
	cases := []struct {
		image   string
		mutable bool
	}{
		{"app:latest", true},
		{"app:", true},
		{"app", true},
		{"registry.example.com/app:v1.2.3", false},
		{"registry.example.com/app:sha256-abc123", false},
		// registry with port — colon in host must not be treated as tag separator
		{"registry.dream.lab:5000/app", true},
		{"registry.dream.lab:5000/app:latest", true},
		{"registry.dream.lab:5000/app:v1.0", false},
		{"localhost:5000/myapp", true},
		{"localhost:5000/myapp:v2.3.1", false},
	}
	for _, tc := range cases {
		got := isMutableTag(tc.image)
		if got != tc.mutable {
			t.Errorf("isMutableTag(%q) = %v, want %v", tc.image, got, tc.mutable)
		}
	}
}
