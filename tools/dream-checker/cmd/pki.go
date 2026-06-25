package cmd

import (
	"os"

	"github.com/igor-policee/dream-ops-lab/tools/dream-checker/internal/pki"
	"github.com/igor-policee/dream-ops-lab/tools/dream-checker/internal/report"
	"github.com/spf13/cobra"
)

var pkiCmd = &cobra.Command{
	Use:   "pki",
	Short: "Run PKI / certificate checks",
	RunE: func(cmd *cobra.Command, args []string) error {
		results, err := collectPKIResults()
		if err != nil {
			return err
		}
		return report.Output(results, outputFormat)
	},
}

func collectPKIResults() ([]report.CheckResult, error) {
	// STEP_CA_ADDR accepts host:port or full URL; PKI module parses it either way
	caAddr := os.Getenv("STEP_CA_ADDR")
	if caAddr == "" {
		caAddr = os.Getenv("STEP_CA_URL") // fallback for backward compatibility
	}
	ns := namespace
	if allNamespaces {
		ns = ""
	}
	return pki.RunChecks(caAddr, ns)
}
