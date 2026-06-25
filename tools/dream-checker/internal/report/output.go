package report

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"
)

const (
	StatusPass = "PASS"
	StatusFail = "FAIL"
	StatusWarn = "WARN"
	StatusSkip = "SKIP"
)

type CheckResult struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
	Details   []string  `json:"details,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

func Output(results []CheckResult, format string) error {
	switch format {
	case "json":
		return printJSON(results)
	default:
		return printTable(results)
	}
}

func printTable(results []CheckResult) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "MODULE\tID\tSTATUS\tMESSAGE")
	for _, r := range results {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", moduleFromID(r.ID), r.ID, r.Status, r.Message)
		for _, d := range r.Details {
			fmt.Fprintf(w, "\t\t\t  %s\n", d)
		}
	}
	w.Flush()

	pass, fail, warn, skip := 0, 0, 0, 0
	for _, r := range results {
		switch r.Status {
		case StatusPass:
			pass++
		case StatusFail:
			fail++
		case StatusWarn:
			warn++
		case StatusSkip:
			skip++
		}
	}
	fmt.Printf("\nSummary: %d PASS, %d WARN, %d FAIL", pass, warn, fail)
	if skip > 0 {
		fmt.Printf(", %d SKIP", skip)
	}
	if fail > 0 {
		fmt.Println(" — exit code 1")
		return fmt.Errorf("%d check(s) failed", fail)
	}
	fmt.Println()
	return nil
}

func printJSON(results []CheckResult) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(results); err != nil {
		return err
	}
	for _, r := range results {
		if r.Status == StatusFail {
			n := countByStatus(results, StatusFail)
			return fmt.Errorf("%d check(s) failed", n)
		}
	}
	return nil
}

func countByStatus(results []CheckResult, status string) int {
	n := 0
	for _, r := range results {
		if r.Status == status {
			n++
		}
	}
	return n
}

func moduleFromID(id string) string {
	switch {
	case strings.HasPrefix(id, "K8S"):
		return "k8s"
	case strings.HasPrefix(id, "VAULT"):
		return "vault"
	case strings.HasPrefix(id, "PKI"):
		return "pki"
	case strings.HasPrefix(id, "SUPPLY"):
		return "supply"
	default:
		return "unknown"
	}
}
