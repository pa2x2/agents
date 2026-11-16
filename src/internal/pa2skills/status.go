package pa2skills

import (
	"errors"
	"fmt"
	"sort"
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

// Statuses lists every status in the order summaries report them.
var Statuses = []string{StatusCurrent, StatusOutdated, StatusModified, StatusDiverged, StatusMissing, StatusOrphaned}

// Filter selects managed installations. Empty fields match every installation.
type Filter struct {
	Names     []string
	Scope     Scope
	Harnesses []string
}

func ValidateFilter(filter Filter) error {
	var validationErrors []error
	for _, name := range filter.Names {
		if err := ValidateName(name); err != nil {
			validationErrors = append(validationErrors, err)
		}
	}
	if filter.Scope != "" {
		validationErrors = append(validationErrors, validateScope(filter.Scope))
	}
	validationErrors = append(validationErrors, validateHarnesses(filter.Harnesses))
	return errors.Join(validationErrors...)
}

// Status reports every managed installation the filter selects.
func (m Manager) Status(filter Filter) ([]InstallationStatus, error) {
	installations, err := m.selectInstallations(filter)
	if err != nil {
		return nil, err
	}
	result := make([]InstallationStatus, 0, len(installations))
	for _, stored := range installations {
		status, _, err := m.installationStatus(stored.Installation)
		if err != nil {
			return nil, err
		}
		result = append(result, InstallationStatus{Installation: stored.Installation, Status: status})
	}
	return result, nil
}

// Diff shows how each selected installation that is not current differs from
// the source checkout, as a diff from the installed copy to the source.
func (m Manager) Diff(filter Filter) error {
	installations, err := m.selectInstallations(filter)
	if err != nil {
		return err
	}
	shown := 0
	for _, stored := range installations {
		installation := stored.Installation
		status, item, err := m.installationStatus(installation)
		if err != nil {
			return err
		}
		if status == StatusCurrent {
			continue
		}
		if shown > 0 {
			fmt.Fprintln(m.Stdout)
		}
		shown++
		fmt.Fprintf(m.Stdout, "== %s for %s at %s: %s\n", installation.Name, installation.Harness, m.Paths.Display(installation.Target), status)
		switch status {
		case StatusMissing:
			fmt.Fprintf(m.Stdout, "The installed copy was deleted; run 'pa2-skills install %s' to restore it.\n", installation.Name)
		case StatusOrphaned:
			fmt.Fprintf(m.Stdout, "No longer in the source checkout; run 'pa2-skills remove %s' to stop tracking it.\n", installation.Name)
		default:
			if err := handlerFor(installation.Kind).diff(m.Stdout, installation.Target, item.Source, installation.Name); err != nil {
				return err
			}
		}
	}
	if shown == 0 {
		fmt.Fprintln(m.Stdout, "Every selected installation matches the source checkout.")
	}
	return nil
}

// ManagedNames lists the names of managed installations without inspecting their targets.
func (m Manager) ManagedNames() ([]string, error) {
	installations, err := m.Paths.installations()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var names []string
	for _, stored := range installations {
		if name := stored.Installation.Name; !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// selectInstallations applies the filter. A name that is neither available nor
// managed is an error, so a typo does not read as "nothing installed".
func (m Manager) selectInstallations(filter Filter) ([]storedInstallation, error) {
	installations, err := m.Paths.installations()
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	if len(filter.Names) > 0 {
		items, err := m.Items()
		if err != nil {
			return nil, err
		}
		known := map[string]bool{}
		var candidates []string
		for _, item := range items {
			known[item.Name] = true
			candidates = append(candidates, item.Name)
		}
		for _, stored := range installations {
			if name := stored.Installation.Name; !known[name] {
				known[name] = true
				candidates = append(candidates, name)
			}
		}
		for _, name := range filter.Names {
			if !known[name] {
				return nil, unknownNameError(name, candidates)
			}
			names[name] = true
		}
	}
	harnesses := map[string]bool{}
	for _, harness := range filter.Harnesses {
		harnesses[harness] = true
	}
	var result []storedInstallation
	for _, stored := range installations {
		installation := stored.Installation
		if len(names) > 0 && !names[installation.Name] ||
			filter.Scope != "" && installation.Scope != string(filter.Scope) ||
			len(harnesses) > 0 && !harnesses[installation.Harness] {
			continue
		}
		result = append(result, stored)
	}
	return result, nil
}

// installationStatus also returns the source item, which is empty for missing and orphaned installations.
func (m Manager) installationStatus(installation Installation) (string, Item, error) {
	handler := handlerFor(installation.Kind)
	localHash, err := handler.targetHash(installation.Target, installation.Name)
	if err != nil {
		return "", Item{}, err
	}
	if localHash == "" {
		return StatusMissing, Item{}, nil
	}
	item, err := m.findItem(installation.Name)
	if errors.Is(err, errUnknownItem) || err == nil && item.Kind != installation.Kind {
		return StatusOrphaned, Item{}, nil
	}
	if err != nil {
		return "", Item{}, err
	}
	sourceHash, err := handler.sourceHash(item.Source)
	if err != nil {
		return "", Item{}, err
	}
	switch {
	case localHash == sourceHash:
		return StatusCurrent, item, nil
	case localHash == installation.Baseline:
		return StatusOutdated, item, nil
	case sourceHash == installation.Baseline:
		return StatusModified, item, nil
	default:
		return StatusDiverged, item, nil
	}
}
