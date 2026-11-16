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

	"golang.org/x/term"
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
	ConflictAsk       ConflictPolicy = "ask"
	ConflictOverwrite ConflictPolicy = "overwrite"
	ConflictSkip      ConflictPolicy = "skip"
	// Answers at a conflict prompt that also decide the rest of the run's conflicts.
	conflictAllOverwrite ConflictPolicy = "all-overwrite"
	conflictAllSkip      ConflictPolicy = "skip-all"
)

type Manager struct {
	Paths  Paths
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

type InstallOptions struct {
	Scope Scope
	// Harnesses may be empty: Install then uses the harnesses pa2-skills
	// already manages, or else the ones whose configuration directory exists.
	Harnesses []string
	// Conflict also decides locally customized installations whose source has
	// not changed: overwrite replaces them, ask and skip keep them.
	Conflict ConflictPolicy
	DryRun   bool
}

func ValidateInstall(names []string, options InstallOptions) error {
	return errors.Join(
		validateNames(names),
		validateScope(options.Scope),
		validateHarnesses(options.Harnesses),
		ValidateConflictPolicy(options.Conflict),
	)
}

func (m Manager) Install(names []string, options InstallOptions) error {
	if err := ValidateInstall(names, options); err != nil {
		return err
	}
	projectRoot, err := m.projectRoot(options.Scope)
	if err != nil {
		return err
	}
	items, err := m.selectItems(names)
	if err != nil {
		return err
	}
	harnesses := options.Harnesses
	if len(harnesses) == 0 {
		if harnesses, err = m.defaultHarnesses(); err != nil {
			return err
		}
	}
	r := m.newRun(options.Conflict, options.DryRun)
	r.replaceCustomized = options.Conflict == ConflictOverwrite
	ref := m.sourceRef()
	return r.finish(func() error {
		for _, item := range items {
			if err := r.installItem(item, ref, options.Scope, harnesses, projectRoot); err != nil {
				return err
			}
		}
		return nil
	}())
}

// defaultHarnesses picks the harnesses pa2-skills already manages, or else the
// harnesses whose configuration directory exists on this machine.
func (m Manager) defaultHarnesses() ([]string, error) {
	installations, err := m.Paths.installations()
	if err != nil {
		return nil, err
	}
	selected := map[string]bool{}
	for _, stored := range installations {
		selected[stored.Installation.Harness] = true
	}
	reason := "already managed by pa2-skills"
	if len(selected) == 0 {
		reason = "detected on this machine"
		for _, harness := range Harnesses {
			if info, err := os.Stat(m.Paths.harnessHome(harness)); err == nil && info.IsDir() {
				selected[harness] = true
			}
		}
	}
	if len(selected) == 0 {
		return nil, fmt.Errorf("no harness was selected or detected; pass --harness %s or %s, or set PA2_SKILLS_HARNESS", strings.Join(Harnesses, ", "), HarnessAll)
	}
	var result []string
	for _, harness := range Harnesses {
		if selected[harness] {
			result = append(result, harness)
		}
	}
	fmt.Fprintf(m.Stdout, "Using %s (%s; pass --harness to choose)\n", strings.Join(result, ", "), reason)
	return result, nil
}

// selectItems resolves names to items, expanding NameAll to every item and removing duplicates.
func (m Manager) selectItems(names []string) ([]Item, error) {
	available, err := m.Items()
	if err != nil {
		return nil, err
	}
	byName := map[string]Item{}
	var candidates []string
	for _, item := range available {
		byName[item.Name] = item
		candidates = append(candidates, item.Name)
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
			return nil, unknownNameError(name, candidates)
		}
		add(item)
	}
	return result, nil
}

// run carries one install, update or removal across all of its items.
type run struct {
	Manager
	// policy starts as the requested policy; an all-overwrite or skip-all
	// answer at a prompt replaces it for the rest of the run.
	policy            ConflictPolicy
	replaceCustomized bool
	dryRun            bool
	unresolved        int
	report            *report
}

func (m Manager) newRun(policy ConflictPolicy, dryRun bool) *run {
	return &run{
		Manager: m,
		policy:  policy,
		dryRun:  dryRun,
		report:  &report{paths: m.Paths, out: m.Stdout, dryRun: dryRun, tally: map[string]int{}},
	}
}

// finish prints what the run did and turns conflicts it could not resolve into an error.
func (r *run) finish(err error) error {
	if err != nil {
		r.report.flush()
		return err
	}
	r.report.summary()
	if r.unresolved > 0 {
		return fmt.Errorf("%d conflict(s) left unresolved; re-run in a terminal to choose, or pass --conflict=overwrite to take the source version or --conflict=skip to keep yours", r.unresolved)
	}
	return nil
}

func (r *run) installItem(item Item, ref string, scope Scope, harnesses []string, projectRoot string) error {
	handler := handlerFor(item.Kind)
	for _, harness := range harnesses {
		target, err := handler.targetPath(r.Paths, item.Name, scope, harness, projectRoot)
		if err != nil {
			return err
		}
		key := projectStateKey(projectRoot, string(scope), harness, item.Name)
		current, known, err := r.Paths.readInstallation(key)
		if err != nil {
			return err
		}
		if known && current.Kind != item.Kind {
			return fmt.Errorf("%s is tracked as a %s for %s at %s but is now a %s; run pa2-skills remove %s first", item.Name, current.Kind, harness, r.Paths.Display(current.Target), item.Kind, item.Name)
		}
		currentHash, err := handler.targetHash(target, item.Name)
		if err != nil {
			return err
		}
		if known && currentHash != "" {
			if err := r.updateTarget(current, key, item.Source, ref); err != nil {
				return err
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
		result, err := r.installTarget(installation, key, item.Source, currentHash)
		if err != nil {
			return err
		}
		r.report.add(installation, result)
	}
	return nil
}

// installTarget deploys an item to a target pa2-skills does not track yet.
// currentHash is "" when nothing is at the target.
func (r *run) installTarget(installation Installation, key, source, currentHash string) (outcome, error) {
	if currentHash == "" {
		return outcomeInstalled, r.materialize(installation, key, source)
	}
	sourceHash, err := handlerFor(installation.Kind).sourceHash(source)
	if err != nil {
		return outcome{}, err
	}
	if currentHash == sourceHash {
		return outcomeAdopted, r.materialize(installation, key, source)
	}
	message := fmt.Sprintf("%s for %s at %s already exists and isn't managed by pa2-skills.", installation.Name, installation.Harness, r.Paths.Display(installation.Target))
	decision, err := r.resolve(installation, source, message)
	if err != nil {
		return outcome{}, err
	}
	switch decision {
	case ConflictOverwrite:
		return outcomeReplacedUnmanaged, r.materialize(installation, key, source)
	case ConflictSkip:
		return outcomeAdoptedCustomized, r.track(installation, key, source)
	}
	r.unresolved++
	return outcomeUnmanagedConflict, nil
}

func (m Manager) UpdateAll(policy ConflictPolicy) error {
	if err := ValidateConflictPolicy(policy); err != nil {
		return err
	}
	if err := m.refreshSource(); err != nil {
		return err
	}
	installations, err := m.Paths.installations()
	if err != nil {
		return err
	}
	if len(installations) == 0 {
		fmt.Fprintln(m.Stdout, "No managed installations to update.")
		return nil
	}
	ref := m.sourceRef()
	r := m.newRun(policy, false)
	r.report.hideCurrent = true
	return r.finish(func() error {
		for _, stored := range installations {
			installation := stored.Installation
			localHash, err := handlerFor(installation.Kind).targetHash(installation.Target, installation.Name)
			if err != nil {
				return err
			}
			if localHash == "" {
				r.report.add(installation, outcomeMissing)
				continue
			}
			item, err := r.findItem(installation.Name)
			if errors.Is(err, errUnknownItem) || err == nil && item.Kind != installation.Kind {
				r.report.add(installation, outcomeOrphaned)
				continue
			}
			if err != nil {
				return fmt.Errorf("update %s at %s: %w", installation.Name, installation.Target, err)
			}
			if err := r.updateTarget(installation, stored.Key, item.Source, ref); err != nil {
				return err
			}
		}
		return nil
	}())
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

// updateTarget refreshes a managed installation that is present at its target.
func (r *run) updateTarget(installation Installation, key, source, ref string) error {
	handler := handlerFor(installation.Kind)
	localHash, err := handler.targetHash(installation.Target, installation.Name)
	if err != nil {
		return err
	}
	sourceHash, err := handler.sourceHash(source)
	if err != nil {
		return err
	}
	updated := installation
	updated.SourceRef = ref
	var result outcome
	switch {
	case localHash == sourceHash:
		result = outcomeCurrent
	case localHash == installation.Baseline:
		result, err = outcomeUpdated, r.materialize(updated, key, source)
	case sourceHash == installation.Baseline && r.replaceCustomized:
		result, err = outcomeReplacedCustomized, r.materialize(updated, key, source)
	case sourceHash == installation.Baseline:
		result = outcomeKeptCustomized
	default:
		message := fmt.Sprintf("%s for %s at %s has local changes, and the source changed too.", installation.Name, installation.Harness, r.Paths.Display(installation.Target))
		var decision ConflictPolicy
		if decision, err = r.resolve(installation, source, message); err != nil {
			return err
		}
		switch decision {
		case ConflictOverwrite:
			result, err = outcomeReplacedCustomized, r.materialize(updated, key, source)
		case ConflictSkip:
			result = outcomeSkippedSource
		default:
			r.unresolved++
			result = outcomeConflict
		}
	}
	if err != nil {
		return err
	}
	r.report.add(installation, result)
	return nil
}

// materialize writes the source to the target and records it as the baseline.
func (r *run) materialize(installation Installation, key, source string) error {
	if r.dryRun {
		return nil
	}
	if err := handlerFor(installation.Kind).write(source, installation.Target, installation.Name); err != nil {
		return err
	}
	return r.track(installation, key, source)
}

// track records the installation with the source as its baseline, leaving the target as it is.
func (r *run) track(installation Installation, key, source string) error {
	if r.dryRun {
		return nil
	}
	baseline, err := r.Paths.saveBaseline(handlerFor(installation.Kind), source)
	if err != nil {
		return err
	}
	installation.Baseline = baseline
	installation.UpdatedAt = time.Now().UTC()
	return r.Paths.writeInstallation(key, installation)
}

// resolve decides a conflict by the run's policy, asking when the policy is
// ask and stdin is a terminal. It returns ConflictAsk when the conflict stays
// unresolved: in a dry run, or when there is nobody to ask.
func (r *run) resolve(installation Installation, source, message string) (ConflictPolicy, error) {
	if r.policy != ConflictAsk {
		return r.policy, nil
	}
	if r.dryRun || !isTerminal(r.Stdin) {
		return ConflictAsk, nil
	}
	r.report.flush()
	decision, err := r.promptConflict(message, installation, source)
	switch decision {
	case conflictAllOverwrite:
		r.policy = ConflictOverwrite
	case conflictAllSkip:
		r.policy = ConflictSkip
	default:
		return decision, err
	}
	return r.policy, nil
}

// promptConflict asks how to handle a conflicting target. Each choice can be
// typed in full or by the shortcut letter shown in brackets.
func (m Manager) promptConflict(message string, installation Installation, source string) (ConflictPolicy, error) {
	reader := bufio.NewReader(m.Stdin)
	for {
		fmt.Fprintf(m.Stdout, "%s\n> [d]iff  [o]verwrite  overwrite [a]ll  [s]kip (keep yours)  s[k]ip all  [q]uit: ", message)
		choice, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		choice = strings.ToLower(strings.TrimSpace(choice))
		if choice == "" && err != nil {
			// End of input, such as Ctrl-D.
			choice = "q"
		}
		switch choice {
		case "d", "diff":
			if err := handlerFor(installation.Kind).diff(m.Stdout, installation.Target, source, installation.Name); err != nil {
				return "", err
			}
		case "o", "overwrite":
			return ConflictOverwrite, nil
		case "a", "all-overwrite":
			return conflictAllOverwrite, nil
		case "s", "skip":
			return ConflictSkip, nil
		case "k", "skip-all":
			return conflictAllSkip, nil
		case "q", "quit":
			return "", errors.New("cancelled")
		default:
			fmt.Fprintln(m.Stdout, "Choose d, o, a, s, k or q.")
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

// SourceIsClean reports whether the source checkout has no uncommitted
// changes; update refuses to pull over them.
func (m Manager) SourceIsClean() (bool, error) {
	status, err := exec.Command("git", "-C", m.Paths.SourceRoot, "status", "--porcelain").Output()
	if err != nil {
		return false, fmt.Errorf("inspect source checkout: %w", err)
	}
	return strings.TrimSpace(string(status)) == "", nil
}

func (m Manager) refreshSource() error {
	before, _ := exec.Command("git", "-C", m.Paths.SourceRoot, "rev-parse", "--short", "HEAD").Output()
	clean, err := m.SourceIsClean()
	if err != nil {
		return err
	}
	if !clean {
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

func validateNames(names []string) error {
	if len(names) == 0 {
		return errors.New("at least one skill or rule name is required")
	}
	var validationErrors []error
	for _, name := range names {
		if name == NameAll {
			continue
		}
		if err := ValidateName(name); err != nil {
			validationErrors = append(validationErrors, err)
		}
	}
	return errors.Join(validationErrors...)
}

func validateScope(scope Scope) error {
	if scope != ScopeUser && scope != ScopeProject {
		return errors.New("--scope must be user or project")
	}
	return nil
}

// validateHarnesses rejects unsupported harnesses; an empty list is valid.
func validateHarnesses(harnesses []string) error {
	var validationErrors []error
	for _, harness := range harnesses {
		valid := false
		for _, supported := range Harnesses {
			valid = valid || harness == supported
		}
		if !valid {
			message := fmt.Sprintf("unsupported harness %q (supported: %s, %s)", harness, strings.Join(Harnesses, ", "), HarnessAll)
			if best := ClosestName(harness, Harnesses); best != "" {
				message += fmt.Sprintf("; did you mean %q?", best)
			}
			validationErrors = append(validationErrors, errors.New(message))
		}
	}
	return errors.Join(validationErrors...)
}

func ValidateConflictPolicy(policy ConflictPolicy) error {
	if policy != ConflictAsk && policy != ConflictOverwrite && policy != ConflictSkip {
		return errors.New("--conflict must be ask, overwrite, or skip")
	}
	return nil
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
	return ok && term.IsTerminal(int(file.Fd()))
}
