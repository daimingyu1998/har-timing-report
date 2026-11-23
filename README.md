# HAR Timing Report

Turn a browser HTTP Archive (HAR) into a Markdown bug report, JSON timing summary, or offline HTML waterfall. Share which requests were slow or failed while keeping raw URLs, headers, and payloads out of the report.

Chrome's [sanitized HAR export](https://developer.chrome.com/docs/devtools/network/reference/#save-as-har) omits certain sensitive headers. A HAR can still contain private paths, query parameters, or payloads. HAR Timing Report produces a separate, smaller report from a fixed set of fields.

## Quick start

Build with Go 1.22 or newer.

```sh
git clone https://github.com/daimingyu1998/har-timing-report.git
cd har-timing-report
go build .

./har-timing-report examples/synthetic.har
./har-timing-report -order start -limit 0 capture.har > timing-report.md
./har-timing-report -format json -limit 0 - < capture.har > timing-report.json
./har-timing-report -format html -order start capture.har > timing-report.html
```

The bundled synthetic capture includes a slow 503 followed by a successful retry. The rows for both requests share an endpoint alias, so a maintainer can spot the retry without seeing the URL.

| # | Origin | Endpoint | Method | Status | Start (ms) | Total (ms) | Wait (ms) |
| ---: | --- | --- | --- | ---: | ---: | ---: | ---: |
| 2 | origin-2 | endpoint-2 | GET | 503 | 60 | 842 | 794 |
| 3 | origin-2 | endpoint-2 | GET | 200 | 1050 | 122 | 116 |
| 1 | origin-1 | endpoint-1 | GET | 200 | 0 | 56 | 32 |

The default view shows the 15 slowest requests. Use `-order start` for chronological order and `-limit 0` for every request. Summary counts and nearest-rank p50 and p95 durations describe the full capture. Missing durations sort last and are excluded from the percentiles.

## What's in the report

- One-based request numbers, origin and endpoint aliases, standard HTTP methods, and response status codes. Endpoints group requests to the same encoded path on the same HTTP origin, ignoring the query and fragment.
- Relative start times and total request durations, rounded to whole milliseconds. JSON also includes blocked, DNS, connect, TLS, send, wait, and receive phases. In [HAR timing semantics](https://w3c.github.io/web-performance/specs/HAR/Overview.html), the TLS phase may already be part of connect time.
- Missing or unsupported timings as `null` in JSON and a dash in Markdown. If the browser recorded status 0, the report counts it as a missing HTTP status.

The output contains no hostnames, paths, headers, cookies, request or response bodies, browser extension fields, or absolute timestamps. Aliases are assigned in capture order and reset for each report. The program makes no network requests and uses only the Go standard library. The standalone HTML waterfall needs no JavaScript or remote assets. Input is capped at 64 MiB and 100,000 entries. JSON output declares `schema_version: 1`.

Review the generated report before sharing. Request order, status codes, relative timings, and repeated endpoint patterns can still reveal activity. Keep original captures local. The report omits the data needed to replay requests or reimport the capture into DevTools.

## Compare two captures

```sh
./har-timing-report -baseline examples/synthetic.har examples/synthetic-regression.har
./har-timing-report -baseline baseline.har -format json current.har > comparison.json
```

Comparison output contains capture-wide status counts and rates, plus p50 and p95 for the total request duration and each available timing phase. Changes are current minus baseline, in milliseconds for durations and percentage points for response rates. The sample shows p95 increasing from 842 ms to 1,442 ms and the 5xx rate increasing from 33.33% to 50.00%.

Comparisons include sample counts and exclude missing measurements. A capture with zero requests has undefined response rates. Compare captures recorded under similar conditions, since traffic mix and cache state affect aggregate metrics. Comparisons work with Markdown or JSON and leave out request rows and aliases. Use `-` for at most one capture to read it from standard input.

## Development

```sh
go test ./...
go vet ./...
```

Use synthetic captures when reporting bugs.

Licensed under [MIT](LICENSE).
