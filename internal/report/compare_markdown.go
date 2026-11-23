package report

import (
	"fmt"
	"io"
	"strings"
)

func (c Comparison) WriteMarkdown(w io.Writer) error {
	var out strings.Builder
	fmt.Fprintln(&out, "# HAR timing comparison")
	fmt.Fprintf(&out, "\nBaseline: %d requests. Current: %d requests.\n\n", c.BaselineRequests, c.CurrentRequests)

	fmt.Fprintln(&out, "## Responses\n\n| Metric | Baseline count | Baseline rate | Current count | Current rate | Change (pp) |")
	fmt.Fprintln(&out, "| --- | ---: | ---: | ---: | ---: | ---: |")
	for _, row := range c.Rates {
		fmt.Fprintf(&out, "| %s | %d | %s | %d | %s | %s |\n",
			row.Metric, row.Baseline.Count, percent(row.Baseline.Percent),
			row.Current.Count, percent(row.Current.Percent), percentagePoints(row.ChangePP))
	}

	fmt.Fprintln(&out, "\n## Timings (ms)\n\n| Metric | Baseline samples | Baseline p50 | Baseline p95 | Current samples | Current p50 | Current p95 | p50 change | p95 change |")
	fmt.Fprintln(&out, "| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |")
	for _, row := range c.Timings {
		fmt.Fprintf(&out, "| %s | %d | %s | %s | %d | %s | %s | %s | %s |\n",
			row.Metric, row.Baseline.Samples, ms(row.Baseline.P50MS), ms(row.Baseline.P95MS),
			row.Current.Samples, ms(row.Current.P50MS), ms(row.Current.P95MS),
			signedMS(row.ChangeP50MS), signedMS(row.ChangeP95MS))
	}

	fmt.Fprintln(&out, "\nChanges are current minus baseline. Rates use all requests in each capture. Missing timings are excluded.")
	_, err := io.WriteString(w, out.String())
	return err
}

func percent(n *float64) string {
	if n == nil {
		return "—"
	}
	return fmt.Sprintf("%.2f%%", *n)
}

func percentagePoints(n *float64) string {
	if n == nil {
		return "—"
	}
	return fmt.Sprintf("%+.2f", *n)
}

func signedMS(n *int64) string {
	if n == nil {
		return "—"
	}
	return fmt.Sprintf("%+d", *n)
}
