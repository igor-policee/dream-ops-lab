package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/igor-policee/dream-ops-lab/tools/dream-checker/internal/report"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func newClient() (kubernetes.Interface, error) {
	// Try in-cluster config first (running as a CronJob); fall back to kubeconfig for local use.
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

func RunChecks(namespace string) ([]report.CheckResult, error) {
	client, err := newClient()
	if err != nil {
		return nil, err
	}

	var results []report.CheckResult
	now := time.Now()

	results = append(results, checkPrivilegedPods(client, namespace, now))
	results = append(results, checkHostNetworkPods(client, namespace, now))
	results = append(results, checkRootContainers(client, namespace, now))
	results = append(results, checkResourceLimits(client, namespace, now))
	results = append(results, checkDefaultServiceAccounts(client, namespace, now))
	results = append(results, checkNetworkPolicies(client, namespace, now))
	results = append(results, checkPodSecurityLabels(client, namespace, now))

	return results, nil
}

// K8S-001: No privileged pods running
func checkPrivilegedPods(client kubernetes.Interface, namespace string, now time.Time) report.CheckResult {
	r := report.CheckResult{ID: "K8S-001", Name: "No privileged pods", CheckedAt: now}

	pods, err := listPods(client, namespace)
	if err != nil {
		r.Status = report.StatusSkip
		r.Message = err.Error()
		return r
	}

	var privileged []string
	for _, pod := range pods.Items {
		for _, c := range pod.Spec.Containers {
			if c.SecurityContext != nil && c.SecurityContext.Privileged != nil && *c.SecurityContext.Privileged {
				privileged = append(privileged, fmt.Sprintf("%s/%s(%s)", pod.Namespace, pod.Name, c.Name))
			}
		}
	}

	if len(privileged) == 0 {
		r.Status = report.StatusPass
		r.Message = "No privileged containers found"
	} else {
		r.Status = report.StatusFail
		r.Message = fmt.Sprintf("%d privileged container(s) found", len(privileged))
		r.Details = privileged
	}
	return r
}

// K8S-002: No pods using hostNetwork
func checkHostNetworkPods(client kubernetes.Interface, namespace string, now time.Time) report.CheckResult {
	r := report.CheckResult{ID: "K8S-002", Name: "No hostNetwork pods", CheckedAt: now}

	pods, err := listPods(client, namespace)
	if err != nil {
		r.Status = report.StatusSkip
		r.Message = err.Error()
		return r
	}

	var offenders []string
	for _, pod := range pods.Items {
		if pod.Spec.HostNetwork {
			offenders = append(offenders, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
		}
	}

	if len(offenders) == 0 {
		r.Status = report.StatusPass
		r.Message = "No hostNetwork pods found"
	} else {
		r.Status = report.StatusFail
		r.Message = fmt.Sprintf("%d pod(s) using hostNetwork", len(offenders))
		r.Details = offenders
	}
	return r
}

// K8S-003: No containers running as root (UID 0)
func checkRootContainers(client kubernetes.Interface, namespace string, now time.Time) report.CheckResult {
	r := report.CheckResult{ID: "K8S-003", Name: "No root containers", CheckedAt: now}

	pods, err := listPods(client, namespace)
	if err != nil {
		r.Status = report.StatusSkip
		r.Message = err.Error()
		return r
	}

	var offenders []string
	for _, pod := range pods.Items {
		podRunsAsRoot := pod.Spec.SecurityContext != nil &&
			pod.Spec.SecurityContext.RunAsUser != nil &&
			*pod.Spec.SecurityContext.RunAsUser == 0

		for _, c := range pod.Spec.Containers {
			runAsRoot := podRunsAsRoot
			if c.SecurityContext != nil && c.SecurityContext.RunAsUser != nil {
				runAsRoot = *c.SecurityContext.RunAsUser == 0
			}
			if runAsRoot {
				offenders = append(offenders, fmt.Sprintf("%s/%s(%s)", pod.Namespace, pod.Name, c.Name))
			}
		}
	}

	if len(offenders) == 0 {
		r.Status = report.StatusPass
		r.Message = "No root containers found"
	} else {
		r.Status = report.StatusFail
		r.Message = fmt.Sprintf("%d container(s) running as root", len(offenders))
		r.Details = offenders
	}
	return r
}

// K8S-004: All containers have CPU and memory limits set
func checkResourceLimits(client kubernetes.Interface, namespace string, now time.Time) report.CheckResult {
	r := report.CheckResult{ID: "K8S-004", Name: "Resource limits set", CheckedAt: now}

	pods, err := listPods(client, namespace)
	if err != nil {
		r.Status = report.StatusSkip
		r.Message = err.Error()
		return r
	}

	var missing []string
	for _, pod := range pods.Items {
		for _, c := range pod.Spec.Containers {
			if c.Resources.Limits == nil ||
				c.Resources.Limits.Cpu().IsZero() ||
				c.Resources.Limits.Memory().IsZero() {
				missing = append(missing, fmt.Sprintf("%s/%s(%s)", pod.Namespace, pod.Name, c.Name))
			}
		}
	}

	if len(missing) == 0 {
		r.Status = report.StatusPass
		r.Message = "All containers have CPU and memory limits"
	} else {
		r.Status = report.StatusWarn
		r.Message = fmt.Sprintf("%d container(s) missing resource limits", len(missing))
		r.Details = missing
	}
	return r
}

// K8S-005: No workloads use automounted default service accounts
func checkDefaultServiceAccounts(client kubernetes.Interface, namespace string, now time.Time) report.CheckResult {
	r := report.CheckResult{ID: "K8S-005", Name: "Default SA not automounted", CheckedAt: now}

	pods, err := listPods(client, namespace)
	if err != nil {
		r.Status = report.StatusSkip
		r.Message = err.Error()
		return r
	}

	var offenders []string
	for _, pod := range pods.Items {
		isDefault := pod.Spec.ServiceAccountName == "" || pod.Spec.ServiceAccountName == "default"
		automounted := pod.Spec.AutomountServiceAccountToken == nil || *pod.Spec.AutomountServiceAccountToken
		if isDefault && automounted {
			offenders = append(offenders, fmt.Sprintf("%s/%s", pod.Namespace, pod.Name))
		}
	}

	if len(offenders) == 0 {
		r.Status = report.StatusPass
		r.Message = "No pods automounting default service account"
	} else {
		r.Status = report.StatusWarn
		r.Message = fmt.Sprintf("%d pod(s) using default SA with automount", len(offenders))
		r.Details = offenders
	}
	return r
}

// K8S-006: Every namespace has at least one NetworkPolicy
func checkNetworkPolicies(client kubernetes.Interface, namespace string, now time.Time) report.CheckResult {
	r := report.CheckResult{ID: "K8S-006", Name: "NetworkPolicy per namespace", CheckedAt: now}

	var namespaces []string
	if namespace != "" {
		namespaces = []string{namespace}
	} else {
		nsList, err := client.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			r.Status = report.StatusSkip
			r.Message = err.Error()
			return r
		}
		for _, ns := range nsList.Items {
			namespaces = append(namespaces, ns.Name)
		}
	}

	var uncovered []string
	for _, ns := range namespaces {
		policies, err := client.NetworkingV1().NetworkPolicies(ns).List(context.Background(), metav1.ListOptions{})
		if err != nil || len(policies.Items) == 0 {
			uncovered = append(uncovered, ns)
		}
	}

	if len(uncovered) == 0 {
		r.Status = report.StatusPass
		r.Message = "All namespaces have NetworkPolicies"
	} else {
		r.Status = report.StatusWarn
		r.Message = fmt.Sprintf("%d namespace(s) without NetworkPolicy", len(uncovered))
		r.Details = uncovered
	}
	return r
}

// K8S-007: Namespaces have pod-security admission labels
func checkPodSecurityLabels(client kubernetes.Interface, namespace string, now time.Time) report.CheckResult {
	r := report.CheckResult{ID: "K8S-007", Name: "Pod security admission labels", CheckedAt: now}

	var namespaces []corev1.Namespace
	if namespace != "" {
		ns, err := client.CoreV1().Namespaces().Get(context.Background(), namespace, metav1.GetOptions{})
		if err != nil {
			r.Status = report.StatusSkip
			r.Message = err.Error()
			return r
		}
		namespaces = []corev1.Namespace{*ns}
	} else {
		nsList, err := client.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			r.Status = report.StatusSkip
			r.Message = err.Error()
			return r
		}
		namespaces = nsList.Items
	}

	var missing []string
	for _, ns := range namespaces {
		if ns.Labels["pod-security.kubernetes.io/enforce"] == "" {
			missing = append(missing, ns.Name)
		}
	}

	if len(missing) == 0 {
		r.Status = report.StatusPass
		r.Message = "All namespaces have pod-security enforce label"
	} else {
		r.Status = report.StatusWarn
		r.Message = fmt.Sprintf("%d namespace(s) missing pod-security enforce label", len(missing))
		r.Details = missing
	}
	return r
}

func listPods(client kubernetes.Interface, namespace string) (*corev1.PodList, error) {
	return client.CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{
		FieldSelector: "status.phase=Running",
	})
}
