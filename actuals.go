package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// ActualRecord is one normalized execution record from a JSONL actuals file.
type ActualRecord struct {
	Stage  string
	Start  time.Time
	End    time.Time
	HasInt bool // has both start and end
	Failed bool
}

// StageStats is the per-stage aggregate of actual records.
type StageStats struct {
	Stage              string  `json:"stage"`
	Runs               int     `json:"runs"`
	Failures           int     `json:"failures"`
	FailureRate        float64 `json:"failure_rate"`
	AvgDurationSeconds float64 `json:"avg_duration_seconds"`
	MaxConcurrency     int     `json:"max_concurrency"`
}

func firstString(m map[string]json.RawMessage, keys ...string) (string, bool) {
	for _, k := range keys {
		raw, ok := m[k]
		if !ok {
			continue
		}
		var s string
		if err := json.Unmarshal(raw, &s); err == nil {
			return s, true
		}
	}
	return "", false
}

func firstBool(m map[string]json.RawMessage, keys ...string) (bool, bool) {
	for _, k := range keys {
		raw, ok := m[k]
		if !ok {
			continue
		}
		var b bool
		if err := json.Unmarshal(raw, &b); err == nil {
			return b, true
		}
	}
	return false, false
}

func parseTimeField(m map[string]json.RawMessage, keys ...string) (time.Time, bool) {
	s, ok := firstString(m, keys...)
	if !ok || strings.TrimSpace(s) == "" {
		return time.Time{}, false
	}
	t, err := ParseDate(s)
	if err != nil {
		// Also try epoch seconds.
		return time.Time{}, false
	}
	return t, true
}

func isFailureStatus(s string) (failed bool, known bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "ok", "success", "succeeded", "successful", "passed", "pass", "done", "completed":
		return false, true
	case "failed", "failure", "error", "fail", "ng":
		return true, true
	}
	return false, false
}

// ParseActuals reads JSONL records, tolerating field-name variation.
// Unreadable lines produce warnings and are skipped.
func ParseActuals(r io.Reader) (records []ActualRecord, warnings []string, err error) {
	sc := bufio.NewScanner(r)
	// Allow long lines.
	sc.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var m map[string]json.RawMessage
		if jerr := json.Unmarshal([]byte(line), &m); jerr != nil {
			warnings = append(warnings, fmt.Sprintf("actuals line %d: invalid JSON, skipped: %v", lineNo, jerr))
			continue
		}
		stage, ok := firstString(m, "stage", "stage_name", "name")
		if !ok || strings.TrimSpace(stage) == "" {
			warnings = append(warnings, fmt.Sprintf("actuals line %d: no stage name, skipped", lineNo))
			continue
		}
		rec := ActualRecord{Stage: stage}
		start, hasStart := parseTimeField(m, "started_at", "start", "begin")
		end, hasEnd := parseTimeField(m, "finished_at", "end", "finish")
		rec.Start, rec.End = start, end
		rec.HasInt = hasStart && hasEnd
		// Failure determination: status string wins, then success bool, then ok bool.
		if st, ok := firstString(m, "status"); ok {
			if f, known := isFailureStatus(st); known {
				rec.Failed = f
			}
		} else if b, ok := firstBool(m, "success"); ok {
			rec.Failed = !b
		} else if b, ok := firstBool(m, "ok"); ok {
			rec.Failed = !b
		}
		records = append(records, rec)
	}
	if serr := sc.Err(); serr != nil {
		return records, warnings, serr
	}
	return records, warnings, nil
}

// AggregateActuals aggregates records per stage.
func AggregateActuals(records []ActualRecord) map[string]*StageStats {
	byStage := map[string][]ActualRecord{}
	for _, rec := range records {
		byStage[rec.Stage] = append(byStage[rec.Stage], rec)
	}
	out := map[string]*StageStats{}
	for stage, recs := range byStage {
		st := &StageStats{Stage: stage, Runs: len(recs)}
		var totalDur float64
		var durCount int
		type ev struct {
			t     time.Time
			delta int // -1 for end, +1 for start; ends sort before starts at same time
		}
		var events []ev
		for _, rec := range recs {
			if rec.Failed {
				st.Failures++
			}
			if rec.HasInt {
				d := rec.End.Sub(rec.Start).Seconds()
				if d < 0 {
					d = 0
				}
				totalDur += d
				durCount++
				events = append(events, ev{rec.Start, +1}, ev{rec.End, -1})
			}
		}
		if st.Runs > 0 {
			st.FailureRate = float64(st.Failures) / float64(st.Runs)
		}
		if durCount > 0 {
			st.AvgDurationSeconds = totalDur / float64(durCount)
		}
		sort.Slice(events, func(i, j int) bool {
			if events[i].t.Equal(events[j].t) {
				return events[i].delta < events[j].delta
			}
			return events[i].t.Before(events[j].t)
		})
		cur, maxc := 0, 0
		for _, e := range events {
			cur += e.delta
			if cur > maxc {
				maxc = cur
			}
		}
		if len(recs) > 0 && maxc == 0 {
			// Records without intervals count as concurrency 1 at most.
			// Keep 0 to signal "no interval data" only when no intervals exist.
			// If there were intervals, maxc >= 1 already.
		}
		st.MaxConcurrency = maxc
		out[stage] = st
	}
	return out
}
