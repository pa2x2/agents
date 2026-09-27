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
	if help, found := lookupCommand(arguments[0]); found && hasHelpFlag(arguments[1:]) {
		printCommandHelp(stdout, help)
		return nil
	}
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
		return printHelp(arguments[1:], stdout)
	default:
		return unknownCommand(arguments[0])
	}
}

func printHelp(arguments []string, stdout io.Writer) error {
	if len(arguments) > 1 {
		return errors.New("usage: pa2-skills help [command]")
	}
	if len(arguments) == 0 {
		printUsage(stdout)
		return nil
	}
	help, found := lookupCommand(arguments[0])
	if !found {
		return unknownCommand(arguments[0])
	}
	printCommandHelp(stdout, help)
	return nil
}

func unknownCommand(name string) error {
	message := fmt.Sprintf("unknown command %q", name)
	best, bestDistance := "", 3
	for _, help := range commandHelps {
		if distance := editDistance(name, help.name); distance < bestDistance {
			best, bestDistance = help.name, distance
		}
	}
	if best != "" {
		message += fmt.Sprintf("; did you mean %q?", best)
	}
	return errors.New(message + "\nRun 'pa2-skills help' for usage.")
}

func editDistance(left, right string) int {
	previous := make([]int, len(right)+1)
	for index := range previous {
		previous[index] = index
	}
	for i := 1; i <= len(left); i++ {
		current := make([]int, len(right)+1)
		current[0] = i
		for j := 1; j <= len(right); j++ {
			cost := 1
			if left[i-1] == right[j-1] {
				cost = 0
			}
			current[j] = min(previous[j]+1, current[j-1]+1, previous[j-1]+cost)
		}
		previous = current
	}
	return previous[len(right)]
}

func hasHelpFlag(arguments []string) bool {
	for _, argument := range arguments {
		switch argument {
		case "--":
			return false
		case "-h", "-help", "--help":
			return true
		}
	}
	return false
}

// parseArguments parses flags that may appear before, between, or after positional arguments.
// Arguments after a literal "--" are always positional.
func parseArguments(flags *flag.FlagSet, arguments []string) ([]string, error) {
	var positionals []string
	for {
		if err := flags.Parse(arguments); err != nil {
			return nil, fmt.Errorf("%w\nRun 'pa2-skills help %s' for usage.", err, flags.Name())
		}
		remaining := flags.Args()
		consumed := len(arguments) - len(remaining)
		if consumed > 0 && arguments[consumed-1] == "--" {
			return append(positionals, remaining...), nil
		}
		if len(remaining) == 0 {
			return positionals, nil
		}
		positionals = append(positionals, remaining[0])
		arguments = remaining[1:]
	}
}

// newFlagSet returns a flag set whose errors are reported only through the returned error.
func newFlagSet(name string) *flag.FlagSet {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() {}
	return flags
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
	flags := newFlagSet("add")
	name := flags.String("name", "", "tracked skill name")
	positionals, err := parseArguments(flags, arguments)
	if err != nil {
		return err
	}
	source := ""
	if len(positionals) > 0 {
		source = positionals[0]
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
	if len(positionals) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("unexpected argument(s): %s", strings.Join(positionals[1:], ", ")))
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
	flags := newFlagSet(command)
	scope := flags.String("scope", environmentDefault("PA2_SKILLS_SCOPE", string(pa2skills.ScopeUser)), "user or project")
	harnesses := flags.String("harness", environmentDefault("PA2_SKILLS_HARNESS", pa2skills.HarnessAll), "comma-separated harnesses, or all")
	conflict := flags.String("conflict", "ask", "ask, overwrite, or skip")
	positionals, err := parseArguments(flags, arguments)
	if err != nil {
		return err
	}
	skill := ""
	if len(positionals) > 0 {
		skill = positionals[0]
	}
	var validationErrors []error
	if len(positionals) > 1 {
		validationErrors = append(validationErrors, fmt.Errorf("unexpected argument(s): %s", strings.Join(positionals[1:], ", ")))
	}
	policy := pa2skills.ConflictPolicy(*conflict)
	selectedHarnesses := pa2skills.ExpandHarnesses(splitValues(*harnesses))
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
	flags := newFlagSet("update")
	check := flags.Bool("check", false, "report available updates without changing files")
	binOnly := flags.Bool("binary-only", false, "only update the pa2-skills binary")
	skillsOnly := flags.Bool("skills-only", false, "only update the source and managed skills")
	conflict := flags.String("conflict", "ask", "ask, overwrite, or skip")
	positionals, err := parseArguments(flags, arguments)
	if err != nil {
		return err
	}
	var validationErrors []error
	if len(positionals) != 0 {
		validationErrors = append(validationErrors, fmt.Errorf("unexpected argument(s): %s", strings.Join(positionals, ", ")))
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

func environmentDefault(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
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

type commandHelp struct {
	name    string
	usage   string
	summary string
	details string
}

var commandHelps = []commandHelp{
	{
		name:    "install",
		usage:   "install <skill> [--scope user|project] [--harness <harnesses>|all] [--conflict ask|overwrite|skip]",
		summary: "Install a skill from the managed source checkout, or refresh an existing installation.",
		details: installFlagsHelp,
	},
	{
		name:    "sync",
		usage:   "sync <skill> [--scope user|project] [--harness <harnesses>|all] [--conflict ask|overwrite|skip]",
		summary: "Fetch the source repository, then install or refresh a skill.",
		details: installFlagsHelp,
	},
	{
		name:    "update",
		usage:   "update [--check] [--binary-only|--skills-only] [--conflict ask|overwrite|skip]",
		summary: "Upgrade the binary, then synchronize the source checkout and every managed installation.",
		details: `Flags:
  --check                          report available updates without changing files
  --binary-only                    only update the pa2-skills binary
  --skills-only                    only update the source checkout and managed skills
  --conflict ask|overwrite|skip    how to resolve skills with local and remote changes (default: ask)
`,
	},
	{name: "list", usage: "list", summary: "List skills available from the managed source checkout."},
	{name: "discover", usage: "discover [path]", summary: "Report skills found below the current or supplied directory."},
	{
		name:    "add",
		usage:   "add <skill-path> [--name <name>]",
		summary: "Copy a local skill into the managed source checkout without Git operations.",
		details: `Flags:
  --name <name>    tracked skill name (default: the directory name)
`,
	},
	{name: "version", usage: "version", summary: "Print the installed command version."},
	{name: "source-path", usage: "source-path", summary: "Print the managed source checkout path."},
	{name: "cd", usage: "cd [path]", summary: "Launch a child shell in the managed source checkout or one of its paths."},
	{name: "completion", usage: "completion zsh", summary: "Print dynamic Zsh completion."},
	{name: "doctor", usage: "doctor", summary: "Check the managed source checkout and local prerequisites."},
	{name: "help", usage: "help [command]", summary: "Show usage for all commands or one command."},
}

const installFlagsHelp = `Flags:
  --scope user|project             install for the user or for the current Git project
                                   (default: $PA2_SKILLS_SCOPE, else user)
  --harness <harnesses>|all        comma-separated harnesses: claude, codex, opencode; all selects every one
                                   (default: $PA2_SKILLS_HARNESS, else all)
  --conflict ask|overwrite|skip    how to resolve local changes (default: ask)
`

func lookupCommand(name string) (commandHelp, bool) {
	if name == "--version" {
		name = "version"
	}
	for _, help := range commandHelps {
		if help.name == name {
			return help, true
		}
	}
	return commandHelp{}, false
}

func printUsage(writer io.Writer) {
	fmt.Fprintln(writer, "Usage:")
	for _, help := range commandHelps {
		fmt.Fprintf(writer, "  pa2-skills %s\n", help.usage)
	}
	fmt.Fprintln(writer, "\nRun 'pa2-skills help <command>' for details.")
}

func printCommandHelp(writer io.Writer, help commandHelp) {
	fmt.Fprintf(writer, "Usage: pa2-skills %s\n\n%s\n", help.usage, help.summary)
	if help.details != "" {
		fmt.Fprintf(writer, "\n%s", help.details)
	}
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
    'help:show usage for a command'
  )
  harnesses=(all claude codex opencode)
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
        '--harness=[comma-separated harnesses, or all]:harness:->harness' \
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
    help)
      _describe -t commands command commands
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
