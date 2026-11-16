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
	"syscall"
	"text/tabwriter"

	"pa2-skills/internal/pa2skills"
)

var version = "development"

// Overridable in tests.
var (
	updateBinary = pa2skills.UpdateBinary
	execBinary   = syscall.Exec
)

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
	case "install":
		return installItems(arguments[1:], manager)
	case "remove":
		return removeItems(arguments[1:], manager)
	case "update":
		return updateInstallation(arguments[1:], manager, stdout, stderr)
	case "list":
		if err := requireNoArguments("list", arguments[1:]); err != nil {
			return err
		}
		return listItems(manager, stdout)
	case "status":
		return showStatus(arguments[1:], manager, stdout)
	case "diff":
		return showDiff(arguments[1:], manager)
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

func unknownCommand(name string) error {
	message := fmt.Sprintf("unknown command %q", name)
	names := make([]string, 0, len(commandHelps))
	for _, help := range commandHelps {
		names = append(names, help.name)
	}
	if best := pa2skills.ClosestName(name, names); best != "" {
		message += fmt.Sprintf("; did you mean %q?", best)
	}
	return errors.New(message + "\nRun 'pa2-skills help' for usage.")
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
	if err := checkFlags(flags, arguments); err != nil {
		return nil, fmt.Errorf("%w\nRun 'pa2-skills help %s' for usage.", err, flags.Name())
	}
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

// checkFlags reports unknown flags and missing flag values before the flag
// package does, because it names flags with a single dash and suggests nothing.
func checkFlags(flags *flag.FlagSet, arguments []string) error {
	for index := 0; index < len(arguments); index++ {
		argument := arguments[index]
		if argument == "--" {
			return nil
		}
		if len(argument) < 2 || argument[0] != '-' {
			continue
		}
		name, _, hasValue := strings.Cut(strings.TrimLeft(argument, "-"), "=")
		defined := flags.Lookup(name)
		if defined == nil {
			var names []string
			flags.VisitAll(func(candidate *flag.Flag) { names = append(names, candidate.Name) })
			message := fmt.Sprintf("unknown flag --%s", name)
			if best := pa2skills.ClosestName(name, names); best != "" {
				message += fmt.Sprintf("; did you mean --%s?", best)
			}
			return errors.New(message)
		}
		if boolean, ok := defined.Value.(interface{ IsBoolFlag() bool }); hasValue || ok && boolean.IsBoolFlag() {
			continue
		}
		if index+1 == len(arguments) {
			return fmt.Errorf("--%s needs a value", name)
		}
		index++
	}
	return nil
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
	if len(findings) == 0 {
		fmt.Fprintln(stdout, "No skills found.")
		return nil
	}
	writer := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "STATUS\tNAME\tPATH\tDETAIL")
	for _, finding := range findings {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", finding.Status, finding.Name, manager.Paths.Display(finding.Path), finding.Detail)
	}
	return writer.Flush()
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
		if err := pa2skills.ValidateName(*name); err != nil {
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
	fmt.Fprintf(stdout, "Added %s\nSource:  %s\nTracked: %s\n\nReview and publish manually:\n  pa2-skills cd\n  git status\n", filepath.Base(target), source, manager.Paths.Display(target))
	return nil
}

// targetFlags registers the --scope and --harness flags of commands that change installations.
func targetFlags(flags *flag.FlagSet) (*string, *string) {
	scope := flags.String("scope", environmentDefault("PA2_SKILLS_SCOPE", string(pa2skills.ScopeUser)), "user or project")
	harnesses := flags.String("harness", environmentDefault("PA2_SKILLS_HARNESS", ""), "comma-separated harnesses, or all")
	return scope, harnesses
}

// filterFlags registers the --scope and --harness flags of commands that only
// inspect installations. They default to every scope and harness.
func filterFlags(flags *flag.FlagSet) (*string, *string) {
	scope := flags.String("scope", "", "user or project")
	harnesses := flags.String("harness", "", "comma-separated harnesses, or all")
	return scope, harnesses
}

func parseFilter(command string, arguments []string, register func(*flag.FlagSet)) (pa2skills.Filter, error) {
	flags := newFlagSet(command)
	scope, harnesses := filterFlags(flags)
	if register != nil {
		register(flags)
	}
	names, err := parseArguments(flags, arguments)
	if err != nil {
		return pa2skills.Filter{}, err
	}
	filter := pa2skills.Filter{Names: names, Scope: pa2skills.Scope(*scope), Harnesses: pa2skills.ExpandHarnesses(splitValues(*harnesses))}
	return filter, invalidArguments(pa2skills.ValidateFilter(filter))
}

func removeItems(arguments []string, manager pa2skills.Manager) error {
	flags := newFlagSet("remove")
	scope, harnesses := targetFlags(flags)
	force := flags.Bool("force", false, "remove installations with local changes")
	dryRun := flags.Bool("dry-run", false, "report what would change without changing files")
	names, err := parseArguments(flags, arguments)
	if err != nil {
		return err
	}
	options := pa2skills.RemoveOptions{
		Scope:     pa2skills.Scope(*scope),
		Harnesses: pa2skills.ExpandHarnesses(splitValues(*harnesses)),
		Force:     *force,
		DryRun:    *dryRun,
	}
	if err := invalidArguments(pa2skills.ValidateRemove(names, options)); err != nil {
		return err
	}
	return manager.Remove(names, options)
}

func installItems(arguments []string, manager pa2skills.Manager) error {
	flags := newFlagSet("install")
	scope, harnesses := targetFlags(flags)
	conflict := flags.String("conflict", "ask", "ask, overwrite, or skip")
	dryRun := flags.Bool("dry-run", false, "report what would change without changing files")
	names, err := parseArguments(flags, arguments)
	if err != nil {
		return err
	}
	options := pa2skills.InstallOptions{
		Scope:     pa2skills.Scope(*scope),
		Harnesses: pa2skills.ExpandHarnesses(splitValues(*harnesses)),
		Conflict:  pa2skills.ConflictPolicy(*conflict),
		DryRun:    *dryRun,
	}
	if err := invalidArguments(pa2skills.ValidateInstall(names, options)); err != nil {
		return err
	}
	return manager.Install(names, options)
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
	validationErrors = append(validationErrors, pa2skills.ValidateConflictPolicy(policy))
	if err := invalidArguments(validationErrors...); err != nil {
		return err
	}
	if !*skillsOnly {
		fmt.Fprintln(stdout, "Binary: checking for updates...")
		result, updated, err := updateBinary(version, *check, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "Binary: check failed: %v\n", err)
		} else {
			fmt.Fprintf(stdout, "Binary: %s\n", result)
		}
		if updated != "" && !*binOnly {
			// This process is still the old binary; hand the rest of the
			// update to the new one so source and skills use the new code.
			arguments := []string{updated, "update", "--skills-only", "--conflict", *conflict}
			if err := execBinary(updated, arguments, os.Environ()); err != nil {
				return fmt.Errorf("start updated binary: %w", err)
			}
			return nil
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
	fmt.Fprintln(stdout, "Source: synchronizing checkout and installed skills and rules...")
	return manager.UpdateAll(policy)
}

func listItems(manager pa2skills.Manager, stdout io.Writer) error {
	items, err := manager.Items()
	if err != nil {
		return err
	}
	placements, err := manager.Placements()
	if err != nil {
		return err
	}
	writer := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "KIND\tNAME\tINSTALLED")
	for _, item := range items {
		fmt.Fprintf(writer, "%s\t%s\t%s\n", item.Kind, item.Name, formatPlacement(placements[item.Name]))
	}
	return writer.Flush()
}

// formatPlacement renders user-scope harnesses as is and current-project ones
// after "project:", for example "claude, codex; project: claude".
func formatPlacement(placement pa2skills.Placement) string {
	var parts []string
	if len(placement.User) > 0 {
		parts = append(parts, strings.Join(placement.User, ", "))
	}
	if len(placement.Project) > 0 {
		parts = append(parts, "project: "+strings.Join(placement.Project, ", "))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, "; ")
}

func showStatus(arguments []string, manager pa2skills.Manager, stdout io.Writer) error {
	var verbose bool
	filter, err := parseFilter("status", arguments, func(flags *flag.FlagSet) {
		flags.BoolVar(&verbose, "verbose", false, "one row per installation, with its target")
		flags.BoolVar(&verbose, "v", false, "one row per installation, with its target")
	})
	if err != nil {
		return err
	}
	statuses, err := manager.Status(filter)
	if err != nil {
		return err
	}
	if len(statuses) == 0 {
		fmt.Fprintln(stdout, "No managed installations.")
		return nil
	}
	writer := tabwriter.NewWriter(stdout, 0, 0, 2, ' ', 0)
	if verbose {
		fmt.Fprintln(writer, "STATUS\tKIND\tNAME\tSCOPE\tHARNESS\tTARGET")
		for _, status := range statuses {
			installation := status.Installation
			fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\n", status.Status, installation.Kind, installation.Name, installation.Scope, installation.Harness, manager.Paths.Display(installation.Target))
		}
	} else {
		fmt.Fprintln(writer, "STATUS\tKIND\tNAME\tSCOPE\tHARNESSES")
		for _, row := range groupStatuses(statuses) {
			scope := row.Installation.Scope
			if row.Installation.ProjectRoot != "" {
				scope += " " + manager.Paths.Display(row.Installation.ProjectRoot)
			}
			fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\n", row.Status, row.Installation.Kind, row.Installation.Name, scope, strings.Join(row.harnesses, ", "))
		}
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	printStatusSummary(stdout, statuses)
	return nil
}

type statusRow struct {
	pa2skills.InstallationStatus
	harnesses []string
}

// groupStatuses merges installations of one item at one location that share a
// status into a single row listing their harnesses.
func groupStatuses(statuses []pa2skills.InstallationStatus) []statusRow {
	type key struct{ status, name, scope, projectRoot string }
	var rows []statusRow
	index := map[key]int{}
	for _, status := range statuses {
		installation := status.Installation
		rowKey := key{status.Status, installation.Name, installation.Scope, installation.ProjectRoot}
		if position, found := index[rowKey]; found {
			rows[position].harnesses = append(rows[position].harnesses, installation.Harness)
			continue
		}
		index[rowKey] = len(rows)
		rows = append(rows, statusRow{status, []string{installation.Harness}})
	}
	return rows
}

func printStatusSummary(stdout io.Writer, statuses []pa2skills.InstallationStatus) {
	counts := map[string]int{}
	for _, status := range statuses {
		counts[status.Status]++
	}
	var parts []string
	for _, status := range pa2skills.Statuses {
		if counts[status] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[status], status))
		}
	}
	fmt.Fprintf(stdout, "\n%d installation(s): %s.\n", len(statuses), strings.Join(parts, ", "))
	if counts[pa2skills.StatusOutdated] > 0 {
		fmt.Fprintln(stdout, "Run 'pa2-skills update' to refresh outdated installations.")
	}
	if counts[pa2skills.StatusModified]+counts[pa2skills.StatusDiverged] > 0 {
		fmt.Fprintln(stdout, "Run 'pa2-skills diff' to see local changes.")
	}
	if counts[pa2skills.StatusMissing] > 0 {
		fmt.Fprintln(stdout, "Run 'pa2-skills install <name>' to restore missing installations, or 'pa2-skills remove <name>' to stop tracking them.")
	}
	if counts[pa2skills.StatusOrphaned] > 0 {
		fmt.Fprintln(stdout, "Run 'pa2-skills remove <name>' to stop tracking orphaned installations.")
	}
}

func showDiff(arguments []string, manager pa2skills.Manager) error {
	filter, err := parseFilter("diff", arguments, nil)
	if err != nil {
		return err
	}
	return manager.Diff(filter)
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
	if os.IsNotExist(err) && len(arguments) == 0 {
		return fmt.Errorf("the managed source checkout %s does not exist; run the bootstrap script", sourceRoot)
	}
	if os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist in the managed source checkout at %s", arguments[0], sourceRoot)
	}
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
