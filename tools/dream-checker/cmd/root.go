package cmd

import (
	"fmt"
	"os"

	"github.com/igor-policee/dream-ops-lab/tools/dream-checker/internal/report"
	"github.com/spf13/cobra"
)

var (
	outputFormat  string
	namespace     string
	allNamespaces bool
)

var rootCmd = &cobra.Command{
	Use:   "dream-checker",
	Short: "Security posture checker for the dream-ops-lab platform",
	Long: `dream-checker runs security checks against Kubernetes, OpenBao,
PKI (step-ca), and supply-chain artefacts. Each module can be run
independently or all at once via the 'all' sub-command.`,
}

var allCmd = &cobra.Command{
	Use:   "all",
	Short: "Run all check modules",
	RunE: func(cmd *cobra.Command, args []string) error {
		type collector func() ([]report.CheckResult, error)
		modules := []collector{
			collectK8sResults,
			collectVaultResults,
			collectPKIResults,
			collectSupplyResults,
		}
		var all []report.CheckResult
		for _, collect := range modules {
			results, err := collect()
			if err != nil {
				return err
			}
			all = append(all, results...)
		}
		return report.Output(all, outputFormat)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputFormat, "output", "o", "table", "Output format: table|json")
	rootCmd.PersistentFlags().StringVarP(&namespace, "namespace", "n", "default", "Kubernetes namespace")
	rootCmd.PersistentFlags().BoolVar(&allNamespaces, "all-namespaces", false, "Check all namespaces")

	rootCmd.AddCommand(allCmd, k8sCmd, vaultCmd, pkiCmd, supplyCmd)
}
