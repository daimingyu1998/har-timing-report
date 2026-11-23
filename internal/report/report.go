// Package report extracts a small, shareable timing report from an HTTP Archive.
package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/url"
	"slices"
	"strings"
	"time"
)

const (
	maxCaptureBytes = 64 << 20
	maxEntries      = 100_000
)

var errTooManyEntries = errors.New("HAR contains more than 100,000 request entries")

// These input types deliberately omit fields such as headers, bodies, cookies,
// and extension metadata. Only Report and its component types are rendered.
type archive struct {
	Log struct {
		Entries entries `json:"entries"`
	} `json:"log"`
}

type entries []*entry

func (items *entries) UnmarshalJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	start, err := decoder.Token()
	if err != nil || start != json.Delim('[') {
		return errors.New("HAR log.entries must be an array")
	}

	result := make(entries, 0)
	for decoder.More() {
		if len(result) >= maxEntries {
			return errTooManyEntries
		}
		var item *entry
		if err := decoder.Decode(&item); err != nil || item == nil {
			return errors.New("invalid HAR request entry")
		}
		result = append(result, item)
	}
	if _, err := decoder.Token(); err != nil {
		return errors.New("invalid HAR request array")
	}
	*items = result
	return nil
}

type entry struct {
	Start   string   `json:"startedDateTime"`
	Total   *float64 `json:"time"`
	Request struct {
		Method string `json:"method"`
		URL    string `json:"url"`
	} `json:"request"`
	Response struct {
		Status int `json:"status"`
	} `json:"response"`
	Timings struct {
		Blocked *float64 `json:"blocked"`
		DNS     *float64 `json:"dns"`
		Connect *float64 `json:"connect"`
		TLS     *float64 `json:"ssl"`
		Send    *float64 `json:"send"`
		Wait    *float64 `json:"wait"`
		Receive *float64 `json:"receive"`
	} `json:"timings"`
}

// Report contains only rounded timings, fixed labels, counts, and aliases.
type Report struct {
	SchemaVersion int       `json:"schema_version"`
	Order         string    `json:"order"`
	Summary       Summary   `json:"summary"`
	Requests      []Request `json:"requests"`
}

type Summary struct {
	Requests       int    `json:"requests"`
	Origins        int    `json:"origins"`
	ClientErrors   int    `json:"http_4xx"`
	ServerErrors   int    `json:"http_5xx"`
	NoHTTPStatus   int    `json:"no_http_status"`
	KnownDurations int    `json:"known_durations"`
	P50MS          *int64 `json:"p50_ms"`
	P95MS          *int64 `json:"p95_ms"`
}

type Request struct {
	Ordinal  int    `json:"ordinal"`
	Origin   string `json:"origin"`
	Endpoint string `json:"endpoint"`
	Method   string `json:"method"`
	Status   *int   `json:"status"`
	StartMS  *int64 `json:"start_ms"`
	TotalMS  *int64 `json:"total_ms"`
	Phases   Phases `json:"phases"`
}

type Phases struct {
	BlockedMS *int64 `json:"blocked_ms"`
	DNSMS     *int64 `json:"dns_ms"`
	ConnectMS *int64 `json:"connect_ms"`
	TLSMS     *int64 `json:"tls_ms"`
	SendMS    *int64 `json:"send_ms"`
	WaitMS    *int64 `json:"wait_ms"`
	ReceiveMS *int64 `json:"receive_ms"`
}

// Read converts a HAR into a report. Its errors never quote capture content.
func Read(input io.Reader, order string, limit int) (Report, error) {
	if (order != "slowest" && order != "start") || limit < 0 {
		return Report{}, errors.New("invalid report options")
	}
	data, err := io.ReadAll(io.LimitReader(input, maxCaptureBytes+1))
	if err != nil {
		return Report{}, errors.New("cannot read input")
	}
	if len(data) > maxCaptureBytes {
		return Report{}, errors.New("capture exceeds the 64 MiB input limit")
	}
	var inputArchive archive
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	err = json.Unmarshal(data, &inputArchive)
	if errors.Is(err, errTooManyEntries) {
		return Report{}, err
	}
	if err != nil || inputArchive.Log.Entries == nil {
		return Report{}, errors.New("expected a valid HAR JSON document with a log.entries array")
	}
	return build(inputArchive.Log.Entries, order, limit), nil
}

func build(entries []*entry, order string, limit int) Report {
	result := Report{
		SchemaVersion: 1,
		Order:         order,
		Summary:       Summary{Requests: len(entries)},
		Requests:      make([]Request, 0, len(entries)),
	}
	origins := make(map[string]string)
	endpoints := make(map[endpointKey]string)
	durations := make([]int64, 0, len(entries))
	starts := make([]time.Time, len(entries))
	var earliest time.Time
	for index, item := range entries {
		start, err := time.Parse(time.RFC3339Nano, item.Start)
		if err == nil && !start.IsZero() {
			starts[index] = start
			if earliest.IsZero() || start.Before(earliest) {
				earliest = start
			}
		}
	}

	for index, item := range entries {
		origin, endpoint := aliases(item.Request.URL, origins, endpoints)
		row := Request{
			Ordinal: index + 1, Origin: origin, Endpoint: endpoint,
			Method: method(item.Request.Method), TotalMS: millis(item.Total),
			Phases: Phases{
				BlockedMS: millis(item.Timings.Blocked), DNSMS: millis(item.Timings.DNS),
				ConnectMS: millis(item.Timings.Connect), TLSMS: millis(item.Timings.TLS),
				SendMS: millis(item.Timings.Send), WaitMS: millis(item.Timings.Wait),
				ReceiveMS: millis(item.Timings.Receive),
			},
		}
		if start := starts[index]; !start.IsZero() {
			offset := int64(math.Round(start.Sub(earliest).Seconds() * 1000))
			row.StartMS = &offset
		}
		status := item.Response.Status
		if status >= 100 && status <= 599 {
			row.Status = &status
		}
		switch {
		case row.Status == nil:
			result.Summary.NoHTTPStatus++
		case status >= 500:
			result.Summary.ServerErrors++
		case status >= 400:
			result.Summary.ClientErrors++
		}
		if row.TotalMS != nil {
			durations = append(durations, *row.TotalMS)
		}
		result.Requests = append(result.Requests, row)
	}
	result.Summary.Origins = len(origins)
	result.Summary.KnownDurations = len(durations)
	slices.Sort(durations)
	result.Summary.P50MS = percentile(durations, 50)
	result.Summary.P95MS = percentile(durations, 95)

	slices.SortStableFunc(result.Requests, func(a, b Request) int {
		if order == "start" {
			return compareTimings(a.StartMS, b.StartMS, false)
		}
		return compareTimings(a.TotalMS, b.TotalMS, true)
	})
	if limit > 0 && len(result.Requests) > limit {
		result.Requests = result.Requests[:limit]
	}
	return result
}

type endpointKey struct {
	origin string
	path   string
}

func aliases(rawURL string, origins map[string]string, endpoints map[endpointKey]string) (string, string) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return "unknown", "unknown"
	}
	host, port := strings.ToLower(u.Hostname()), u.Port()
	if (u.Scheme == "http" && port == "80") || (u.Scheme == "https" && port == "443") {
		port = ""
	}
	// Keep grouping local to this report. The origin key is never serialized.
	originKey := u.Scheme + "|" + host + "|" + port
	origin, exists := origins[originKey]
	if !exists {
		origin = fmt.Sprintf("origin-%d", len(origins)+1)
		origins[originKey] = origin
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	key := endpointKey{origin: origin, path: path}
	endpoint, exists := endpoints[key]
	if !exists {
		endpoint = fmt.Sprintf("endpoint-%d", len(endpoints)+1)
		endpoints[key] = endpoint
	}
	return origin, endpoint
}

func millis(value *float64) *int64 {
	// Keep rounded millisecond values precise in JSON readers using float64.
	if value == nil || *value < 0 || *value >= (1<<53)-1 {
		return nil
	}
	rounded := int64(math.Round(*value))
	return &rounded
}

func method(raw string) string {
	switch raw {
	case "GET", "HEAD", "POST", "PUT", "DELETE", "CONNECT", "OPTIONS", "TRACE", "PATCH":
		return raw
	default:
		return "OTHER"
	}
}

func percentile(sorted []int64, percent int) *int64 {
	if len(sorted) == 0 {
		return nil
	}
	rank := int(math.Ceil(float64(len(sorted)) * float64(percent) / 100))
	value := sorted[rank-1]
	return &value
}

func compareTimings(a, b *int64, descending bool) int {
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return 1
	}
	if b == nil {
		return -1
	}
	if descending {
		a, b = b, a
	}
	switch {
	case *a < *b:
		return -1
	case *a > *b:
		return 1
	default:
		return 0
	}
}
