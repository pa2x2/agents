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

func (m Manager) SkillNames() ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(m.Paths.SourceRoot, "skills"))
	if err != nil {
		return nil, fmt.Errorf("read skills source: %w", err)
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(m.Paths.SourceRoot, "skills", entry.Name(), "SKILL.md")); err == nil {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func (m Manager) Install(skill string, scope Scope, harnesses []string, policy ConflictPolicy) error {
	projectRoot, err := m.projectRoot(scope)
	if err != nil {
		return err
	}
	source, ref, err := m.skillSource(skill)
	if err != nil {
		return err
	}
	if err := validateHarnesses(harnesses); err != nil {
		return err
	}
	activePolicy := policy
	for _, harness := range harnesses {
		target, err := m.targetPath(skill, scope, harness, projectRoot)
		if err != nil {
			return err
		}
		key := projectStateKey(projectRoot, string(scope), harness, skill)
		current, known, err := m.Paths.readInstallation(key)
		if err != nil {
			return err
		}
		if known {
			if err := m.updateTarget(current, key, source, ref, &activePolicy); err != nil {
				return err
			}
			continue
		}
		if currentHash, err := treeHash(target); err != nil {
			return err
		} else if currentHash != "" {
			installation := Installation{
				Version:     1,
				Scope:       string(scope),
				Harness:     harness,
				Skill:       skill,
				ProjectRoot: projectRoot,
				Target:      target,
				SourceRef:   ref,
			}
			sourceHash, err := treeHash(source)
			if err != nil {
				return err
			}
			if currentHash == sourceHash {
				if err := m.materialize(installation, key, source); err != nil {
					return err
				}
				fmt.Fprintf(m.Stdout, "Adopted matching %s for %s at %s\n", skill, harness, target)
				continue
			}
			decision, err := m.resolveUnmanagedConflict(target, source, activePolicy)
			if err != nil {
				return err
			}
			if decision == ConflictOverwrite {
				if err := m.materialize(installation, key, source); err != nil {
					return err
				}
				fmt.Fprintf(m.Stdout, "Replaced unmanaged %s for %s at %s\n", skill, harness, target)
				continue
			}
			baseline, err := m.Paths.saveBaseline(source)
			if err != nil {
				return err
			}
			installation.Baseline = baseline
			installation.UpdatedAt = time.Now().UTC()
			if err := m.Paths.writeInstallation(key, installation); err != nil {
				return err
			}
			fmt.Fprintf(m.Stdout, "Adopted locally customized %s for %s at %s\n", skill, harness, target)
			continue
		}
		if err := m.materialize(Installation{
			Version:     1,
			Scope:       string(scope),
			Harness:     harness,
			Skill:       skill,
			ProjectRoot: projectRoot,
			Target:      target,
			SourceRef:   ref,
		}, key, source); err != nil {
			return err
		}
		fmt.Fprintf(m.Stdout, "Installed %s for %s at %s\n", skill, harness, target)
	}
	return nil
}

func (m Manager) Sync(skill string, scope Scope, harnesses []string, policy ConflictPolicy) error {
	if err := m.refreshSource(); err != nil {
		return err
	}
	return m.Install(skill, scope, harnesses, policy)
}

func (m Manager) UpdateAll(policy ConflictPolicy) error {
	if err := m.refreshSource(); err != nil {
		return err
	}
	installations, err := m.Paths.installations()
	if err != nil {
		return err
	}
	activePolicy := policy
	for _, stored := range installations {
		installation := stored.Installation
		source, ref, err := m.skillSource(installation.Skill)
		if err != nil {
			return fmt.Errorf("sync %s at %s: %w", installation.Skill, installation.Target, err)
		}
		if err := m.updateTarget(installation, stored.Key, source, ref, &activePolicy); err != nil {
			return err
		}
	}
	fmt.Fprintf(m.Stdout, "Skills: synchronized %d managed installation(s)\n", len(installations))
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

func (m Manager) updateTarget(installation Installation, key, source, ref string, policy *ConflictPolicy) error {
	localHash, err := treeHash(installation.Target)
	if err != nil {
		return err
	}
	sourceHash, err := treeHash(source)
	if err != nil {
		return err
	}
	if localHash == sourceHash {
		return nil
	}
	if localHash == installation.Baseline {
		return m.materialize(installationWithRef(installation, ref), key, source)
	}
	if sourceHash == installation.Baseline {
		fmt.Fprintf(m.Stdout, "Kept locally customized %s at %s\n", installation.Skill, installation.Target)
		return nil
	}
	decision, err := m.resolveConflict(installation, source, *policy)
	if err != nil {
		return err
	}
	if decision == ConflictAllOverwrite {
		*policy = ConflictOverwrite
		decision = ConflictOverwrite
	}
	if decision == ConflictSkip {
		fmt.Fprintf(m.Stdout, "Skipped remote changes for %s at %s\n", installation.Skill, installation.Target)
		return nil
	}
	return m.materialize(installationWithRef(installation, ref), key, source)
}

func (m Manager) materialize(installation Installation, key, source string) error {
	if err := copyTree(source, installation.Target); err != nil {
		return err
	}
	baseline, err := m.Paths.saveBaseline(source)
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
	reader := bufio.NewReader(m.Stdin)
	for {
		fmt.Fprintf(m.Stdout, "%s has changed since pa2-skills last wrote it.\n> diff / overwrite / all-overwrite / skip / quit\n", installation.Target)
		choice, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		switch strings.TrimSpace(choice) {
		case "diff":
			if err := showDiff(m.Stdout, installation.Target, source); err != nil {
				return "", err
			}
		case "overwrite":
			return ConflictOverwrite, nil
		case "all-overwrite":
			return ConflictAllOverwrite, nil
		case "skip":
			return ConflictSkip, nil
		case "quit", "":
			return "", errors.New("update cancelled")
		default:
			fmt.Fprintln(m.Stdout, "Choose diff, overwrite, all-overwrite, skip, or quit.")
		}
	}
}

func (m Manager) resolveUnmanagedConflict(target, source string, policy ConflictPolicy) (ConflictPolicy, error) {
	if policy == ConflictOverwrite || policy == ConflictSkip {
		return policy, nil
	}
	if !isTerminal(m.Stdin) {
		return "", fmt.Errorf("%s exists but has no private pa2-skills state; re-run with --conflict=overwrite or --conflict=skip", target)
	}
	reader := bufio.NewReader(m.Stdin)
	for {
		fmt.Fprintf(m.Stdout, "%s exists but was not installed by this machine.\n> diff / overwrite / skip / quit\n", target)
		choice, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		switch strings.TrimSpace(choice) {
		case "diff":
			if err := showDiff(m.Stdout, target, source); err != nil {
				return "", err
			}
		case "overwrite":
			return ConflictOverwrite, nil
		case "skip":
			return ConflictSkip, nil
		case "quit", "":
			return "", errors.New("installation cancelled")
		default:
			fmt.Fprintln(m.Stdout, "Choose diff, overwrite, skip, or quit.")
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

func (m Manager) skillSource(skill string) (string, string, error) {
	source := filepath.Join(m.Paths.SourceRoot, "skills", skill)
	if _, err := os.Stat(filepath.Join(source, "SKILL.md")); err != nil {
		if os.IsNotExist(err) {
			return "", "", fmt.Errorf("unknown skill %q", skill)
		}
		return "", "", err
	}
	ref := "local"
	if output, err := exec.Command("git", "-C", m.Paths.SourceRoot, "rev-parse", "HEAD").Output(); err == nil {
		ref = strings.TrimSpace(string(output))
	}
	return source, ref, nil
}

func (m Manager) targetPath(skill string, scope Scope, harness, projectRoot string) (string, error) {
	if scope == ScopeUser {
		switch harness {
		case "codex":
			return filepath.Join(m.Paths.Home, ".agents", "skills", skill), nil
		case "claude":
			return filepath.Join(m.Paths.Home, ".claude", "skills", skill), nil
		case "opencode":
			return filepath.Join(m.Paths.ConfigHome, "opencode", "skills", skill), nil
		}
	}
	if scope == ScopeProject {
		switch harness {
		case "codex":
			return filepath.Join(projectRoot, ".agents", "skills", skill), nil
		case "claude":
			return filepath.Join(projectRoot, ".claude", "skills", skill), nil
		case "opencode":
			return filepath.Join(projectRoot, ".opencode", "skills", skill), nil
		}
	}
	return "", fmt.Errorf("unsupported harness %q", harness)
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

func validateHarnesses(harnesses []string) error {
	if len(harnesses) == 0 {
		return errors.New("at least one --harness value is required")
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
			return fmt.Errorf("unsupported harness %q (supported: %s)", harness, strings.Join(Harnesses, ", "))
		}
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
	if !ok {
		return false
	}
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
