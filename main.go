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

	input := stdin
	if path := flags.Arg(0); path != "-" {
		file, err := os.Open(path)
		if err != nil {
			fmt.Fprintln(stderr, "har-timing-report: cannot open input file")
			return 1
		}
		defer file.Close()
		input = file
	}
	timingReport, err := report.Read(input, *order, *limit)
	if err != nil {
		fmt.Fprintln(stderr, "har-timing-report:", err)
		return 1
	}
	switch *format {
	case "json":
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		err = encoder.Encode(timingReport)
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
