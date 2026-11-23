package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/daimingyu1998/har-timing-report/internal/report"
)

func TestCLIProducesShareableReports(t *testing.T) {
	for _, format := range []string{"markdown", "json", "html"} {
		t.Run(format, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run([]string{"-format", format, "examples/synthetic.har"}, nil, &stdout, &stderr)
			if code != 0 {
				t.Fatalf("exit %d: %s", code, &stderr)
			}
			for _, private := range []string{
				"app.example.test", "api.example.test", "demo-id", "example-query",
				"example-retry", "example-credential", "example-private-body", "2026-01-01",
			} {
				if strings.Contains(stdout.String(), private) || strings.Contains(stderr.String(), private) {
					t.Fatalf("raw capture value leaked: %q", private)
				}
			}
			switch format {
			case "markdown":
				if !strings.Contains(stdout.String(), "| 2 | origin-2 | endpoint-2 | GET | 503 | 60 | 842 | 794 |") {
					t.Fatalf("slow request missing: %s", &stdout)
				}
				return
			case "html":
				output := stdout.String()
				if !strings.Contains(output, `class="bar error" style="left: `) ||
					!strings.Contains(output, `default-src 'none'`) ||
					strings.Contains(output, "ZgotmplZ") {
					t.Fatal("invalid offline waterfall")
				}
				return
			}
			var parsed report.Report
			if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
				t.Fatal(err)
			}
			if parsed.Summary.Requests != 3 || parsed.Summary.ServerErrors != 1 || len(parsed.Requests) != 3 ||
				parsed.Requests[0].Endpoint != parsed.Requests[1].Endpoint ||
				parsed.Requests[0].Phases.TLSMS == nil ||
				*parsed.Requests[0].Phases.TLSMS != 21 {
				t.Fatalf("timing or retry information missing: %+v", parsed)
			}
		})
	}
}

func TestCLIRejectsInvalidInputWithoutQuotingIt(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-"}, strings.NewReader(`{"log":{"entries":["private-content"]}}`), &stdout, &stderr)
	if code != 1 || stdout.Len() != 0 || strings.Contains(stderr.String(), "private-content") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, &stdout, &stderr)
	}
}

func TestCLIReadsStdinAndLimitsRows(t *testing.T) {
	input, err := os.ReadFile("examples/synthetic.har")
	if err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := run([]string{"-format", "json", "-order", "start", "-limit", "1", "-"}, bytes.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, &stderr)
	}
	var parsed report.Report
	if err := json.Unmarshal(stdout.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Requests) != 1 || parsed.Requests[0].Ordinal != 1 || parsed.Summary.Requests != 3 {
		t.Fatalf("wrong row order or capture summary: %+v", parsed)
	}
}
