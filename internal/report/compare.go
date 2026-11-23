package report

import (
	"math"
	"slices"
)

// Comparison contains capture-level aggregates. No request rows or aliases
// are included, because aliases from different captures are unrelated.
type Comparison struct {
	SchemaVersion    int                `json:"schema_version"`
	BaselineRequests int                `json:"baseline_requests"`
	CurrentRequests  int                `json:"current_requests"`
	Rates            []RateComparison   `json:"rates"`
	Timings          []TimingComparison `json:"timings"`
}

type RateComparison struct {
	Metric   string   `json:"metric"`
	Baseline Rate     `json:"baseline"`
	Current  Rate     `json:"current"`
	ChangePP *float64 `json:"change_percentage_points"`
}

type Rate struct {
	Count   int      `json:"count"`
	Percent *float64 `json:"percent"`
}

type TimingComparison struct {
	Metric      string      `json:"metric"`
	Baseline    TimingStats `json:"baseline"`
	Current     TimingStats `json:"current"`
	ChangeP50MS *int64      `json:"change_p50_ms"`
	ChangeP95MS *int64      `json:"change_p95_ms"`
}

type TimingStats struct {
	Samples int    `json:"samples"`
	P50MS   *int64 `json:"p50_ms"`
	P95MS   *int64 `json:"p95_ms"`
}

// Compare compares complete reports produced with no row limit.
func Compare(baseline, current Report) Comparison {
	before, after := baseline.Summary, current.Summary
	result := Comparison{
		SchemaVersion:    1,
		BaselineRequests: before.Requests,
		CurrentRequests:  after.Requests,
		Rates: []RateComparison{
			compareRate("http_4xx", before.ClientErrors, before.Requests, after.ClientErrors, after.Requests),
			compareRate("http_5xx", before.ServerErrors, before.Requests, after.ServerErrors, after.Requests),
			compareRate("no_http_status", before.NoHTTPStatus, before.Requests, after.NoHTTPStatus, after.Requests),
		},
		Timings: []TimingComparison{
			compareTiming("total",
				TimingStats{Samples: before.KnownDurations, P50MS: before.P50MS, P95MS: before.P95MS},
				TimingStats{Samples: after.KnownDurations, P50MS: after.P50MS, P95MS: after.P95MS}),
		},
	}
	for _, phase := range []struct {
		name string
		pick func(Phases) *int64
	}{
		{"blocked", func(p Phases) *int64 { return p.BlockedMS }},
		{"dns", func(p Phases) *int64 { return p.DNSMS }},
		{"connect", func(p Phases) *int64 { return p.ConnectMS }},
		{"tls", func(p Phases) *int64 { return p.TLSMS }},
		{"send", func(p Phases) *int64 { return p.SendMS }},
		{"wait", func(p Phases) *int64 { return p.WaitMS }},
		{"receive", func(p Phases) *int64 { return p.ReceiveMS }},
	} {
		result.Timings = append(result.Timings, compareTiming(
			phase.name, phaseStats(baseline.Requests, phase.pick), phaseStats(current.Requests, phase.pick),
		))
	}
	return result
}

func compareRate(name string, before, beforeTotal, after, afterTotal int) RateComparison {
	result := RateComparison{
		Metric:   name,
		Baseline: Rate{Count: before},
		Current:  Rate{Count: after},
	}
	if beforeTotal > 0 {
		value := roundedPercent(float64(before) / float64(beforeTotal))
		result.Baseline.Percent = &value
	}
	if afterTotal > 0 {
		value := roundedPercent(float64(after) / float64(afterTotal))
		result.Current.Percent = &value
	}
	if beforeTotal > 0 && afterTotal > 0 {
		value := roundedPercent(float64(after)/float64(afterTotal) - float64(before)/float64(beforeTotal))
		result.ChangePP = &value
	}
	return result
}

func roundedPercent(fraction float64) float64 {
	rounded := math.Round(fraction * 10000)
	if rounded == 0 {
		return 0
	}
	return rounded / 100
}

func phaseStats(requests []Request, pick func(Phases) *int64) TimingStats {
	values := make([]int64, 0, len(requests))
	for _, request := range requests {
		if value := pick(request.Phases); value != nil {
			values = append(values, *value)
		}
	}
	slices.Sort(values)
	return TimingStats{Samples: len(values), P50MS: percentile(values, 50), P95MS: percentile(values, 95)}
}

func compareTiming(name string, before, after TimingStats) TimingComparison {
	return TimingComparison{
		Metric: name, Baseline: before, Current: after,
		ChangeP50MS: difference(before.P50MS, after.P50MS),
		ChangeP95MS: difference(before.P95MS, after.P95MS),
	}
}

func difference(before, after *int64) *int64 {
	if before == nil || after == nil {
		return nil
	}
	change := *after - *before
	return &change
}
