package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/igor-policee/dream-ops-lab/tools/bao-rotator/internal/rotator"
	"github.com/spf13/cobra"
)

var (
	vaultAddr  string
	vaultToken string
)

func main() {
	root := &cobra.Command{
		Use:   "bao-rotator",
		Short: "Rotate secrets in OpenBao / Vault",
	}

	root.PersistentFlags().StringVar(&vaultAddr, "addr", "", "Vault/Bao address (overrides VAULT_ADDR)")
	root.PersistentFlags().StringVar(&vaultToken, "token", "", "Vault/Bao token (overrides VAULT_TOKEN)")

	root.AddCommand(listCmd(), rotateCmd(), auditCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <mount>",
		Short: "List secrets at a KV mount path",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := newRotator()
			if err != nil {
				return err
			}
			keys, err := r.List(args[0])
			if err != nil {
				return err
			}
			for _, k := range keys {
				fmt.Println(k)
			}
			return nil
		},
	}
}

func rotateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rotate <mount> <path>",
		Short: "Rotate a secret at the given KV path",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := newRotator()
			if err != nil {
				return err
			}
			result, err := r.Rotate(args[0], args[1])
			if err != nil {
				return err
			}
			slog.Info("secret rotated",
				"mount", args[0],
				"path", args[1],
				"version", result.Version,
			)
			return nil
		},
	}
}

func auditCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "audit <mount>",
		Short: "Audit secrets at a KV mount path for age and rotation policy",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			r, err := newRotator()
			if err != nil {
				return err
			}
			entries, err := r.Audit(args[0])
			if err != nil {
				return err
			}
			for _, entry := range entries {
				if entry.NeedsRotation {
					slog.Warn("secret needs rotation",
						"path", entry.Path,
						"age_days", entry.AgeDays,
						"version", entry.Version,
					)
				} else {
					slog.Info("secret ok",
						"path", entry.Path,
						"age_days", entry.AgeDays,
						"version", entry.Version,
					)
				}
			}
			return nil
		},
	}
}

func newRotator() (*rotator.Rotator, error) {
	addr := vaultAddr
	if addr == "" {
		addr = os.Getenv("VAULT_ADDR")
	}
	token := vaultToken
	if token == "" {
		token = os.Getenv("VAULT_TOKEN")
	}
	return rotator.New(addr, token)
}
