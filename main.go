package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/daimingyu1998/har-timing-report/internal/report"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("har-timing-report", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	format := flags.String("format", "markdown", "markdown, json, or html")
	limit := flags.Int("limit", 15, "maximum requests to show (0 for all)")
	order := flags.String("order", "slowest", "slowest or start")
	baseline := flags.String("baseline", "", "compare with this HAR (markdown or json)")
	usage := func(w io.Writer) {
		fmt.Fprintln(w, "Usage: har-timing-report [options] <capture.har|->")
		flags.SetOutput(w)
		flags.PrintDefaults()
	}

	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		usage(stdout)
		return 0
	} else if err != nil {
		// Flag errors may quote user-provided text, including a local path.
		usage(stderr)
		return 2
	}
	if flags.NArg() != 1 || *limit < 0 ||
		(*format != "markdown" && *format != "json" && *format != "html") ||
		(*order != "slowest" && *order != "start") {
		usage(stderr)
		return 2
	}

	path := flags.Arg(0)
	if *baseline != "" {
		if *format == "html" || (*baseline == "-" && path == "-") ||
			*order != "slowest" || *limit != 15 {
			fmt.Fprintln(stderr, "har-timing-report: comparison needs markdown or json, default row options, and at most one stdin capture")
			usage(stderr)
			return 2
		}
		return runComparison(*baseline, path, *format, stdin, stdout, stderr)
	}
	timingReport, err := readCapture(path, stdin, *order, *limit)
	if err != nil {
		fmt.Fprintln(stderr, "har-timing-report:", err)
		return 1
	}
	switch *format {
	case "json":
		err = writeJSON(stdout, timingReport)
	case "html":
		err = timingReport.WriteHTML(stdout)
	default:
		err = timingReport.WriteMarkdown(stdout)
	}
	if err != nil {
		fmt.Fprintln(stderr, "har-timing-report: cannot write report")
		return 1
	}
	return 0
}

func readCapture(path string, stdin io.Reader, order string, limit int) (report.Report, error) {
	if path == "-" {
		return report.Read(stdin, order, limit)
	}
	file, err := os.Open(path)
	if err != nil {
		return report.Report{}, errors.New("cannot open input file")
	}
	defer file.Close()
	return report.Read(file, order, limit)
}

func writeJSON(out io.Writer, value any) error {
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func runComparison(baselinePath, currentPath, format string, stdin io.Reader, stdout, stderr io.Writer) int {
	baseline, err := readCapture(baselinePath, stdin, "slowest", 0)
	if err != nil {
		fmt.Fprintln(stderr, "har-timing-report: baseline capture:", err)
		return 1
	}
	current, err := readCapture(currentPath, stdin, "slowest", 0)
	if err != nil {
		fmt.Fprintln(stderr, "har-timing-report: current capture:", err)
		return 1
	}
	comparison := report.Compare(baseline, current)
	if format == "json" {
		err = writeJSON(stdout, comparison)
	} else {
		err = comparison.WriteMarkdown(stdout)
	}
	if err != nil {
		fmt.Fprintln(stderr, "har-timing-report: cannot write comparison")
		return 1
	}
	return 0
}
