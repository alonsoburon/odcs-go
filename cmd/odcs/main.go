// Command odcs validates, lints and compares ODCS data contracts.
//
//	odcs validate FILE...
//	odcs lint [-strict] [-json] FILE...
//	odcs diff [-json] OLD NEW
//
// Exit status is 0 on success, 1 when a check fails (invalid document, lint
// errors, breaking changes) and 2 on usage or I/O errors.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	odcs "github.com/alonsoburon/odcs-go"
)

const usage = `odcs: tools for Open Data Contract Standard (ODCS %s) documents

Usage:
  odcs validate FILE...               check files against the ODCS JSON Schema
  odcs lint [-strict] [-json] FILE... schema validation plus semantic checks
  odcs diff [-json] OLD NEW           list changes; exit 1 if any is breaking
  odcs version                        print the supported ODCS version

Exit status: 0 ok, 1 check failed, 2 usage or I/O error.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintf(stderr, usage, odcs.SchemaVersion)
		return 2
	}
	switch args[0] {
	case "validate":
		return validate(args[1:], stdout, stderr)
	case "lint":
		return lint(args[1:], stdout, stderr)
	case "diff":
		return diff(args[1:], stdout, stderr)
	case "version":
		fmt.Fprintln(stdout, odcs.SchemaVersion)
		return 0
	case "help", "-h", "-help", "--help":
		fmt.Fprintf(stdout, usage, odcs.SchemaVersion)
		return 0
	}
	fmt.Fprintf(stderr, "odcs: unknown command %q\n\n", args[0])
	fmt.Fprintf(stderr, usage, odcs.SchemaVersion)
	return 2
}

func newFlags(name string, stderr io.Writer) *flag.FlagSet {
	fs := flag.NewFlagSet("odcs "+name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	return fs
}

func validate(args []string, stdout, stderr io.Writer) int {
	fs := newFlags("validate", stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "usage: odcs validate FILE...")
		return 2
	}
	code := 0
	for _, path := range fs.Args() {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		if c := check(path, odcs.Validate(data), stdout); c > code {
			code = c
		}
	}
	return code
}

// check prints the outcome of validating one file and returns its exit code.
func check(path string, err error, out io.Writer) int {
	var ve *odcs.ValidationError
	switch {
	case err == nil:
		fmt.Fprintf(out, "%s: valid\n", path)
		return 0
	case errors.As(err, &ve):
		for _, e := range ve.Errors {
			fmt.Fprintf(out, "%s: error: %s\n", path, e)
		}
	default:
		fmt.Fprintf(out, "%s: error: %v\n", path, err)
	}
	return 1
}

type fileIssue struct {
	File string `json:"file"`
	odcs.Issue
}

func lint(args []string, stdout, stderr io.Writer) int {
	fs := newFlags("lint", stderr)
	strict := fs.Bool("strict", false, "fail on warnings too")
	asJSON := fs.Bool("json", false, "print issues as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(stderr, "usage: odcs lint [-strict] [-json] FILE...")
		return 2
	}
	code := 0
	all := []fileIssue{}
	for _, path := range fs.Args() {
		data, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 2
		}
		var issues []odcs.Issue
		var ve *odcs.ValidationError
		if err := odcs.Validate(data); errors.As(err, &ve) {
			for _, e := range ve.Errors {
				issues = append(issues, odcs.Issue{Severity: odcs.SeverityError, Path: e.Path, Message: e.Message})
			}
		} else if err != nil {
			issues = append(issues, odcs.Issue{Severity: odcs.SeverityError, Message: err.Error()})
		}
		if c, err := odcs.Parse(data); err == nil {
			issues = append(issues, odcs.Lint(c)...)
		}
		for _, i := range issues {
			if i.Severity == odcs.SeverityError || (*strict && i.Severity == odcs.SeverityWarning) {
				code = 1
			}
			all = append(all, fileIssue{path, i})
			if !*asJSON {
				fmt.Fprintf(stdout, "%s: %s\n", path, i)
			}
		}
		if len(issues) == 0 && !*asJSON {
			fmt.Fprintf(stdout, "%s: ok\n", path)
		}
	}
	if *asJSON {
		writeJSON(stdout, all)
	}
	return code
}

func diff(args []string, stdout, stderr io.Writer) int {
	fs := newFlags("diff", stderr)
	asJSON := fs.Bool("json", false, "print changes as JSON")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(stderr, "usage: odcs diff [-json] OLD NEW")
		return 2
	}
	old, err := odcs.ParseFile(fs.Arg(0))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	new, err := odcs.ParseFile(fs.Arg(1))
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	changes := odcs.Diff(old, new)
	if *asJSON {
		if changes == nil {
			changes = []odcs.Change{}
		}
		writeJSON(stdout, changes)
	} else {
		for _, c := range changes {
			fmt.Fprintln(stdout, c)
		}
		if len(changes) == 0 {
			fmt.Fprintln(stdout, "no changes")
		}
	}
	if odcs.HasBreaking(changes) {
		return 1
	}
	return 0
}

func writeJSON(w io.Writer, v any) {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
