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

// Status reports every managed installation, optionally limited to the named skills and rules.
func (m Manager) Status(names []string) ([]InstallationStatus, error) {
	installations, err := m.Paths.installations()
	if err != nil {
		return nil, err
	}
	selected := map[string]bool{}
	for _, name := range names {
		selected[name] = true
	}
	var result []InstallationStatus
	for _, stored := range installations {
		installation := stored.Installation
		if len(selected) > 0 && !selected[installation.Name] {
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
	handler := handlerFor(installation.Kind)
	localHash, err := handler.targetHash(installation.Target, installation.Name)
	if err != nil {
		return "", err
	}
	if localHash == "" {
		return StatusMissing, nil
	}
	item, err := m.findItem(installation.Name)
	if errors.Is(err, errUnknownItem) || err == nil && item.Kind != installation.Kind {
		return StatusOrphaned, nil
	}
	if err != nil {
		return "", err
	}
	sourceHash, err := handler.sourceHash(item.Source)
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
