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

type Installation struct {
	Version     int       `json:"version"`
	Scope       string    `json:"scope"`
	Harness     string    `json:"harness"`
	Skill       string    `json:"skill"`
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
	var installation Installation
	if err := json.Unmarshal(contents, &installation); err != nil {
		return Installation{}, false, fmt.Errorf("read installation state: %w", err)
	}
	return installation, true, nil
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
	temporary, err := os.CreateTemp(directory, ".installation-")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if _, err := temporary.Write(append(contents, '\n')); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Chmod(0o600); err != nil {
		temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryName, p.InstallStatePath(key))
}

func (p Paths) installations() ([]struct {
	Key          string
	Installation Installation
}, error) {
	entries, err := os.ReadDir(p.InstallationsRoot())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read installation state: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	result := make([]struct {
		Key          string
		Installation Installation
	}, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		contents, err := os.ReadFile(filepath.Join(p.InstallationsRoot(), entry.Name()))
		if err != nil {
			return nil, err
		}
		var installation Installation
		if err := json.Unmarshal(contents, &installation); err != nil {
			return nil, fmt.Errorf("read installation state %s: %w", entry.Name(), err)
		}
		result = append(result, struct {
			Key          string
			Installation Installation
		}{strings.TrimSuffix(entry.Name(), ".json"), installation})
	}
	return result, nil
}

func (p Paths) saveBaseline(source string) (string, error) {
	hash, err := treeHash(source)
	if err != nil {
		return "", err
	}
	target := p.BaselinePath(hash)
	if _, err := os.Stat(target); err == nil {
		return hash, nil
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := copyTree(source, target); err != nil {
		return "", fmt.Errorf("save baseline: %w", err)
	}
	return hash, nil
}
