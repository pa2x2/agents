package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pa2x2/agents/src/internal/pa2skills"
)

var version = "development"

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "pa2-skills:", err)
		os.Exit(1)
	}
}

func run(arguments []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(arguments) == 0 {
		printUsage(stdout)
		return nil
	}
	paths, err := pa2skills.ResolvePaths()
	if err != nil {
		return err
	}
	manager := pa2skills.Manager{Paths: paths, Stdin: stdin, Stdout: stdout, Stderr: stderr}
	switch arguments[0] {
	case "version", "--version":
		fmt.Fprintln(stdout, version)
		return nil
	case "install", "update":
		return installOrUpdate(arguments[0], arguments[1:], manager, stdout)
	case "list":
		return listSkills(manager, stdout)
	case "source-path":
		fmt.Fprintln(stdout, paths.SourceRoot)
		return nil
	case "cd":
		return changeDirectory(arguments[1:], paths.SourceRoot, stdin, stdout, stderr)
	case "completion":
		return completion(arguments[1:], manager, stdout)
	case "doctor":
		return doctor(manager, stdout)
	case "help", "--help", "-h":
		printUsage(stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q", arguments[0])
	}
}

func installOrUpdate(command string, arguments []string, manager pa2skills.Manager, stdout io.Writer) error {
	if len(arguments) == 0 || strings.HasPrefix(arguments[0], "-") {
		return fmt.Errorf("usage: pa2-skills %s <skill> --scope user|project --harness codex,claude", command)
	}
	skill := arguments[0]
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stdout)
	scope := flags.String("scope", "", "user or project")
	harnesses := flags.String("harness", "", "comma-separated harnesses")
	conflict := flags.String("conflict", "ask", "ask, overwrite, or skip")
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("usage: pa2-skills %s <skill> --scope user|project --harness codex,claude", command)
	}
	if *scope != string(pa2skills.ScopeUser) && *scope != string(pa2skills.ScopeProject) {
		return errors.New("--scope must be user or project")
	}
	policy := pa2skills.ConflictPolicy(*conflict)
	if policy != pa2skills.ConflictAsk && policy != pa2skills.ConflictOverwrite && policy != pa2skills.ConflictSkip {
		return errors.New("--conflict must be ask, overwrite, or skip")
	}
	selectedHarnesses := splitValues(*harnesses)
	if command == "install" {
		return manager.Install(skill, pa2skills.Scope(*scope), selectedHarnesses, policy)
	}
	return manager.Update(skill, pa2skills.Scope(*scope), selectedHarnesses, policy)
}

func listSkills(manager pa2skills.Manager, stdout io.Writer) error {
	names, err := manager.SkillNames()
	if err != nil {
		return err
	}
	for _, name := range names {
		fmt.Fprintln(stdout, name)
	}
	return nil
}

func changeDirectory(arguments []string, sourceRoot string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(arguments) > 1 {
		return errors.New("usage: pa2-skills cd [path]")
	}
	directory := sourceRoot
	if len(arguments) == 1 {
		directory = filepath.Join(sourceRoot, arguments[0])
	}
	resolved, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return fmt.Errorf("resolve source path: %w", err)
	}
	relative, err := filepath.Rel(sourceRoot, resolved)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("cd path must remain inside the managed source checkout")
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	child := exec.Command(shell)
	child.Dir = resolved
	child.Stdin = stdin
	child.Stdout = stdout
	child.Stderr = stderr
	child.Env = append(os.Environ(), "PA2_SKILLS_SOURCE_DIR="+sourceRoot)
	return child.Run()
}

func completion(arguments []string, manager pa2skills.Manager, stdout io.Writer) error {
	if len(arguments) != 1 {
		return errors.New("usage: pa2-skills completion zsh|values")
	}
	switch arguments[0] {
	case "zsh":
		fmt.Fprint(stdout, zshCompletion)
		return nil
	case "values":
		names, err := manager.SkillNames()
		if err != nil {
			return err
		}
		for _, name := range names {
			fmt.Fprintln(stdout, name)
		}
		return nil
	default:
		return errors.New("completion supports zsh or values")
	}
}

func doctor(manager pa2skills.Manager, stdout io.Writer) error {
	if _, err := os.Stat(filepath.Join(manager.Paths.SourceRoot, ".git")); err != nil {
		return fmt.Errorf("managed source checkout is unavailable at %s; run the bootstrap script", manager.Paths.SourceRoot)
	}
	if _, err := exec.LookPath("git"); err != nil {
		return errors.New("git is required")
	}
	fmt.Fprintf(stdout, "Source checkout: %s\n", manager.Paths.SourceRoot)
	fmt.Fprintf(stdout, "Private state: %s\n", manager.Paths.StateRoot())
	return nil
}

func splitValues(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if normalized := strings.TrimSpace(part); normalized != "" {
			result = append(result, normalized)
		}
	}
	sort.Strings(result)
	return result
}

func printUsage(writer io.Writer) {
	fmt.Fprint(writer, `Usage:
  pa2-skills install <skill> --scope user|project --harness codex,claude [--conflict ask|overwrite|skip]
  pa2-skills update <skill> --scope user|project --harness codex,claude [--conflict ask|overwrite|skip]
  pa2-skills list
  pa2-skills source-path
  pa2-skills cd [path]
  pa2-skills completion zsh
  pa2-skills doctor
`)
}

const zshCompletion = `#compdef pa2-skills

_pa2_skills() {
  local -a commands skills harnesses scopes conflicts
  commands=(
    'install:install or refresh a skill'
    'update:fetch the source repository and refresh a skill'
	    'version:print the installed command version'
    'list:list available skills'
    'source-path:print the managed source checkout'
    'cd:open a shell in the managed source checkout'
    'completion:generate shell completion'
    'doctor:check the local installation'
  )
  harnesses=(claude codex opencode)
  scopes=(user project)
  conflicts=(ask overwrite skip)
  case $words[2] in
    install|update)
      _arguments -s \
        '--scope=[installation scope]:scope:->scope' \
        '--harness=[comma-separated harnesses]:harness:->harness' \
        '--conflict=[conflict policy]:policy:->conflict' \
        '1:skill:->skill'
      case $state in
        scope) _values 'scope' $scopes ;;
        harness) _values -s , 'harness' $harnesses ;;
        conflict) _values 'conflict policy' $conflicts ;;
        skill) skills=("${(@f)$($words[1] completion values 2>/dev/null)}"); _describe -t skills skill skills ;;
      esac
      ;;
    completion)
      _values 'format' zsh values
      ;;
    cd)
      _files -/
      ;;
    *)
      _describe -t commands command commands
      ;;
  esac
}

compdef _pa2_skills pa2-skills
`
