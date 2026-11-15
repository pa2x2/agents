package pa2skills

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Installation is the private record of one deployed item. The name keeps its
// original "skill" JSON key, and records written before rules existed have no
// kind, so readInstallation treats them as skills.
type Installation struct {
	Version     int       `json:"version"`
	Scope       string    `json:"scope"`
	Harness     string    `json:"harness"`
	Kind        Kind      `json:"kind,omitempty"`
	Name        string    `json:"skill"`
	ProjectRoot string    `json:"project_root,omitempty"`
	Target      string    `json:"target"`
	SourceRef   string    `json:"source_ref"`
	Baseline    string    `json:"baseline"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (p Paths) readInstallation(key string) (Installation, bool, error) {
	contents, err := os.ReadFile(p.InstallStatePath(key))
	if os.IsNotExist(err) {
		return Installation{}, false, nil
	}
	if err != nil {
		return Installation{}, false, err
	}
	installation, err := decodeInstallation(contents)
	if err != nil {
		return Installation{}, false, fmt.Errorf("read installation state: %w", err)
	}
	return installation, true, nil
}

func decodeInstallation(contents []byte) (Installation, error) {
	var installation Installation
	if err := json.Unmarshal(contents, &installation); err != nil {
		return Installation{}, err
	}
	if installation.Kind == "" {
		installation.Kind = KindSkill
	}
	return installation, nil
}

func (p Paths) writeInstallation(key string, installation Installation) error {
	directory := filepath.Dir(p.InstallStatePath(key))
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	contents, err := json.MarshalIndent(installation, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(p.InstallStatePath(key), append(contents, '\n'), 0o600)
}

type storedInstallation struct {
	Key          string
	Installation Installation
}

func (p Paths) installations() ([]storedInstallation, error) {
	entries, err := os.ReadDir(p.InstallationsRoot())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read installation state: %w", err)
	}
	result := make([]storedInstallation, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(p.InstallationsRoot(), entry.Name()))
		if err != nil {
			return nil, err
		}
		installation, err := decodeInstallation(contents)
		if err != nil {
			return nil, fmt.Errorf("read installation state %s: %w", entry.Name(), err)
		}
		result = append(result, storedInstallation{strings.TrimSuffix(entry.Name(), ".json"), installation})
	}
	sort.SliceStable(result, func(i, j int) bool {
		left, right := result[i].Installation, result[j].Installation
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		if left.Scope != right.Scope {
			return left.Scope > right.Scope
		}
		if left.ProjectRoot != right.ProjectRoot {
			return left.ProjectRoot < right.ProjectRoot
		}
		return left.Harness < right.Harness
	})
	return result, nil
}

func (p Paths) saveBaseline(handler kindHandler, source string) (string, error) {
	hash, err := handler.sourceHash(source)
	if err != nil {
		return "", err
	}
	target := p.BaselinePath(hash)
	if _, err := os.Stat(target); err == nil {
		return hash, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := handler.saveBaseline(source, target); err != nil {
		return "", fmt.Errorf("save baseline: %w", err)
	}
	return hash, nil
}

func (p Paths) removeInstallation(key string) error {
	if err := os.Remove(p.InstallStatePath(key)); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// pruneBaselines deletes baselines that no installation references.
func (p Paths) pruneBaselines() error {
	installations, err := p.installations()
	if err != nil {
		return err
	}
	referenced := map[string]bool{}
	for _, stored := range installations {
		referenced[stored.Installation.Baseline] = true
	}
	root := p.BaselinesRoot()
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !referenced[entry.Name()] {
			if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
				return err
			}
		}
	}
	return nil
}
