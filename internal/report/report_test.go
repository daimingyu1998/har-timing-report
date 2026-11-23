package report

import (
	"strings"
	"testing"
)

func TestUnknownTimingsAndStatus(t *testing.T) {
	input := `{"log":{"entries":[
		{"startedDateTime":"","time":-1,"request":{"method":"PRIVATE_METHOD","url":"data:text/plain,private"},"response":{"status":0},"timings":{"dns":-1}},
		{"startedDateTime":"2026-01-01T12:00:00.000Z","time":0,"request":{"method":"GET","url":"https://example.test"},"response":{"status":404}}
	]}}`
	report, err := Read(strings.NewReader(input), "slowest", 0)
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.Origins != 1 || report.Summary.NoHTTPStatus != 1 || report.Summary.ClientErrors != 1 ||
		report.Summary.KnownDurations != 1 || *report.Summary.P50MS != 0 {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}
	if report.Requests[0].Ordinal != 2 || *report.Requests[0].StartMS != 0 {
		t.Fatalf("known zero time must sort before missing time: %+v", report.Requests[0])
	}
	unknown := report.Requests[1]
	if unknown.StartMS != nil || unknown.TotalMS != nil || unknown.Phases.DNSMS != nil ||
		unknown.Status != nil || unknown.Method != "OTHER" || unknown.Origin != "unknown" {
		t.Fatalf("unexpected missing-value representation: %+v", unknown)
	}
}

func TestReadRequiresOneValidArchive(t *testing.T) {
	for _, input := range []string{
		`{}`, `{"log":{"entries":null}}`, `{"log":{"entries":[null]}}`,
		`{"log":{"entries":[]}} {}`, `{"log":{"entries":{"0":{}}}}`,
	} {
		if _, err := Read(strings.NewReader(input), "slowest", 0); err == nil {
			t.Fatalf("accepted invalid archive %s", input)
		}
	}
	empty, err := Read(strings.NewReader("\xef\xbb\xbf"+`{"log":{"entries":[]}}`), "slowest", 0)
	if err != nil || empty.Summary.Requests != 0 || empty.Requests == nil {
		t.Fatalf("empty UTF-8 archive: %+v, %v", empty, err)
	}
}

func TestEntryLimit(t *testing.T) {
	input := `{"log":{"entries":[` + strings.Repeat("{},", maxEntries) + `{}]}}`
	if _, err := Read(strings.NewReader(input), "slowest", 0); err != errTooManyEntries {
		t.Fatalf("expected entry limit error, got %v", err)
	}
}
