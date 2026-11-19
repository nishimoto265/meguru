package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Loop is a meguru loop definition loaded from YAML.
type Loop struct {
	Loop           string        `yaml:"loop" json:"loop"`
	Purpose        string        `yaml:"purpose" json:"purpose"`
	UpdatedAt      string        `yaml:"updated_at" json:"updated_at,omitempty"`
	CurrentStage   string        `yaml:"current_stage" json:"current_stage,omitempty"`
	Cycles         int           `yaml:"cycles" json:"cycles"`
	Metrics        []Metric      `yaml:"metrics" json:"metrics,omitempty"`
	Stages         []Stage       `yaml:"stages" json:"stages,omitempty"`
	Improvements   []Improvement `yaml:"improvements" json:"improvements,omitempty"`
	NextCandidates []string      `yaml:"next_candidates" json:"next_candidates,omitempty"`

	File string `yaml:"-" json:"file,omitempty"`
}

type Metric struct {
	Name       string `yaml:"name" json:"name"`
	Current    string `yaml:"current" json:"current,omitempty"`
	Target     string `yaml:"target" json:"target,omitempty"`
	MeasuredBy string `yaml:"measured_by" json:"measured_by,omitempty"`
	Updated    string `yaml:"updated" json:"updated,omitempty"`
}

type Stage struct {
	Name         string `yaml:"name" json:"name"`
	Split        string `yaml:"split" json:"split,omitempty"`
	ParallelMax  *int   `yaml:"parallel_max" json:"parallel_max,omitempty"`
	ParallelNote string `yaml:"parallel_note" json:"parallel_note,omitempty"`
	Reason       string `yaml:"reason" json:"reason,omitempty"`
}

type Improvement struct {
	Date    string `yaml:"date" json:"date,omitempty"`
	Change  string `yaml:"change" json:"change,omitempty"`
	AimedAt string `yaml:"aimed_at" json:"aimed_at,omitempty"`
	Result  string `yaml:"result" json:"result,omitempty"`
}

// ParseDate parses "2006-01-02" or RFC3339 (and a few close variants).
func ParseDate(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date")
	}
	layouts := []string{
		"2006-01-02",
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse date %q", s)
}

// LoadLoop reads and parses one YAML definition file.
func LoadLoop(path string) (*Loop, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var l Loop
	dec := yaml.NewDecoder(strings.NewReader(string(data)))
	dec.KnownFields(false)
	if err := dec.Decode(&l); err != nil {
		return nil, fmt.Errorf("YAML parse error: %w", err)
	}
	l.File = path
	return &l, nil
}

// ExpandPaths expands file/dir arguments into a sorted list of YAML files.
func ExpandPaths(args []string) ([]string, error) {
	var out []string
	for _, a := range args {
		info, err := os.Stat(a)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			out = append(out, a)
			continue
		}
		var found []string
		err = filepath.WalkDir(a, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(p))
			if ext == ".yaml" || ext == ".yml" {
				found = append(found, p)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		sort.Strings(found)
		out = append(out, found...)
	}
	sort.Strings(out)
	return out, nil
}

// LoadLoops expands paths and loads every definition.
// Load errors are returned as per-file errors keyed by path so callers can
// report them without aborting the whole run.
func LoadLoops(args []string) (loops []*Loop, loadErrs map[string]error, err error) {
	files, err := ExpandPaths(args)
	if err != nil {
		return nil, nil, err
	}
	loadErrs = map[string]error{}
	for _, f := range files {
		l, lerr := LoadLoop(f)
		if lerr != nil {
			loadErrs[f] = lerr
			continue
		}
		loops = append(loops, l)
	}
	return loops, loadErrs, nil
}
