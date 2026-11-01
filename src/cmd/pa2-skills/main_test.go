package main

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"pa2-skills/internal/pa2skills"
)

func TestParseArgumentsAcceptsFlagsAnywhere(t *testing.T) {
	flags := newFlagSet("install")
	scope := flags.String("scope", "", "")
	positionals, err := parseArguments(flags, []string{"first", "--scope", "user", "second", "--", "--third"})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := strings.Join(positionals, ","), "first,second,--third"; got != want {
		t.Fatalf("positionals = %q, want %q", got, want)
	}
	if *scope != "user" {
		t.Fatalf("scope = %q, want user", *scope)
	}
}

func TestUnknownCommandSuggestsClosestCommand(t *testing.T) {
	if err := unknownCommand("instal"); !strings.Contains(err.Error(), `did you mean "install"?`) {
		t.Fatalf("error = %q, want an install suggestion", err)
	}
	if err := unknownCommand("zzzzzz"); strings.Contains(err.Error(), "did you mean") {
		t.Fatalf("error = %q, want no suggestion", err)
	}
}

func TestUpdateHandsSkillsToNewBinary(t *testing.T) {
	originalUpdate, originalExec := updateBinary, execBinary
	t.Cleanup(func() { updateBinary, execBinary = originalUpdate, originalExec })
	updateBinary = func(string, bool, io.Writer) (string, string, error) {
		return "updated v1 -> v2", "/new/pa2-skills", nil
	}

	var execs [][]string
	execBinary = func(path string, arguments []string, _ []string) error {
		execs = append(execs, append([]string{path}, arguments...))
		return nil
	}
	var stdout, stderr bytes.Buffer
	if err := updateInstallation([]string{"--conflict", "skip"}, pa2skills.Manager{}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	want := "/new/pa2-skills /new/pa2-skills update --skills-only --conflict skip"
	if len(execs) != 1 || strings.Join(execs[0], " ") != want {
		t.Fatalf("execs = %q, want [%q]", execs, want)
	}

	execs = nil
	if err := updateInstallation([]string{"--binary-only"}, pa2skills.Manager{}, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if len(execs) != 0 {
		t.Fatalf("--binary-only re-executed: %q", execs)
	}
}
