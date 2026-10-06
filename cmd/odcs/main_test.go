package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	full    = "../../testdata/upstream/all/full-example.odcs.yaml"
	oldFile = "../../testdata/diff/old.odcs.yaml"
	newFile = "../../testdata/diff/new.odcs.yaml"
)

func writeTemp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "c.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRun(t *testing.T) {
	invalid := writeTemp(t, "apiVersion: v3.2.0\nkind: DataContract\nversion: 1.0.0\n")
	tests := []struct {
		name     string
		args     []string
		code     int
		contains string
	}{
		{"no args", nil, 2, "Usage:"},
		{"help", []string{"help"}, 0, "Usage:"},
		{"unknown", []string{"frobnicate"}, 2, "unknown command"},
		{"version", []string{"version"}, 0, "v3.2.0"},
		{"validate ok", []string{"validate", full}, 0, "valid"},
		{"validate invalid", []string{"validate", invalid}, 1, "missing property 'id'"},
		{"validate missing file", []string{"validate", "nope.yaml"}, 2, "no such file"},
		{"validate no files", []string{"validate"}, 2, "usage"},
		{"lint warnings", []string{"lint", full}, 0, "warning: /slaProperties/0/element"},
		{"lint strict", []string{"lint", "-strict", full}, 1, "warning"},
		{"lint invalid", []string{"lint", invalid}, 1, "error: /id: id is missing"},
		{"lint clean", []string{"lint", oldFile}, 0, "ok"},
		{"diff breaking", []string{"diff", oldFile, newFile}, 1, "BREAKING removed servers[lake]"},
		{"diff same", []string{"diff", oldFile, oldFile}, 0, "no changes"},
		{"diff usage", []string{"diff", oldFile}, 2, "usage"},
		{"diff bad flag", []string{"diff", "-x", oldFile, newFile}, 2, "flag provided but not defined"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := run(tt.args, &out, &errOut)
			all := out.String() + errOut.String()
			if code != tt.code {
				t.Errorf("exit code = %d, want %d\n%s", code, tt.code, all)
			}
			if !strings.Contains(all, tt.contains) {
				t.Errorf("output does not contain %q:\n%s", tt.contains, all)
			}
		})
	}
}

func TestRunJSON(t *testing.T) {
	var out bytes.Buffer
	if code := run([]string{"diff", "-json", oldFile, newFile}, &out, &out); code != 1 {
		t.Fatalf("exit code %d", code)
	}
	var changes []map[string]any
	if err := json.Unmarshal(out.Bytes(), &changes); err != nil || len(changes) == 0 {
		t.Fatalf("bad JSON (%v):\n%s", err, out.String())
	}

	out.Reset()
	if code := run([]string{"lint", "-json", full}, &out, &out); code != 0 {
		t.Fatalf("exit code %d", code)
	}
	var issues []struct {
		File     string `json:"file"`
		Severity string `json:"severity"`
		Path     string `json:"path"`
	}
	if err := json.Unmarshal(out.Bytes(), &issues); err != nil || len(issues) == 0 || issues[0].Severity != "warning" || issues[0].File != full {
		t.Fatalf("bad JSON (%v):\n%s", err, out.String())
	}
}
