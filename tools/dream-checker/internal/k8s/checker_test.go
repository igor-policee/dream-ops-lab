package k8s

import (
	"testing"
	"time"

	"github.com/igor-policee/dream-ops-lab/tools/dream-checker/internal/report"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func boolPtr(b bool) *bool { return &b }
func int64Ptr(i int64) *int64 { return &i }

func makePod(name, namespace string, modify func(*corev1.Pod)) *corev1.Pod {
	p := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "app:latest",
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("100m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
					SecurityContext: &corev1.SecurityContext{
						RunAsUser:  int64Ptr(1000),
						Privileged: boolPtr(false),
					},
				},
			},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	if modify != nil {
		modify(p)
	}
	return p
}

func TestCheckPrivilegedPods_Pass(t *testing.T) {
	pod := makePod("safe", "default", nil)
	client := fake.NewSimpleClientset(pod)
	r := checkPrivilegedPods(client, "default", time.Now())
	if r.Status != report.StatusPass {
		t.Errorf("expected PASS, got %s: %s", r.Status, r.Message)
	}
}

func TestCheckPrivilegedPods_Fail(t *testing.T) {
	pod := makePod("priv", "default", func(p *corev1.Pod) {
		p.Spec.Containers[0].SecurityContext.Privileged = boolPtr(true)
	})
	client := fake.NewSimpleClientset(pod)
	r := checkPrivilegedPods(client, "default", time.Now())
	if r.Status != report.StatusFail {
		t.Errorf("expected FAIL, got %s: %s", r.Status, r.Message)
	}
}

func TestCheckHostNetworkPods_Pass(t *testing.T) {
	pod := makePod("safe", "default", nil)
	client := fake.NewSimpleClientset(pod)
	r := checkHostNetworkPods(client, "default", time.Now())
	if r.Status != report.StatusPass {
		t.Errorf("expected PASS, got %s: %s", r.Status, r.Message)
	}
}

func TestCheckHostNetworkPods_Fail(t *testing.T) {
	pod := makePod("hostnet", "default", func(p *corev1.Pod) {
		p.Spec.HostNetwork = true
	})
	client := fake.NewSimpleClientset(pod)
	r := checkHostNetworkPods(client, "default", time.Now())
	if r.Status != report.StatusFail {
		t.Errorf("expected FAIL, got %s: %s", r.Status, r.Message)
	}
}

func TestCheckRootContainers_Pass(t *testing.T) {
	pod := makePod("safe", "default", nil)
	client := fake.NewSimpleClientset(pod)
	r := checkRootContainers(client, "default", time.Now())
	if r.Status != report.StatusPass {
		t.Errorf("expected PASS, got %s: %s", r.Status, r.Message)
	}
}

func TestCheckRootContainers_Fail(t *testing.T) {
	pod := makePod("root", "default", func(p *corev1.Pod) {
		p.Spec.Containers[0].SecurityContext.RunAsUser = int64Ptr(0)
	})
	client := fake.NewSimpleClientset(pod)
	r := checkRootContainers(client, "default", time.Now())
	if r.Status != report.StatusFail {
		t.Errorf("expected FAIL, got %s: %s", r.Status, r.Message)
	}
}

func TestCheckResourceLimits_Pass(t *testing.T) {
	pod := makePod("safe", "default", nil)
	client := fake.NewSimpleClientset(pod)
	r := checkResourceLimits(client, "default", time.Now())
	if r.Status != report.StatusPass {
		t.Errorf("expected PASS, got %s: %s", r.Status, r.Message)
	}
}

func TestCheckResourceLimits_Warn(t *testing.T) {
	pod := makePod("nolimits", "default", func(p *corev1.Pod) {
		p.Spec.Containers[0].Resources.Limits = nil
	})
	client := fake.NewSimpleClientset(pod)
	r := checkResourceLimits(client, "default", time.Now())
	if r.Status != report.StatusWarn {
		t.Errorf("expected WARN, got %s: %s", r.Status, r.Message)
	}
}
