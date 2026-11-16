package pa2skills

import (
	"errors"
	"fmt"
)

type RemoveOptions struct {
	Scope Scope
	// Harnesses may be empty to remove the items from every harness.
	Harnesses []string
	// Force also removes installations with local changes.
	Force  bool
	DryRun bool
}

func ValidateRemove(names []string, options RemoveOptions) error {
	return errors.Join(validateNames(names), validateScope(options.Scope), validateHarnesses(options.Harnesses))
}

// Remove deletes managed installations and their private state. Installations with local
// changes are kept unless forced; files pa2-skills did not install are never touched.
// A rule block shared by several harnesses stays until the last of them is removed.
func (m Manager) Remove(names []string, options RemoveOptions) error {
	if err := ValidateRemove(names, options); err != nil {
		return err
	}
	projectRoot, err := m.projectRoot(options.Scope)
	if err != nil {
		return err
	}
	installations, err := m.Paths.installations()
	if err != nil {
		return err
	}
	r := m.newRun(ConflictAsk, options.DryRun)
	matched, err := r.remove(names, options, projectRoot, installations)
	if err := r.finish(err); err != nil {
		return err
	}
	var problems []error
	for _, name := range names {
		if name == NameAll || matched[name] {
			continue
		}
		message := fmt.Sprintf("%s has no managed %s-scope installation%s", name, options.Scope, harnessSuffix(options.Harnesses))
		var candidates []string
		for _, stored := range installations {
			candidates = append(candidates, stored.Installation.Name)
		}
		if best := ClosestName(name, candidates); best != "" {
			message += fmt.Sprintf("; did you mean %q?", best)
		}
		problems = append(problems, errors.New(message))
	}
	if len(matched) == 0 && len(problems) == 0 {
		problems = append(problems, fmt.Errorf("no managed %s-scope installations%s", options.Scope, harnessSuffix(options.Harnesses)))
	}
	if !options.DryRun {
		if err := m.Paths.pruneBaselines(); err != nil {
			return err
		}
	}
	return errors.Join(problems...)
}

func harnessSuffix(harnesses []string) string {
	if len(harnesses) == 0 {
		return ""
	}
	return " for the selected harnesses"
}

// remove deletes the selected installations and returns the names it matched.
func (r *run) remove(names []string, options RemoveOptions, projectRoot string, installations []storedInstallation) (map[string]bool, error) {
	selectedNames := map[string]bool{}
	for _, name := range names {
		selectedNames[name] = true
	}
	selectedHarnesses := map[string]bool{}
	for _, harness := range options.Harnesses {
		selectedHarnesses[harness] = true
	}
	// Targets are compared after resolving symlinks, so a CLAUDE.md that links
	// to AGENTS.md counts as the same file.
	type deployment struct{ kind, target, name string }
	deploymentOf := func(installation Installation) (deployment, error) {
		target, err := resolveSymlink(installation.Target)
		return deployment{string(installation.Kind), target, installation.Name}, err
	}
	trackers := map[deployment]int{}
	for _, stored := range installations {
		deployed, err := deploymentOf(stored.Installation)
		if err != nil {
			return nil, err
		}
		trackers[deployed]++
	}
	matched := map[string]bool{}
	for _, stored := range installations {
		installation := stored.Installation
		if installation.Scope != string(options.Scope) || installation.ProjectRoot != projectRoot {
			continue
		}
		if len(selectedHarnesses) > 0 && !selectedHarnesses[installation.Harness] {
			continue
		}
		if !selectedNames[NameAll] && !selectedNames[installation.Name] {
			continue
		}
		matched[installation.Name] = true
		handler := handlerFor(installation.Kind)
		localHash, err := handler.targetHash(installation.Target, installation.Name)
		if err != nil {
			return nil, err
		}
		if localHash != "" && localHash != installation.Baseline && !options.Force {
			r.report.add(installation, outcomeKeptOnRemove)
			continue
		}
		deployed, err := deploymentOf(installation)
		if err != nil {
			return nil, err
		}
		shared := trackers[deployed] > 1
		if !r.dryRun {
			if !shared {
				if err := handler.remove(installation.Target, installation.Name); err != nil {
					return nil, fmt.Errorf("remove %s from %s: %w", installation.Name, installation.Target, err)
				}
			}
			if err := r.Paths.removeInstallation(stored.Key); err != nil {
				return nil, err
			}
		}
		trackers[deployed]--
		switch {
		case localHash == "":
			r.report.add(installation, outcomeUntrackedMissing)
		case shared:
			r.report.add(installation, outcomeUntrackedShared)
		default:
			r.report.add(installation, outcomeRemoved)
		}
	}
	return matched, nil
}
