package cmd

import (
	"github.com/igor-policee/dream-ops-lab/tools/dream-checker/internal/report"
	"github.com/igor-policee/dream-ops-lab/tools/dream-checker/internal/supply"
	"github.com/spf13/cobra"
)

var supplyCmd = &cobra.Command{
	Use:   "supply",
	Short: "Run supply-chain security checks",
	RunE: func(cmd *cobra.Command, args []string) error {
		results, err := collectSupplyResults()
		if err != nil {
			return err
		}
		return report.Output(results, outputFormat)
	},
}

func collectSupplyResults() ([]report.CheckResult, error) {
	return supply.RunChecks()
}
