package main

import "fmt"

// ValidateLoop returns human-readable issues for one definition.
// An empty slice means the definition is valid.
func ValidateLoop(l *Loop) []string {
	var issues []string
	if l.Loop == "" {
		issues = append(issues, "loop: loop name is required")
	}
	if l.Purpose == "" {
		issues = append(issues, "purpose: purpose is required")
	}
	metricNames := map[string]bool{}
	for i, m := range l.Metrics {
		label := fmt.Sprintf("metrics[%d]", i)
		if m.Name != "" {
			label = fmt.Sprintf("metrics[%q]", m.Name)
		} else {
			issues = append(issues, fmt.Sprintf("%s: name is required", label))
		}
		if m.Name != "" {
			metricNames[m.Name] = true
		}
		if m.Target == "" {
			issues = append(issues, fmt.Sprintf("%s: target is required", label))
		}
		if m.MeasuredBy == "" {
			issues = append(issues, fmt.Sprintf("%s: measured_by is required", label))
		}
	}
	for i, s := range l.Stages {
		label := fmt.Sprintf("stages[%d]", i)
		if s.Name != "" {
			label = fmt.Sprintf("stages[%q]", s.Name)
		} else {
			issues = append(issues, fmt.Sprintf("%s: name is required", label))
		}
		if s.Split == "" {
			issues = append(issues, fmt.Sprintf("%s: split is required", label))
		}
		if s.Reason == "" {
			issues = append(issues, fmt.Sprintf("%s: reason is required", label))
		}
		if s.ParallelMax == nil && s.ParallelNote == "" {
			issues = append(issues, fmt.Sprintf("%s: parallel_max or parallel_note is required", label))
		}
	}
	for i, imp := range l.Improvements {
		label := fmt.Sprintf("improvements[%d]", i)
		if imp.AimedAt == "" {
			issues = append(issues, fmt.Sprintf("%s: aimed_at is required", label))
		} else if !metricNames[imp.AimedAt] {
			issues = append(issues, fmt.Sprintf("%s: aimed_at %q does not match any defined metric", label, imp.AimedAt))
		}
	}
	return issues
}
