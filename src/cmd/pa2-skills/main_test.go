package main

import (
	"strings"
	"testing"
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
