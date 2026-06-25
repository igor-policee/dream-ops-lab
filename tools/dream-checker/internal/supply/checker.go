package supply

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/igor-policee/dream-ops-lab/tools/dream-checker/internal/report"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func newK8sClient() (kubernetes.Interface, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		rules := clientcmd.NewDefaultClientConfigLoadingRules()
		cfg, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, nil).ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("build kubeconfig: %w", err)
		}
	}
	return kubernetes.NewForConfig(cfg)
}

func RunChecks() ([]report.CheckResult, error) {
	client, err := newK8sClient()
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var results []report.CheckResult

	results = append(results, checkGitleaks(now))
	results = append(results, checkTrivyAvailable(now))
	results = append(results, checkCosignImages(client, now))
	results = append(results, checkSBOMPresence(client, now))
	results = append(results, checkImageTags(client, now))

	return results, nil
}

// SUPPLY-001: gitleaks is installed and available
func checkGitleaks(now time.Time) report.CheckResult {
	r := report.CheckResult{ID: "SUPPLY-001", Name: "Gitleaks available", CheckedAt: now}

	cmd := exec.Command("gitleaks", "version")
	out, err := cmd.Output()
	if err != nil {
		r.Status = report.StatusWarn
		r.Message = "gitleaks not found in PATH — secret scanning disabled"
		return r
	}

	r.Status = report.StatusPass
	r.Message = strings.TrimSpace(string(out))
	return r
}

// SUPPLY-002: trivy is installed and available
func checkTrivyAvailable(now time.Time) report.CheckResult {
	r := report.CheckResult{ID: "SUPPLY-002", Name: "Trivy available", CheckedAt: now}

	cmd := exec.Command("trivy", "version", "--format", "json")
	out, err := cmd.Output()
	if err != nil {
		r.Status = report.StatusWarn
		r.Message = "trivy not found in PATH — vulnerability scanning unavailable"
		return r
	}

	line := strings.SplitN(strings.TrimSpace(string(out)), "\n", 2)[0]
	r.Status = report.StatusPass
	r.Message = line
	return r
}

// SUPPLY-003: All non-system workload images have cosign signatures (checked via annotation)
func checkCosignImages(client kubernetes.Interface, now time.Time) report.CheckResult {
	r := report.CheckResult{ID: "SUPPLY-003", Name: "Images cosign-signed", CheckedAt: now}

	pods, err := client.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		r.Status = report.StatusSkip
		r.Message = fmt.Sprintf("list pods failed: %v", err)
		return r
	}

	var unsigned []string
	for _, pod := range pods.Items {
		if isSystemNamespace(pod.Namespace) {
			continue
		}
		if pod.Annotations["cosign.sigstore.dev/signed"] != "true" {
			unsigned = append(unsigned, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
		}
	}

	if len(unsigned) == 0 {
		r.Status = report.StatusPass
		r.Message = "All non-system pods have cosign signature annotation"
	} else {
		r.Status = report.StatusWarn
		r.Message = fmt.Sprintf("%d pod(s) missing cosign signature annotation", len(unsigned))
		r.Details = unsigned
	}
	return r
}

// SUPPLY-004: Workload pods reference SBOM ConfigMaps or have SBOM annotation
func checkSBOMPresence(client kubernetes.Interface, now time.Time) report.CheckResult {
	r := report.CheckResult{ID: "SUPPLY-004", Name: "SBOM present", CheckedAt: now}

	pods, err := client.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		r.Status = report.StatusSkip
		r.Message = fmt.Sprintf("list pods failed: %v", err)
		return r
	}

	var noSBOM []string
	for _, pod := range pods.Items {
		if isSystemNamespace(pod.Namespace) {
			continue
		}
		if pod.Annotations["sbom.syft.dev/image-digest"] == "" {
			noSBOM = append(noSBOM, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
		}
	}

	if len(noSBOM) == 0 {
		r.Status = report.StatusPass
		r.Message = "All non-system pods have SBOM annotation"
	} else {
		r.Status = report.StatusWarn
		r.Message = fmt.Sprintf("%d pod(s) missing SBOM annotation", len(noSBOM))
		r.Details = noSBOM
	}
	return r
}

// SUPPLY-005: No workload containers use latest or missing image tags
func checkImageTags(client kubernetes.Interface, now time.Time) report.CheckResult {
	r := report.CheckResult{ID: "SUPPLY-005", Name: "No mutable image tags", CheckedAt: now}

	pods, err := client.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		r.Status = report.StatusSkip
		r.Message = fmt.Sprintf("list pods failed: %v", err)
		return r
	}

	var mutable []string
	for _, pod := range pods.Items {
		if isSystemNamespace(pod.Namespace) {
			continue
		}
		for _, c := range pod.Spec.Containers {
			if isMutableTag(c.Image) {
				mutable = append(mutable, fmt.Sprintf("%s/%s/%s: %s", pod.Namespace, pod.Name, c.Name, c.Image))
			}
		}
	}

	if len(mutable) == 0 {
		r.Status = report.StatusPass
		r.Message = "No mutable image tags in non-system workloads"
	} else {
		r.Status = report.StatusFail
		r.Message = fmt.Sprintf("%d container(s) use mutable image tags", len(mutable))
		r.Details = mutable
	}
	return r
}

func isMutableTag(image string) bool {
	// Isolate the name component (after the last /) so that a registry port like
	// registry.dream.lab:5000/app is not mistaken for a tag separator.
	name := image
	if idx := strings.LastIndex(image, "/"); idx >= 0 {
		name = image[idx+1:]
	}
	if !strings.Contains(name, ":") {
		return true
	}
	tag := strings.SplitN(name, ":", 2)[1]
	return tag == "latest" || tag == ""
}

func isSystemNamespace(ns string) bool {
	system := map[string]bool{
		"kube-system":     true,
		"kube-public":     true,
		"kube-node-lease": true,
	}
	return system[ns]
}
