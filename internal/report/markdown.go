package report

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

func (r Report) WriteMarkdown(w io.Writer) error {
	var out strings.Builder
	fmt.Fprintln(&out, "# HAR timing report")
	fmt.Fprintln(&out)
	fmt.Fprintf(&out, "%d requests across %d known HTTP origins. Showing %d requests.\n\n",
		r.Summary.Requests, r.Summary.Origins, len(r.Requests))
	fmt.Fprintf(&out, "- HTTP 4xx: %d\n- HTTP 5xx: %d\n- Missing HTTP status: %d\n",
		r.Summary.ClientErrors, r.Summary.ServerErrors, r.Summary.NoHTTPStatus)
	fmt.Fprintf(&out, "- Total duration (ms, %d known): p50 %s, p95 %s\n\n",
		r.Summary.KnownDurations, ms(r.Summary.P50MS), ms(r.Summary.P95MS))
	if len(r.Requests) > 0 {
		heading := "Requests by duration"
		if r.Order == "start" {
			heading = "Requests by start time"
		}
		fmt.Fprintf(&out, "## %s\n\n", heading)
		fmt.Fprintln(&out, "| # | Origin | Endpoint | Method | Status | Start (ms) | Total (ms) | Wait (ms) |")
		fmt.Fprintln(&out, "| ---: | --- | --- | --- | ---: | ---: | ---: | ---: |")
		for _, row := range r.Requests {
			status := "—"
			if row.Status != nil {
				status = strconv.Itoa(*row.Status)
			}
			fmt.Fprintf(&out, "| %d | %s | %s | %s | %s | %s | %s | %s |\n",
				row.Ordinal, row.Origin, row.Endpoint, row.Method, status,
				ms(row.StartMS), ms(row.TotalMS), ms(row.Phases.WaitMS))
		}
		fmt.Fprintln(&out)
	}
	fmt.Fprintln(&out, "Aliases are local to this capture. Start times are relative to the first captured request with a valid time.")
	_, err := io.WriteString(w, out.String())
	return err
}

func ms(value *int64) string {
	if value == nil {
		return "—"
	}
	return strconv.FormatInt(*value, 10)
}
