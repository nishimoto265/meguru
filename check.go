package main

import (
	"fmt"
	"time"
)

// Finding is one check detection.
type Finding struct {
	File    string `json:"file,omitempty"`
	Loop    string `json:"loop,omitempty"`
	Kind    string `json:"kind"` // stale_metric | missing_result | parallel_exceeded
	Message string `json:"message"`
}

type CheckOptions struct {
	StaleDays int
	Now       time.Time
	Actuals   map[string]*StageStats // nil = skip divergence check
}

// CheckLoop runs stale / missing-result / divergence checks on one loop.
func CheckLoop(l *Loop, opts CheckOptions) []Finding {
	var findings []Finding
	add := func(kind, format string, args ...any) {
		findings = append(findings, Finding{
			File: l.File, Loop: l.Loop, Kind: kind,
			Message: fmt.Sprintf(format, args...),
		})
	}
	// a. stale metrics
	for _, m := range l.Metrics {
		dateStr := m.Updated
		if dateStr == "" {
			dateStr = l.UpdatedAt
		}
		if dateStr == "" {
			continue // no date to judge
		}
		t, err := ParseDate(dateStr)
		if err != nil {
			continue // unparseable dates are not judged here
		}
		// Compare by calendar days.
		nowDay := time.Date(opts.Now.Year(), opts.Now.Month(), opts.Now.Day(), 0, 0, 0, 0, time.UTC)
		updDay := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		days := int(nowDay.Sub(updDay).Hours() / 24)
		if days > opts.StaleDays {
			add("stale_metric", "metric %q is stale: updated %s is %d days before %s (stale-days=%d)", m.Name, dateStr, days, nowDay.Format("2006-01-02"), opts.StaleDays)
		}
	}
	// b. improvements without result
	for i, imp := range l.Improvements {
		if imp.Result == "" {
			label := imp.Change
			if label == "" {
				label = fmt.Sprintf("improvements[%d]", i)
			}
			add("missing_result", "improvement (%s, %s) has no result recorded", imp.Date, label)
		}
	}
	// c. divergence from actuals
	if opts.Actuals != nil {
		for _, s := range l.Stages {
			if s.ParallelMax == nil {
				continue
			}
			stats, ok := opts.Actuals[s.Name]
			if !ok {
				continue
			}
			if stats.MaxConcurrency > *s.ParallelMax {
				add("parallel_exceeded", "stage %q max concurrency %d exceeds designed parallel_max %d", s.Name, stats.MaxConcurrency, *s.ParallelMax)
			}
		}
	}
	return findings
}
