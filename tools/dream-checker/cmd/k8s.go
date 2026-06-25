package cmd

import (
	"github.com/igor-policee/dream-ops-lab/tools/dream-checker/internal/k8s"
	"github.com/igor-policee/dream-ops-lab/tools/dream-checker/internal/report"
	"github.com/spf13/cobra"
)

var k8sCmd = &cobra.Command{
	Use:   "k8s",
	Short: "Run Kubernetes security checks",
	RunE: func(cmd *cobra.Command, args []string) error {
		results, err := collectK8sResults()
		if err != nil {
			return err
		}
		return report.Output(results, outputFormat)
	},
}

func collectK8sResults() ([]report.CheckResult, error) {
	ns := namespace
	if allNamespaces {
		ns = ""
	}
	return k8s.RunChecks(ns)
}
