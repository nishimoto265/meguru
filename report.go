package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ReportDigest is the machine-readable digest of one loop.
type ReportDigest struct {
	File           string         `json:"file,omitempty"`
	Loop           string         `json:"loop"`
	Purpose        string         `json:"purpose"`
	CurrentStage   string         `json:"current_stage,omitempty"`
	Cycles         int            `json:"cycles"`
	Metrics        []ReportMetric `json:"metrics,omitempty"`
	RecentImprove  []Improvement  `json:"recent_improvements,omitempty"`
	NextCandidates []string       `json:"next_candidates,omitempty"`
}

type ReportMetric struct {
	Name    string `json:"name"`
	Current string `json:"current,omitempty"`
	Target  string `json:"target,omitempty"`
}

func DigestOf(l *Loop) ReportDigest {
	d := ReportDigest{
		File: l.File, Loop: l.Loop, Purpose: l.Purpose,
		CurrentStage: l.CurrentStage, Cycles: l.Cycles,
		NextCandidates: l.NextCandidates,
	}
	for _, m := range l.Metrics {
		d.Metrics = append(d.Metrics, ReportMetric{Name: m.Name, Current: m.Current, Target: m.Target})
	}
	// Recent improvements: last up to 3 in file order reversed (latest last in file = most recent).
	imps := l.Improvements
	if len(imps) > 3 {
		imps = imps[len(imps)-3:]
	}
	// Present most-recent first.
	for i := len(imps) - 1; i >= 0; i-- {
		d.RecentImprove = append(d.RecentImprove, imps[i])
	}
	return d
}

func WriteReportJSON(w io.Writer, loops []*Loop) error {
	digests := make([]ReportDigest, 0, len(loops))
	for _, l := range loops {
		digests = append(digests, DigestOf(l))
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(digests)
}

func WriteReportMarkdown(w io.Writer, loops []*Loop) {
	for i, l := range loops {
		if i > 0 {
			fmt.Fprintln(w, "---")
		}
		d := DigestOf(l)
		fmt.Fprintf(w, "## %s\n\n", d.Loop)
		fmt.Fprintf(w, "- Purpose: %s\n", d.Purpose)
		if d.CurrentStage != "" {
			fmt.Fprintf(w, "- Current stage: %s\n", d.CurrentStage)
		}
		fmt.Fprintf(w, "- Cycles: %d\n", d.Cycles)
		if len(d.Metrics) > 0 {
			fmt.Fprintln(w, "\n### Metrics")
			for _, m := range d.Metrics {
				fmt.Fprintf(w, "- %s: %s → %s\n", m.Name, m.Current, m.Target)
			}
		}
		if len(d.RecentImprove) > 0 {
			fmt.Fprintln(w, "\n### Recent improvements")
			for _, imp := range d.RecentImprove {
				line := fmt.Sprintf("- [%s] %s", imp.Date, imp.Change)
				if imp.AimedAt != "" {
					line += fmt.Sprintf(" (aimed at: %s)", imp.AimedAt)
				}
				if imp.Result != "" {
					line += fmt.Sprintf(" — result: %s", imp.Result)
				}
				fmt.Fprintln(w, strings.TrimSpace(line))
			}
		}
		if len(d.NextCandidates) > 0 {
			fmt.Fprintln(w, "\n### Next candidates")
			for _, c := range d.NextCandidates {
				fmt.Fprintf(w, "- %s\n", c)
			}
		}
		fmt.Fprintln(w)
	}
}
