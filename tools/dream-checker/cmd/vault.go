package cmd

import (
	"os"

	"github.com/igor-policee/dream-ops-lab/tools/dream-checker/internal/report"
	"github.com/igor-policee/dream-ops-lab/tools/dream-checker/internal/vault"
	"github.com/spf13/cobra"
)

var vaultCmd = &cobra.Command{
	Use:   "vault",
	Short: "Run OpenBao / Vault security checks",
	RunE: func(cmd *cobra.Command, args []string) error {
		results, err := collectVaultResults()
		if err != nil {
			return err
		}
		return report.Output(results, outputFormat)
	},
}

func collectVaultResults() ([]report.CheckResult, error) {
	addr := os.Getenv("VAULT_ADDR")
	token := os.Getenv("VAULT_TOKEN")
	return vault.RunChecks(addr, token)
}
