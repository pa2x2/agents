package pa2skills

import (
	"errors"
)

// InstallationStatus describes how a managed installation compares with the local source checkout.
type InstallationStatus struct {
	Installation Installation
	Status       string
}

const (
	StatusCurrent  = "current"
	StatusOutdated = "outdated"
	StatusModified = "modified"
	StatusDiverged = "diverged"
	StatusMissing  = "missing"
	StatusOrphaned = "orphaned"
)

// Status reports every managed installation, optionally limited to the named skills.
func (m Manager) Status(skills []string) ([]InstallationStatus, error) {
	installations, err := m.Paths.installations()
	if err != nil {
		return nil, err
	}
	selected := map[string]bool{}
	for _, skill := range skills {
		selected[skill] = true
	}
	var result []InstallationStatus
	for _, stored := range installations {
		installation := stored.Installation
		if len(selected) > 0 && !selected[installation.Skill] {
			continue
		}
		status, err := m.installationStatus(installation)
		if err != nil {
			return nil, err
		}
		result = append(result, InstallationStatus{Installation: installation, Status: status})
	}
	return result, nil
}

func (m Manager) installationStatus(installation Installation) (string, error) {
	localHash, err := treeHash(installation.Target)
	if err != nil {
		return "", err
	}
	if localHash == "" {
		return StatusMissing, nil
	}
	source, _, err := m.skillSource(installation.Skill)
	if errors.Is(err, errUnknownSkill) {
		return StatusOrphaned, nil
	}
	if err != nil {
		return "", err
	}
	sourceHash, err := treeHash(source)
	if err != nil {
		return "", err
	}
	switch {
	case localHash == sourceHash:
		return StatusCurrent, nil
	case localHash == installation.Baseline:
		return StatusOutdated, nil
	case sourceHash == installation.Baseline:
		return StatusModified, nil
	default:
		return StatusDiverged, nil
	}
}
