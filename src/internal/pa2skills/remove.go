package pa2skills

import (
	"errors"
	"fmt"
	"os"
)

// Remove deletes managed installations and their private state. Installations with local
// changes are kept unless force is set; directories pa2-skills did not install are never touched.
func (m Manager) Remove(skills []string, scope Scope, harnesses []string, force bool) error {
	if err := ValidateInstallArguments(skills, scope, harnesses, ConflictAsk); err != nil {
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
	selectedSkills := map[string]bool{}
	for _, skill := range skills {
		selectedSkills[skill] = true
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
		if !selectedSkills[SkillAll] && !selectedSkills[installation.Skill] {
			continue
		}
		matched[installation.Skill] = true
		localHash, err := treeHash(installation.Target)
		if err != nil {
			return err
		}
		if localHash != "" && localHash != installation.Baseline && !force {
			fmt.Fprintf(m.Stdout, "Kept locally customized %s for %s at %s; re-run with --force to remove it\n", installation.Skill, installation.Harness, installation.Target)
			continue
		}
		if err := os.RemoveAll(installation.Target); err != nil {
			return fmt.Errorf("remove %s: %w", installation.Target, err)
		}
		if err := m.Paths.removeInstallation(stored.Key); err != nil {
			return err
		}
		if localHash == "" {
			fmt.Fprintf(m.Stdout, "Stopped tracking missing %s for %s at %s\n", installation.Skill, installation.Harness, installation.Target)
		} else {
			fmt.Fprintf(m.Stdout, "Removed %s for %s at %s\n", installation.Skill, installation.Harness, installation.Target)
		}
	}
	var missing []error
	for _, skill := range skills {
		if skill != SkillAll && !matched[skill] {
			missing = append(missing, fmt.Errorf("%s has no managed %s-scope installation for the selected harnesses", skill, scope))
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
