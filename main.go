package main

import (
	"flag"
	"fmt"
	"os"
	"time"
)

func usage() {
	fmt.Fprintf(os.Stderr, `meguru - improvement-loop management CLI

Usage:
  meguru validate <file|dir>...
  meguru report [--format md|json] <file|dir>...
  meguru check [--actuals <jsonl>] [--stale-days N] [--now <date>] <file|dir>...

Commands:
  validate  Validate loop definition files
  report    Print a digest of loops (default format: md)
  check     Detect stale metrics, unrecorded results, and divergence from actuals
`)
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 2
	}
	switch args[0] {
	case "validate":
		return cmdValidate(args[1:])
	case "report":
		return cmdReport(args[1:])
	case "check":
		return cmdCheck(args[1:])
	case "-h", "--help", "help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", args[0])
		usage()
		return 2
	}
}

func cmdValidate(args []string) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "validate: at least one file or directory is required")
		return 2
	}
	loops, loadErrs, err := LoadLoops(fs.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, "validate:", err)
		return 2
	}
	failed := false
	// Report load errors in path order as validation problems.
	for path, lerr := range loadErrs {
		fmt.Printf("%s: %v\n", path, lerr)
		failed = true
	}
	for _, l := range loops {
		issues := ValidateLoop(l)
		if len(issues) == 0 {
			fmt.Printf("%s: OK\n", l.File)
			continue
		}
		failed = true
		for _, iss := range issues {
			fmt.Printf("%s: %s\n", l.File, iss)
		}
	}
	if failed {
		return 1
	}
	return 0
}

func cmdReport(args []string) int {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	format := fs.String("format", "md", "output format: md or json")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *format != "md" && *format != "json" {
		fmt.Fprintln(os.Stderr, "report: --format must be md or json")
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "report: at least one file or directory is required")
		return 2
	}
	loops, loadErrs, err := LoadLoops(fs.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, "report:", err)
		return 2
	}
	if len(loadErrs) > 0 {
		for path, lerr := range loadErrs {
			fmt.Fprintf(os.Stderr, "%s: %v\n", path, lerr)
		}
		return 1
	}
	if *format == "json" {
		if err := WriteReportJSON(os.Stdout, loops); err != nil {
			fmt.Fprintln(os.Stderr, "report:", err)
			return 1
		}
	} else {
		WriteReportMarkdown(os.Stdout, loops)
	}
	return 0
}

func cmdCheck(args []string) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	actualsPath := fs.String("actuals", "", "path to actuals JSONL file")
	staleDays := fs.Int("stale-days", 30, "days after which a metric is stale")
	nowStr := fs.String("now", "", "reference date (2006-01-02 or RFC3339); default today")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "check: at least one file or directory is required")
		return 2
	}
	now := time.Now()
	if *nowStr != "" {
		t, err := ParseDate(*nowStr)
		if err != nil {
			fmt.Fprintln(os.Stderr, "check: invalid --now:", err)
			return 2
		}
		now = t
	}
	var actuals map[string]*StageStats
	if *actualsPath != "" {
		f, err := os.Open(*actualsPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "check: cannot open actuals:", err)
			return 2
		}
		defer f.Close()
		records, warnings, err := ParseActuals(f)
		if err != nil {
			fmt.Fprintln(os.Stderr, "check: cannot read actuals:", err)
			return 2
		}
		for _, w := range warnings {
			fmt.Fprintln(os.Stderr, "warning:", w)
		}
		actuals = AggregateActuals(records)
	}
	loops, loadErrs, err := LoadLoops(fs.Args())
	if err != nil {
		fmt.Fprintln(os.Stderr, "check:", err)
		return 2
	}
	var findings []Finding
	for path, lerr := range loadErrs {
		findings = append(findings, Finding{File: path, Kind: "load_error", Message: lerr.Error()})
	}
	opts := CheckOptions{StaleDays: *staleDays, Now: now, Actuals: actuals}
	for _, l := range loops {
		findings = append(findings, CheckLoop(l, opts)...)
	}
	if len(findings) == 0 {
		fmt.Println("no findings")
		return 0
	}
	for _, f := range findings {
		fmt.Printf("%s [%s] %s: %s\n", f.File, f.Loop, f.Kind, f.Message)
	}
	return 1
}
