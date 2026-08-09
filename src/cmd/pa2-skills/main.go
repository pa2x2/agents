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

	"pa2-skills/internal/pa2skills"
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
		if err := requireNoArguments("version", arguments[1:]); err != nil {
			return err
		}
		fmt.Fprintln(stdout, version)
		return nil
	case "install", "sync":
		return installOrSync(arguments[0], arguments[1:], manager, stdout)
	case "update":
		return updateInstallation(arguments[1:], manager, stdout, stderr)
	case "list":
		if err := requireNoArguments("list", arguments[1:]); err != nil {
			return err
		}
		return listSkills(manager, stdout)
	case "discover":
		return discoverSkills(arguments[1:], manager, stdout)
	case "add":
		return addSkill(arguments[1:], manager, stdout)
	case "source-path":
		if err := requireNoArguments("source-path", arguments[1:]); err != nil {
			return err
		}
		fmt.Fprintln(stdout, paths.SourceRoot)
		return nil
	case "cd":
		return changeDirectory(arguments[1:], paths.SourceRoot, stdin, stdout, stderr)
	case "completion":
		return completion(arguments[1:], manager, stdout)
	case "doctor":
		if err := requireNoArguments("doctor", arguments[1:]); err != nil {
			return err
		}
		return doctor(manager, stdout)
	case "help", "--help", "-h":
		if err := requireNoArguments("help", arguments[1:]); err != nil {
			return err
		}
		printUsage(stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q", arguments[0])
	}
}

func discoverSkills(arguments []string, manager pa2skills.Manager, stdout io.Writer) error {
	if len(arguments) > 1 {
		return errors.New("usage: pa2-skills discover [path]")
	}
	root := "."
	if len(arguments) == 1 {
		root = arguments[0]
	}
	findings, err := manager.Discover(root)
	if err != nil {
		return err
	}
	for _, finding := range findings {
		line := fmt.Sprintf("%-8s  %s  %s", finding.Status, finding.Path, finding.Name)
		if finding.Detail != "" {
			line += "  " + finding.Detail
		}
		fmt.Fprintln(stdout, line)
	}
	if len(findings) == 0 {
		fmt.Fprintln(stdout, "No skills found.")
	}
	return nil
}

func addSkill(arguments []string, manager pa2skills.Manager, stdout io.Writer) error {
	source := ""
	flagArguments := arguments
	if len(arguments) > 0 && !strings.HasPrefix(arguments[0], "-") {
		source = arguments[0]
		flagArguments = arguments[1:]
	}
	flags := flag.NewFlagSet("add", flag.ContinueOnError)
	flags.SetOutput(stdout)
	name := flags.String("name", "", "tracked skill name")
	if err := flags.Parse(flagArguments); err != nil {
		return err
	}
	var validationErrors []error
	if source == "" {
		validationErrors = append(validationErrors, errors.New("skill path is required"))
	}
	if *name != "" {
		if err := pa2skills.ValidateSkillName(*name); err != nil {
			validationErrors = append(validationErrors, err)
		}
	}
	if flags.NArg() != 0 {
		validationErrors = append(validationErrors, fmt.Errorf("unexpected argument(s): %s", strings.Join(flags.Args(), ", ")))
	}
	if err := invalidArguments(validationErrors...); err != nil {
		return err
	}
	target, err := manager.AddSkill(source, *name)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Added %s\nSource:  %s\nTracked: %s\n\nReview and publish manually:\n  pa2-skills cd\n  git status\n", filepath.Base(target), source, target)
	return nil
}

func installOrSync(command string, arguments []string, manager pa2skills.Manager, stdout io.Writer) error {
	skill := ""
	flagArguments := arguments
	if len(arguments) > 0 && !strings.HasPrefix(arguments[0], "-") {
		skill = arguments[0]
		flagArguments = arguments[1:]
	}
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stdout)
	scope := flags.String("scope", "", "user or project")
	harnesses := flags.String("harness", "", "comma-separated harnesses")
	conflict := flags.String("conflict", "ask", "ask, overwrite, or skip")
	if err := flags.Parse(flagArguments); err != nil {
		return err
	}
	var validationErrors []error
	if flags.NArg() != 0 {
		validationErrors = append(validationErrors, fmt.Errorf("unexpected argument(s): %s", strings.Join(flags.Args(), ", ")))
	}
	policy := pa2skills.ConflictPolicy(*conflict)
	selectedHarnesses := splitValues(*harnesses)
	if err := pa2skills.ValidateInstallArguments(skill, pa2skills.Scope(*scope), selectedHarnesses, policy); err != nil {
		validationErrors = append(validationErrors, err)
	}
	if err := invalidArguments(validationErrors...); err != nil {
		return err
	}
	if command == "install" {
		return manager.Install(skill, pa2skills.Scope(*scope), selectedHarnesses, policy)
	}
	return manager.Sync(skill, pa2skills.Scope(*scope), selectedHarnesses, policy)
}

func updateInstallation(arguments []string, manager pa2skills.Manager, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("update", flag.ContinueOnError)
	flags.SetOutput(stdout)
	check := flags.Bool("check", false, "report available updates without changing files")
	binOnly := flags.Bool("binary-only", false, "only update the pa2-skills binary")
	skillsOnly := flags.Bool("skills-only", false, "only update the source and managed skills")
	conflict := flags.String("conflict", "ask", "ask, overwrite, or skip")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	var validationErrors []error
	if flags.NArg() != 0 {
		validationErrors = append(validationErrors, fmt.Errorf("unexpected argument(s): %s", strings.Join(flags.Args(), ", ")))
	}
	if *binOnly && *skillsOnly {
		validationErrors = append(validationErrors, errors.New("--binary-only and --skills-only cannot be used together"))
	}
	policy := pa2skills.ConflictPolicy(*conflict)
	if policy != pa2skills.ConflictAsk && policy != pa2skills.ConflictOverwrite && policy != pa2skills.ConflictSkip {
		validationErrors = append(validationErrors, errors.New("--conflict must be ask, overwrite, or skip"))
	}
	if err := invalidArguments(validationErrors...); err != nil {
		return err
	}
	if !*skillsOnly {
		fmt.Fprintln(stdout, "Binary: checking for updates...")
		result, err := pa2skills.UpdateBinary(version, *check, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "Binary: check failed: %v\n", err)
		} else {
			fmt.Fprintf(stdout, "Binary: %s\n", result)
		}
	}
	if *binOnly {
		return nil
	}
	if *check {
		fmt.Fprintln(stdout, "Source: checking for updates...")
		result, err := manager.CheckSource()
		if err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Source: %s\n", result)
		return nil
	}
	fmt.Fprintln(stdout, "Source: synchronizing checkout and installed skills...")
	return manager.UpdateAll(policy)
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
	var validationErrors []error
	if len(arguments) == 0 {
		validationErrors = append(validationErrors, errors.New("completion format is required (zsh or values)"))
	} else if arguments[0] != "zsh" && arguments[0] != "values" {
		validationErrors = append(validationErrors, fmt.Errorf("unsupported completion format %q (supported: zsh, values)", arguments[0]))
	}
	if len(arguments) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("unexpected argument(s): %s", strings.Join(arguments[1:], ", ")))
	}
	if err := invalidArguments(validationErrors...); err != nil {
		return err
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
	}
	return nil
}

func doctor(manager pa2skills.Manager, stdout io.Writer) error {
	var validationErrors []error
	if _, err := os.Stat(filepath.Join(manager.Paths.SourceRoot, ".git")); err != nil {
		validationErrors = append(validationErrors, fmt.Errorf("managed source checkout is unavailable at %s; run the bootstrap script", manager.Paths.SourceRoot))
	}
	if _, err := exec.LookPath("git"); err != nil {
		validationErrors = append(validationErrors, errors.New("git is required"))
	}
	if err := errors.Join(validationErrors...); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "Source checkout: %s\n", manager.Paths.SourceRoot)
	fmt.Fprintf(stdout, "Private state: %s\n", manager.Paths.StateRoot())
	return nil
}

func invalidArguments(validationErrors ...error) error {
	if err := errors.Join(validationErrors...); err != nil {
		return fmt.Errorf("invalid arguments:\n  - %s", strings.ReplaceAll(err.Error(), "\n", "\n  - "))
	}
	return nil
}

func requireNoArguments(command string, arguments []string) error {
	if len(arguments) == 0 {
		return nil
	}
	return invalidArguments(fmt.Errorf("%s does not accept arguments: %s", command, strings.Join(arguments, ", ")))
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
  pa2-skills sync <skill> --scope user|project --harness codex,claude [--conflict ask|overwrite|skip]
  pa2-skills update [--check] [--binary-only|--skills-only] [--conflict ask|overwrite|skip]
  pa2-skills list
  pa2-skills discover [path]
  pa2-skills add <skill-path> [--name <name>]
  pa2-skills source-path
  pa2-skills cd [path]
  pa2-skills completion zsh
  pa2-skills doctor
`)
}

const zshCompletion = `#compdef pa2-skills

_pa2_skills() {
  local command
  local -a commands skills harnesses scopes conflicts
  commands=(
	'install:install or refresh a skill'
	'sync:fetch the source repository and refresh a skill'
	'update:update the binary, source, and managed skills'
	    'version:print the installed command version'
    'list:list available skills'
    'discover:find skills below a directory'
    'add:copy a skill into the managed source checkout'
    'source-path:print the managed source checkout'
    'cd:open a shell in the managed source checkout'
    'completion:generate shell completion'
    'doctor:check the local installation'
  )
  harnesses=(claude codex opencode)
  scopes=(user project)
  conflicts=(ask overwrite skip)
  command=$words[2]
  if (( CURRENT > 2 )); then
    words=($words[1] "${words[3,-1]}")
    (( CURRENT-- ))
  fi
  case $command in
    install|sync)
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
    update)
      _arguments -s \
        '--check[report available updates without changing files]' \
        '--binary-only[only update the pa2-skills binary]' \
        '--skills-only[only update source and managed skills]' \
        '--conflict=[conflict policy]:policy:->conflict'
      case $state in
        conflict) _values 'conflict policy' $conflicts ;;
      esac
      ;;
    discover)
      _files -/
      ;;
    add)
      _arguments -s \
        '--name=[tracked skill name]:name:' \
        '1:skill directory:_files -/'
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
