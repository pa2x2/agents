package pa2skills

import (
	"errors"
	"fmt"
)

// Remove deletes managed installations and their private state. Installations with local
// changes are kept unless force is set; files pa2-skills did not install are never touched.
// A rule block shared by several harnesses stays until the last of them is removed.
func (m Manager) Remove(names []string, scope Scope, harnesses []string, force bool) error {
	if err := ValidateInstallArguments(names, scope, harnesses, ConflictAsk); err != nil {
		return err
	}
	projectRoot, err := m.projectRoot(scope)
	if err != nil {
		return err
	}
	installations, err := m.Paths.installations()
	if err != nil {
		return err
	}
	selectedNames := map[string]bool{}
	for _, name := range names {
		selectedNames[name] = true
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
			return err
		}
		trackers[deployed]++
	}
	selectedHarnesses := map[string]bool{}
	for _, harness := range harnesses {
		selectedHarnesses[harness] = true
	}
	matched := map[string]bool{}
	for _, stored := range installations {
		installation := stored.Installation
		if installation.Scope != string(scope) || installation.ProjectRoot != projectRoot || !selectedHarnesses[installation.Harness] {
			continue
		}
		if !selectedNames[NameAll] && !selectedNames[installation.Name] {
			continue
		}
		matched[installation.Name] = true
		handler := handlerFor(installation.Kind)
		localHash, err := handler.targetHash(installation.Target, installation.Name)
		if err != nil {
			return err
		}
		if localHash != "" && localHash != installation.Baseline && !force {
			fmt.Fprintf(m.Stdout, "Kept locally customized %s for %s at %s; re-run with --force to remove it\n", installation.Name, installation.Harness, installation.Target)
			continue
		}
		deployed, err := deploymentOf(installation)
		if err != nil {
			return err
		}
		shared := trackers[deployed] > 1
		if !shared {
			if err := handler.remove(installation.Target, installation.Name); err != nil {
				return fmt.Errorf("remove %s from %s: %w", installation.Name, installation.Target, err)
			}
		}
		if err := m.Paths.removeInstallation(stored.Key); err != nil {
			return err
		}
		trackers[deployed]--
		switch {
		case localHash == "":
			fmt.Fprintf(m.Stdout, "Stopped tracking missing %s for %s at %s\n", installation.Name, installation.Harness, installation.Target)
		case shared:
			fmt.Fprintf(m.Stdout, "Stopped tracking %s for %s; kept it at %s for the other harnesses that use it\n", installation.Name, installation.Harness, installation.Target)
		default:
			fmt.Fprintf(m.Stdout, "Removed %s for %s at %s\n", installation.Name, installation.Harness, installation.Target)
		}
	}
	var missing []error
	for _, name := range names {
		if name != NameAll && !matched[name] {
			missing = append(missing, fmt.Errorf("%s has no managed %s-scope installation for the selected harnesses", name, scope))
		}
	}
	if len(matched) == 0 && len(missing) == 0 {
		missing = append(missing, fmt.Errorf("no managed %s-scope installations for the selected harnesses", scope))
	}
	if err := m.Paths.pruneBaselines(); err != nil {
		return err
	}
	return errors.Join(missing...)
}
