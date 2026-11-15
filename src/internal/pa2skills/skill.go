package pa2skills

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// skillHandler deploys a skill as a copy of its source directory.
type skillHandler struct{}

func (skillHandler) kind() Kind { return KindSkill }

func (skillHandler) names(sourceRoot string) ([]string, error) {
	root := filepath.Join(sourceRoot, "skills")
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read skills source: %w", err)
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(root, entry.Name(), "SKILL.md")); err == nil {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

func (skillHandler) sourcePath(sourceRoot, name string) string {
	return filepath.Join(sourceRoot, "skills", name)
}

func (skillHandler) targetPath(paths Paths, name string, scope Scope, harness, projectRoot string) (string, error) {
	if scope == ScopeUser {
		switch harness {
		case "codex":
			return filepath.Join(paths.Home, ".agents", "skills", name), nil
		case "claude":
			return filepath.Join(paths.Home, ".claude", "skills", name), nil
		case "opencode":
			return filepath.Join(paths.ConfigHome, "opencode", "skills", name), nil
		}
	}
	if scope == ScopeProject {
		switch harness {
		case "codex":
			return filepath.Join(projectRoot, ".agents", "skills", name), nil
		case "claude":
			return filepath.Join(projectRoot, ".claude", "skills", name), nil
		case "opencode":
			return filepath.Join(projectRoot, ".opencode", "skills", name), nil
		}
	}
	return "", fmt.Errorf("unsupported harness %q", harness)
}

func (skillHandler) sourceHash(source string) (string, error) { return treeHash(source) }

func (skillHandler) targetHash(target, _ string) (string, error) { return treeHash(target) }

func (skillHandler) write(source, target, _ string) error { return copyTree(source, target) }

func (skillHandler) remove(target, _ string) error { return os.RemoveAll(target) }

func (skillHandler) saveBaseline(source, destination string) error {
	return copyTree(source, destination)
}

func (skillHandler) diff(output io.Writer, target, source, _ string) error {
	return showDiff(output, target, source)
}
