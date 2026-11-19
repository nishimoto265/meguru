package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func validLoop() *Loop {
	pm := 2
	return &Loop{
		Loop: "demo", Purpose: "improve things", UpdatedAt: "2026-11-15",
		CurrentStage: "stage-a", Cycles: 1,
		Metrics: []Metric{{Name: "m1", Current: "1", Target: "10", MeasuredBy: "ledger", Updated: "2026-11-15"}},
		Stages:  []Stage{{Name: "stage-a", Split: "unit", ParallelMax: &pm, Reason: "parallel helps"}},
		Improvements: []Improvement{
			{Date: "2026-11-10", Change: "did x", AimedAt: "m1", Result: "improved"},
		},
		NextCandidates: []string{"next idea"},
	}
}

func TestValidateValid(t *testing.T) {
	if issues := ValidateLoop(validLoop()); len(issues) != 0 {
		t.Fatalf("expected no issues, got %v", issues)
	}
	// parallel_note instead of parallel_max is valid
	l := validLoop()
	l.Stages[0].ParallelMax = nil
	l.Stages[0].ParallelNote = "varies by resource"
	if issues := ValidateLoop(l); len(issues) != 0 {
		t.Fatalf("parallel_note variant should be valid, got %v", issues)
	}
}

func TestValidateIssues(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Loop)
		want   string
	}{
		{"no loop name", func(l *Loop) { l.Loop = "" }, "loop name"},
		{"no purpose", func(l *Loop) { l.Purpose = "" }, "purpose"},
		{"metric no target", func(l *Loop) { l.Metrics[0].Target = "" }, "target"},
		{"metric no measured_by", func(l *Loop) { l.Metrics[0].MeasuredBy = "" }, "measured_by"},
		{"stage no split", func(l *Loop) { l.Stages[0].Split = "" }, "split"},
		{"stage no reason", func(l *Loop) { l.Stages[0].Reason = "" }, "reason"},
		{"stage no parallel", func(l *Loop) { l.Stages[0].ParallelMax = nil; l.Stages[0].ParallelNote = "" }, "parallel_max or parallel_note"},
		{"aimed_at unknown", func(l *Loop) { l.Improvements[0].AimedAt = "nope" }, "does not match"},
	}
	for _, c := range cases {
		l := validLoop()
		c.mutate(l)
		issues := ValidateLoop(l)
		found := false
		for _, iss := range issues {
			if strings.Contains(iss, c.want) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: expected issue containing %q, got %v", c.name, c.want, issues)
		}
	}
}

func TestCheckStale(t *testing.T) {
	now, _ := ParseDate("2026-11-15")
	l := validLoop()
	// Fresh: updated same day -> no stale finding
	if fs := CheckLoop(l, CheckOptions{StaleDays: 30, Now: now}); len(fs) != 0 {
		t.Fatalf("fresh loop should have no findings, got %v", fs)
	}
	// Stale via metric updated
	l.Metrics[0].Updated = "2026-09-01"
	fs := CheckLoop(l, CheckOptions{StaleDays: 30, Now: now})
	if len(fs) != 1 || fs[0].Kind != "stale_metric" {
		t.Fatalf("expected 1 stale finding, got %v", fs)
	}
	// Fallback to loop updated_at when metric updated empty; RFC3339 also parses
	l2 := validLoop()
	l2.Metrics[0].Updated = ""
	l2.UpdatedAt = "2026-09-01T00:00:00Z"
	fs = CheckLoop(l2, CheckOptions{StaleDays: 30, Now: now})
	if len(fs) != 1 || fs[0].Kind != "stale_metric" {
		t.Fatalf("expected stale via fallback/RFC3339, got %v", fs)
	}
}

func TestCheckMissingResult(t *testing.T) {
	now, _ := ParseDate("2026-11-15")
	l := validLoop()
	l.Improvements[0].Result = ""
	fs := CheckLoop(l, CheckOptions{StaleDays: 30, Now: now})
	if len(fs) != 1 || fs[0].Kind != "missing_result" {
		t.Fatalf("expected missing_result, got %v", fs)
	}
}

func TestCheckParallelExceeded(t *testing.T) {
	now, _ := ParseDate("2026-11-15")
	// 3 overlapping runs on stage-a (parallel_max=2) -> exceeded
	jsonl := strings.Join([]string{
		`{"stage":"stage-a","started_at":"2026-11-15T09:00:00Z","finished_at":"2026-11-15T09:10:00Z","status":"ok"}`,
		`{"stage":"stage-a","started_at":"2026-11-15T09:01:00Z","finished_at":"2026-11-15T09:10:00Z","status":"ok"}`,
		`{"stage":"stage-a","started_at":"2026-11-15T09:02:00Z","finished_at":"2026-11-15T09:10:00Z","status":"ok"}`,
	}, "\n")
	recs, warns, err := ParseActuals(strings.NewReader(jsonl))
	if err != nil || len(warns) != 0 {
		t.Fatalf("parse: err=%v warns=%v", err, warns)
	}
	stats := AggregateActuals(recs)
	if stats["stage-a"].MaxConcurrency != 3 {
		t.Fatalf("expected max concurrency 3, got %d", stats["stage-a"].MaxConcurrency)
	}
	l := validLoop()
	fs := CheckLoop(l, CheckOptions{StaleDays: 30, Now: now, Actuals: stats})
	found := false
	for _, f := range fs {
		if f.Kind == "parallel_exceeded" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected parallel_exceeded, got %v", fs)
	}
	// Within limit -> no finding
	jsonl2 := `{"stage":"stage-a","started_at":"2026-11-15T09:00:00Z","finished_at":"2026-11-15T09:10:00Z","status":"ok"}`
	recs2, _, _ := ParseActuals(strings.NewReader(jsonl2))
	fs = CheckLoop(l, CheckOptions{StaleDays: 30, Now: now, Actuals: AggregateActuals(recs2)})
	if len(fs) != 0 {
		t.Fatalf("within limit should have no findings, got %v", fs)
	}
	// Stage without parallel_max is not compared
	l2 := validLoop()
	l2.Stages[0].ParallelMax = nil
	l2.Stages[0].ParallelNote = "note"
	fs = CheckLoop(l2, CheckOptions{StaleDays: 30, Now: now, Actuals: stats})
	if len(fs) != 0 {
		t.Fatalf("no parallel_max should skip comparison, got %v", fs)
	}
}

func TestActualsFieldVariationAndAggregation(t *testing.T) {
	jsonl := strings.Join([]string{
		`{"stage_name":"collect","start":"2026-11-15T09:00:00Z","end":"2026-11-15T09:10:00Z","success":true}`,
		`{"name":"collect","begin":"2026-11-15T09:05:00Z","finish":"2026-11-15T09:15:00Z","ok":false}`,
		`{"stage":"collect","started_at":"2026-11-15T10:00:00Z","finished_at":"2026-11-15T10:20:00Z","status":"error"}`,
		`not json at all`,
		`{"started_at":"2026-11-15T10:00:00Z"}`, // no stage -> skipped
		``,
	}, "\n")
	recs, warns, err := ParseActuals(strings.NewReader(jsonl))
	if err != nil {
		t.Fatal(err)
	}
	if len(recs) != 3 {
		t.Fatalf("expected 3 records, got %d", len(recs))
	}
	if len(warns) != 2 {
		t.Fatalf("expected 2 warnings, got %v", warns)
	}
	stats := AggregateActuals(recs)["collect"]
	if stats.Runs != 3 || stats.Failures != 2 {
		t.Fatalf("runs/failures = %d/%d, want 3/2", stats.Runs, stats.Failures)
	}
	if stats.MaxConcurrency != 2 {
		t.Fatalf("max concurrency = %d, want 2", stats.MaxConcurrency)
	}
	// durations: 600, 600, 1200 -> avg 800
	if stats.AvgDurationSeconds != 800 {
		t.Fatalf("avg duration = %v, want 800", stats.AvgDurationSeconds)
	}
	if stats.FailureRate != 2.0/3.0 {
		t.Fatalf("failure rate = %v", stats.FailureRate)
	}
}

func TestReportMarkdownAndJSON(t *testing.T) {
	l := validLoop()
	l.Improvements = append(l.Improvements,
		Improvement{Date: "2026-11-11", Change: "did y", AimedAt: "m1", Result: "ok"},
		Improvement{Date: "2026-11-12", Change: "did z", AimedAt: "m1", Result: "ok"},
		Improvement{Date: "2026-11-13", Change: "did w", AimedAt: "m1", Result: "ok"},
	)
	var md bytes.Buffer
	WriteReportMarkdown(&md, []*Loop{l})
	for _, want := range []string{"## demo", "improve things", "stage-a", "m1: 1 → 10", "next idea"} {
		if !strings.Contains(md.String(), want) {
			t.Errorf("markdown missing %q:\n%s", want, md.String())
		}
	}
	// Recent improvements capped at 3
	if strings.Contains(md.String(), "did x") {
		t.Errorf("markdown should cap recent improvements at 3, oldest dropped:\n%s", md.String())
	}
	var js bytes.Buffer
	if err := WriteReportJSON(&js, []*Loop{l}); err != nil {
		t.Fatal(err)
	}
	var digests []ReportDigest
	if err := json.Unmarshal(js.Bytes(), &digests); err != nil {
		t.Fatalf("json invalid: %v\n%s", err, js.String())
	}
	if len(digests) != 1 || digests[0].Loop != "demo" || len(digests[0].RecentImprove) != 3 {
		t.Fatalf("unexpected digests: %+v", digests)
	}
}

func TestParseDate(t *testing.T) {
	for _, s := range []string{"2026-11-15", "2026-11-15T09:00:00+09:00", "2026-11-15T09:00:00Z"} {
		if _, err := ParseDate(s); err != nil {
			t.Errorf("ParseDate(%q): %v", s, err)
		}
	}
	if _, err := ParseDate("15/11/2026"); err == nil {
		t.Error("expected error for unsupported format")
	}
	_ = time.Now
}
