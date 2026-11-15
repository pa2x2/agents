package pa2skills

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var Harnesses = []string{"claude", "codex", "opencode"}

// HarnessAll selects every supported harness.
const HarnessAll = "all"

// NameAll selects every skill and rule in the managed source checkout.
const NameAll = "all"

type Scope string

const (
	ScopeUser    Scope = "user"
	ScopeProject Scope = "project"
)

type ConflictPolicy string

const (
	ConflictAsk          ConflictPolicy = "ask"
	ConflictOverwrite    ConflictPolicy = "overwrite"
	ConflictSkip         ConflictPolicy = "skip"
	ConflictAllOverwrite ConflictPolicy = "all-overwrite"
)

type Manager struct {
	Paths  Paths
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

func (m Manager) Install(names []string, scope Scope, harnesses []string, policy ConflictPolicy) error {
	if err := ValidateInstallArguments(names, scope, harnesses, policy); err != nil {
		return err
	}
	projectRoot, err := m.projectRoot(scope)
	if err != nil {
		return err
	}
	items, err := m.selectItems(names)
	if err != nil {
		return err
	}
	ref := m.sourceRef()
	activePolicy := policy
	for _, item := range items {
		if err := m.installItem(item, ref, scope, harnesses, projectRoot, &activePolicy); err != nil {
			return err
		}
	}
	return nil
}

// selectItems resolves names to items, expanding NameAll to every item and removing duplicates.
func (m Manager) selectItems(names []string) ([]Item, error) {
	available, err := m.Items()
	if err != nil {
		return nil, err
	}
	byName := map[string]Item{}
	for _, item := range available {
		byName[item.Name] = item
	}
	seen := map[string]bool{}
	var result []Item
	add := func(item Item) {
		if !seen[item.Name] {
			seen[item.Name] = true
			result = append(result, item)
		}
	}
	for _, name := range names {
		if name == NameAll {
			for _, item := range available {
				add(item)
			}
			continue
		}
		item, found := byName[name]
		if !found {
			return nil, fmt.Errorf("%w %q", errUnknownItem, name)
		}
		add(item)
	}
	return result, nil
}

func (m Manager) installItem(item Item, ref string, scope Scope, harnesses []string, projectRoot string, activePolicy *ConflictPolicy) error {
	handler := handlerFor(item.Kind)
	for _, harness := range harnesses {
		target, err := handler.targetPath(m.Paths, item.Name, scope, harness, projectRoot)
		if err != nil {
			return err
		}
		key := projectStateKey(projectRoot, string(scope), harness, item.Name)
		current, known, err := m.Paths.readInstallation(key)
		if err != nil {
			return err
		}
		if known && current.Kind != item.Kind {
			return fmt.Errorf("%s is tracked as a %s for %s at %s but is now a %s; run pa2-skills remove %s first", item.Name, current.Kind, harness, current.Target, item.Kind, item.Name)
		}
		currentHash, err := handler.targetHash(target, item.Name)
		if err != nil {
			return err
		}
		if known && currentHash != "" {
			upToDate, err := m.updateTarget(current, key, item.Source, ref, activePolicy)
			if err != nil {
				return err
			}
			if upToDate {
				fmt.Fprintf(m.Stdout, "Already current: %s for %s at %s\n", item.Name, harness, target)
			}
			continue
		}
		installation := Installation{
			Version:     1,
			Scope:       string(scope),
			Harness:     harness,
			Kind:        item.Kind,
			Name:        item.Name,
			ProjectRoot: projectRoot,
			Target:      target,
			SourceRef:   ref,
		}
		if currentHash != "" {
			sourceHash, err := handler.sourceHash(item.Source)
			if err != nil {
				return err
			}
			if currentHash == sourceHash {
				if err := m.materialize(installation, key, item.Source); err != nil {
					return err
				}
				fmt.Fprintf(m.Stdout, "Adopted matching %s for %s at %s\n", item.Name, harness, target)
				continue
			}
			decision, err := m.resolveUnmanagedConflict(installation, item.Source, *activePolicy)
			if err != nil {
				return err
			}
			if decision == ConflictAllOverwrite {
				*activePolicy = ConflictOverwrite
				decision = ConflictOverwrite
			}
			if decision == ConflictOverwrite {
				if err := m.materialize(installation, key, item.Source); err != nil {
					return err
				}
				fmt.Fprintf(m.Stdout, "Replaced unmanaged %s for %s at %s\n", item.Name, harness, target)
				continue
			}
			baseline, err := m.Paths.saveBaseline(handler, item.Source)
			if err != nil {
				return err
			}
			installation.Baseline = baseline
			installation.UpdatedAt = time.Now().UTC()
			if err := m.Paths.writeInstallation(key, installation); err != nil {
				return err
			}
			fmt.Fprintf(m.Stdout, "Adopted locally customized %s for %s at %s\n", item.Name, harness, target)
			continue
		}
		if err := m.materialize(installation, key, item.Source); err != nil {
			return err
		}
		fmt.Fprintf(m.Stdout, "Installed %s %s for %s at %s\n", item.Kind, item.Name, harness, target)
	}
	return nil
}

func (m Manager) Sync(names []string, scope Scope, harnesses []string, policy ConflictPolicy) error {
	if err := ValidateInstallArguments(names, scope, harnesses, policy); err != nil {
		return err
	}
	if err := m.refreshSource(); err != nil {
		return err
	}
	return m.Install(names, scope, harnesses, policy)
}

func (m Manager) UpdateAll(policy ConflictPolicy) error {
	if err := m.refreshSource(); err != nil {
		return err
	}
	installations, err := m.Paths.installations()
	if err != nil {
		return err
	}
	ref := m.sourceRef()
	activePolicy := policy
	for _, stored := range installations {
		installation := stored.Installation
		localHash, err := handlerFor(installation.Kind).targetHash(installation.Target, installation.Name)
		if err != nil {
			return err
		}
		if localHash == "" {
			fmt.Fprintf(m.Stdout, "Missing %s for %s at %s; run pa2-skills install to restore it or pa2-skills remove to stop tracking it\n", installation.Name, installation.Harness, installation.Target)
			continue
		}
		item, err := m.findItem(installation.Name)
		if errors.Is(err, errUnknownItem) || err == nil && item.Kind != installation.Kind {
			fmt.Fprintf(m.Stdout, "Skipped %s for %s at %s: no longer in the source checkout; run pa2-skills remove to stop tracking it\n", installation.Name, installation.Harness, installation.Target)
			continue
		}
		if err != nil {
			return fmt.Errorf("sync %s at %s: %w", installation.Name, installation.Target, err)
		}
		if _, err := m.updateTarget(installation, stored.Key, item.Source, ref, &activePolicy); err != nil {
			return err
		}
	}
	fmt.Fprintf(m.Stdout, "Skills and rules: synchronized %d managed installation(s)\n", len(installations))
	return nil
}

func (m Manager) CheckSource() (string, error) {
	local, err := exec.Command("git", "-C", m.Paths.SourceRoot, "rev-parse", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("inspect source checkout: %w", err)
	}
	remote, err := exec.Command("git", "-C", m.Paths.SourceRoot, "ls-remote", "origin", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("check remote source: %w", err)
	}
	fields := strings.Fields(string(remote))
	if len(fields) == 0 {
		return "", errors.New("remote source did not return HEAD")
	}
	localRef := strings.TrimSpace(string(local))
	if fields[0] == localRef {
		return fmt.Sprintf("already current at %.7s", localRef), nil
	}
	return fmt.Sprintf("update available: %.7s -> %.7s", localRef, fields[0]), nil
}

// updateTarget refreshes a managed installation and reports whether it already matched the source.
func (m Manager) updateTarget(installation Installation, key, source, ref string, policy *ConflictPolicy) (bool, error) {
	handler := handlerFor(installation.Kind)
	localHash, err := handler.targetHash(installation.Target, installation.Name)
	if err != nil {
		return false, err
	}
	sourceHash, err := handler.sourceHash(source)
	if err != nil {
		return false, err
	}
	if localHash == sourceHash {
		return true, nil
	}
	if localHash == installation.Baseline {
		return false, m.materializeUpdate(installation, key, source, ref)
	}
	if sourceHash == installation.Baseline {
		fmt.Fprintf(m.Stdout, "Kept locally customized %s for %s at %s\n", installation.Name, installation.Harness, installation.Target)
		return false, nil
	}
	decision, err := m.resolveConflict(installation, source, *policy)
	if err != nil {
		return false, err
	}
	if decision == ConflictAllOverwrite {
		*policy = ConflictOverwrite
		decision = ConflictOverwrite
	}
	if decision == ConflictSkip {
		fmt.Fprintf(m.Stdout, "Skipped remote changes for %s for %s at %s\n", installation.Name, installation.Harness, installation.Target)
		return false, nil
	}
	return false, m.materializeUpdate(installation, key, source, ref)
}

func (m Manager) materializeUpdate(installation Installation, key, source, ref string) error {
	if err := m.materialize(installationWithRef(installation, ref), key, source); err != nil {
		return err
	}
	fmt.Fprintf(m.Stdout, "Updated %s for %s at %s\n", installation.Name, installation.Harness, installation.Target)
	return nil
}

func (m Manager) materialize(installation Installation, key, source string) error {
	handler := handlerFor(installation.Kind)
	if err := handler.write(source, installation.Target, installation.Name); err != nil {
		return err
	}
	baseline, err := m.Paths.saveBaseline(handler, source)
	if err != nil {
		return err
	}
	installation.Baseline = baseline
	installation.UpdatedAt = time.Now().UTC()
	if err := m.Paths.writeInstallation(key, installation); err != nil {
		return err
	}
	return nil
}

func (m Manager) resolveConflict(installation Installation, source string, policy ConflictPolicy) (ConflictPolicy, error) {
	if policy == ConflictOverwrite || policy == ConflictSkip {
		return policy, nil
	}
	if !isTerminal(m.Stdin) {
		return "", fmt.Errorf("%s has local and remote changes; re-run with --conflict=overwrite or --conflict=skip", installation.Target)
	}
	return m.promptConflict(installation.Name+" at "+installation.Target+" has changed since pa2-skills last wrote it.", installation, source, "update cancelled")
}

func (m Manager) resolveUnmanagedConflict(installation Installation, source string, policy ConflictPolicy) (ConflictPolicy, error) {
	if policy == ConflictOverwrite || policy == ConflictSkip {
		return policy, nil
	}
	if !isTerminal(m.Stdin) {
		return "", fmt.Errorf("%s at %s exists but has no private pa2-skills state; re-run with --conflict=overwrite or --conflict=skip", installation.Name, installation.Target)
	}
	return m.promptConflict(installation.Name+" at "+installation.Target+" exists but was not installed by this machine.", installation, source, "installation cancelled")
}

// promptConflict asks how to handle a conflicting target. Each choice can be
// typed in full or by the shortcut letter shown in brackets.
func (m Manager) promptConflict(message string, installation Installation, source, cancelled string) (ConflictPolicy, error) {
	reader := bufio.NewReader(m.Stdin)
	for {
		fmt.Fprintf(m.Stdout, "%s\n> [d]iff / [o]verwrite / [a]ll-overwrite / [s]kip / [q]uit\n", message)
		choice, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		switch strings.ToLower(strings.TrimSpace(choice)) {
		case "d", "diff":
			if err := handlerFor(installation.Kind).diff(m.Stdout, installation.Target, source, installation.Name); err != nil {
				return "", err
			}
		case "o", "overwrite":
			return ConflictOverwrite, nil
		case "a", "all-overwrite":
			return ConflictAllOverwrite, nil
		case "s", "skip":
			return ConflictSkip, nil
		case "q", "quit", "":
			return "", errors.New(cancelled)
		default:
			fmt.Fprintln(m.Stdout, "Choose d(iff), o(verwrite), a(ll-overwrite), s(kip), or q(uit).")
		}
	}
}

func (m Manager) projectRoot(scope Scope) (string, error) {
	if scope == ScopeUser {
		return "", nil
	}
	output, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", errors.New("project scope requires running inside a Git worktree")
	}
	return filepath.EvalSymlinks(strings.TrimSpace(string(output)))
}

// sourceRef is the source checkout commit recorded with each installation.
func (m Manager) sourceRef() string {
	output, err := exec.Command("git", "-C", m.Paths.SourceRoot, "rev-parse", "HEAD").Output()
	if err != nil {
		return "local"
	}
	return strings.TrimSpace(string(output))
}

func (m Manager) refreshSource() error {
	before, _ := exec.Command("git", "-C", m.Paths.SourceRoot, "rev-parse", "--short", "HEAD").Output()
	status, err := exec.Command("git", "-C", m.Paths.SourceRoot, "status", "--porcelain").Output()
	if err != nil {
		return fmt.Errorf("inspect source checkout: %w", err)
	}
	if strings.TrimSpace(string(status)) != "" {
		return errors.New("source checkout has local changes; use pa2-skills cd to review, commit, or stash them before updating")
	}
	command := exec.Command("git", "-C", m.Paths.SourceRoot, "pull", "--ff-only")
	command.Stderr = m.Stderr
	if err := command.Run(); err != nil {
		return err
	}
	after, _ := exec.Command("git", "-C", m.Paths.SourceRoot, "rev-parse", "--short", "HEAD").Output()
	beforeRef := strings.TrimSpace(string(before))
	afterRef := strings.TrimSpace(string(after))
	if beforeRef == afterRef {
		fmt.Fprintf(m.Stdout, "Source: already current at %s\n", afterRef)
	} else {
		fmt.Fprintf(m.Stdout, "Source: updated %s -> %s\n", beforeRef, afterRef)
	}
	return nil
}

func installationWithRef(installation Installation, ref string) Installation {
	installation.SourceRef = ref
	return installation
}

// ExpandHarnesses replaces HarnessAll with every supported harness and removes duplicates.
func ExpandHarnesses(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		expanded := []string{value}
		if value == HarnessAll {
			expanded = Harnesses
		}
		for _, harness := range expanded {
			if !seen[harness] {
				seen[harness] = true
				result = append(result, harness)
			}
		}
	}
	sort.Strings(result)
	return result
}

func ValidateInstallArguments(names []string, scope Scope, harnesses []string, policy ConflictPolicy) error {
	var validationErrors []error
	if len(names) == 0 {
		validationErrors = append(validationErrors, errors.New("at least one skill or rule name is required"))
	}
	for _, name := range names {
		if name == NameAll {
			continue
		}
		if err := ValidateName(name); err != nil {
			validationErrors = append(validationErrors, err)
		}
	}
	if scope != ScopeUser && scope != ScopeProject {
		validationErrors = append(validationErrors, errors.New("--scope must be user or project"))
	}
	if policy != ConflictAsk && policy != ConflictOverwrite && policy != ConflictSkip {
		validationErrors = append(validationErrors, errors.New("--conflict must be ask, overwrite, or skip"))
	}
	if len(harnesses) == 0 {
		validationErrors = append(validationErrors, errors.New("at least one --harness value is required"))
	}
	seen := map[string]bool{}
	for _, harness := range harnesses {
		if seen[harness] {
			continue
		}
		seen[harness] = true
		valid := false
		for _, supported := range Harnesses {
			valid = valid || harness == supported
		}
		if !valid {
			validationErrors = append(validationErrors, fmt.Errorf("unsupported harness %q (supported: %s, %s)", harness, strings.Join(Harnesses, ", "), HarnessAll))
		}
	}
	return errors.Join(validationErrors...)
}

func showDiff(output io.Writer, current, source string) error {
	command := exec.Command("git", "diff", "--no-index", "--", current, source)
	command.Stdout = output
	command.Stderr = output
	err := command.Run()
	var exitError *exec.ExitError
	if errors.As(err, &exitError) && exitError.ExitCode() == 1 {
		return nil
	}
	return err
}

func isTerminal(reader io.Reader) bool {
	file, ok := reader.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
