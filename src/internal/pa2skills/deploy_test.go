package pa2skills

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallCopiesSkillAndStoresPrivateState(t *testing.T) {
	manager, paths := testManager(t)
	writeSkill(t, paths.SourceRoot, "example", "first")

	if err := manager.Install("example", ScopeUser, []string{"codex"}, ConflictAsk); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(paths.Home, ".agents", "skills", "example", "SKILL.md")
	if got := readFile(t, target); got != "first" {
		t.Fatalf("target = %q, want first", got)
	}
	states, err := filepath.Glob(filepath.Join(paths.StateRoot(), "installations", "*.json"))
	if err != nil || len(states) != 1 {
		t.Fatalf("state files = %v, %v", states, err)
	}
	if _, err := os.Stat(filepath.Join(paths.Home, ".agents", "skills", "example", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}

func TestInstallRefreshesUnmodifiedTarget(t *testing.T) {
	manager, paths := testManager(t)
	writeSkill(t, paths.SourceRoot, "example", "first")
	if err := manager.Install("example", ScopeUser, []string{"codex"}, ConflictAsk); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, paths.SourceRoot, "example", "second")
	if err := manager.Install("example", ScopeUser, []string{"codex"}, ConflictAsk); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(paths.Home, ".agents", "skills", "example", "SKILL.md")); got != "second" {
		t.Fatalf("target = %q, want second", got)
	}
}

func TestInstallPreservesLocalOnlyChange(t *testing.T) {
	manager, paths := testManager(t)
	writeSkill(t, paths.SourceRoot, "example", "first")
	if err := manager.Install("example", ScopeUser, []string{"codex"}, ConflictAsk); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(paths.Home, ".agents", "skills", "example", "SKILL.md")
	writeFile(t, target, "local")
	if err := manager.Install("example", ScopeUser, []string{"codex"}, ConflictAsk); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, target); got != "local" {
		t.Fatalf("target = %q, want local", got)
	}
}

func TestInstallRespectsConflictPolicy(t *testing.T) {
	manager, paths := testManager(t)
	writeSkill(t, paths.SourceRoot, "example", "first")
	if err := manager.Install("example", ScopeUser, []string{"codex"}, ConflictAsk); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(paths.Home, ".agents", "skills", "example", "SKILL.md")
	writeFile(t, target, "local")
	writeSkill(t, paths.SourceRoot, "example", "remote")
	if err := manager.Install("example", ScopeUser, []string{"codex"}, ConflictSkip); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, target); got != "local" {
		t.Fatalf("skipped target = %q, want local", got)
	}
	if err := manager.Install("example", ScopeUser, []string{"codex"}, ConflictOverwrite); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, target); got != "remote" {
		t.Fatalf("overwritten target = %q, want remote", got)
	}
}

func TestInstallAdoptsExistingCustomizedTarget(t *testing.T) {
	manager, paths := testManager(t)
	writeSkill(t, paths.SourceRoot, "example", "remote")
	target := filepath.Join(paths.Home, ".agents", "skills", "example", "SKILL.md")
	writeFile(t, target, "team-customized")

	if err := manager.Install("example", ScopeUser, []string{"codex"}, ConflictSkip); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, target); got != "team-customized" {
		t.Fatalf("target = %q, want team-customized", got)
	}
	if err := manager.Install("example", ScopeUser, []string{"codex"}, ConflictSkip); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, target); got != "team-customized" {
		t.Fatalf("managed target = %q, want team-customized", got)
	}
}

func testManager(t *testing.T) (Manager, Paths) {
	t.Helper()
	root := t.TempDir()
	paths := Paths{
		Home:       filepath.Join(root, "home"),
		DataHome:   filepath.Join(root, "data"),
		StateHome:  filepath.Join(root, "state"),
		ConfigHome: filepath.Join(root, "config"),
		SourceRoot: filepath.Join(root, "source"),
	}
	return Manager{Paths: paths, Stdin: strings.NewReader(""), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}, paths
}

func writeSkill(t *testing.T, source, name, contents string) {
	t.Helper()
	writeFile(t, filepath.Join(source, "skills", name, "SKILL.md"), contents)
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}
