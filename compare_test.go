package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/daimingyu1998/har-timing-report/internal/report"
)

func TestCompareCaptures(t *testing.T) {
	for _, format := range []string{"markdown", "json"} {
		t.Run(format, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			args := []string{"-baseline", "examples/synthetic.har", "-format", format, "examples/synthetic-regression.har"}
			if code := run(args, nil, &stdout, &stderr); code != 0 {
				t.Fatalf("exit %d: %s", code, &stderr)
			}
			output := stdout.String()
			for _, raw := range []string{
				"api-regression.example.test", "demo-id", "regression-private-query",
				"regression-private-body", "example-credential", "2026-02-01",
				"endpoint-2", "origin-2", "synthetic-regression.har",
			} {
				if strings.Contains(output, raw) {
					t.Fatalf("comparison exposed %q", raw)
				}
			}
			if format == "markdown" {
				if !strings.Contains(output, "| http_5xx | 1 | 33.33% | 2 | 50.00% | +16.67 |") ||
					!strings.Contains(output, "| total | 3 | 122 | 842 | 4 | 122 | 1442 | +0 | +600 |") {
					t.Fatalf("comparison lacks expected regression: %s", output)
				}
				return
			}
			var c report.Comparison
			if err := json.Unmarshal(stdout.Bytes(), &c); err != nil {
				t.Fatal(err)
			}
			if c.SchemaVersion != 1 || c.BaselineRequests != 3 || c.CurrentRequests != 4 {
				t.Fatalf("incorrect capture counts or version: %+v", c)
			}
			for _, timing := range c.Timings {
				if timing.Metric == "wait" {
					if timing.Baseline.Samples != 3 || timing.Current.Samples != 4 ||
						timing.ChangeP95MS == nil || *timing.ChangeP95MS != 600 {
						t.Fatalf("incorrect wait comparison: %+v", timing)
					}
					return
				}
			}
			t.Fatal("wait comparison missing")
		})
	}
}

func TestCompareHandlesEmptyBaseline(t *testing.T) {
	var stdout, stderr bytes.Buffer
	args := []string{"-baseline", "-", "-format", "json", "examples/synthetic.har"}
	if code := run(args, strings.NewReader(`{"log":{"entries":[]}}`), &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, &stderr)
	}
	var c report.Comparison
	if err := json.Unmarshal(stdout.Bytes(), &c); err != nil {
		t.Fatal(err)
	}
	if len(c.Rates) != 3 || len(c.Timings) != 8 ||
		c.Rates[0].Baseline.Percent != nil || c.Rates[0].ChangePP != nil ||
		c.Timings[0].Baseline.P50MS != nil || c.Timings[0].ChangeP50MS != nil ||
		c.Timings[0].Current.P95MS == nil {
		t.Fatalf("empty baseline must have unknown rates and deltas: %+v", c)
	}
}

func TestCompareRejectsUnsupportedInputs(t *testing.T) {
	for _, args := range [][]string{
		{"-baseline", "-", "-"},
		{"-baseline", "examples/synthetic.har", "-format", "html", "examples/synthetic-regression.har"},
		{"-baseline", "examples/synthetic.har", "-limit", "1", "examples/synthetic-regression.har"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(args, nil, &stdout, &stderr); code != 2 || stdout.Len() != 0 {
			t.Fatalf("unexpected result for %v: exit %d, stdout %q", args, code, &stdout)
		}
	}
}
