package pa2skills

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallCopiesSkillAndStoresPrivateState(t *testing.T) {
	manager, paths := testManager(t)
	writeSkill(t, paths.SourceRoot, "example", "first")

	if err := manager.Install([]string{"example"}, ScopeUser, []string{"codex"}, ConflictAsk); err != nil {
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
	if err := manager.Install([]string{"example"}, ScopeUser, []string{"codex"}, ConflictAsk); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, paths.SourceRoot, "example", "second")
	if err := manager.Install([]string{"example"}, ScopeUser, []string{"codex"}, ConflictAsk); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(paths.Home, ".agents", "skills", "example", "SKILL.md")); got != "second" {
		t.Fatalf("target = %q, want second", got)
	}
}

func TestInstallPreservesLocalOnlyChange(t *testing.T) {
	manager, paths := testManager(t)
	writeSkill(t, paths.SourceRoot, "example", "first")
	if err := manager.Install([]string{"example"}, ScopeUser, []string{"codex"}, ConflictAsk); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(paths.Home, ".agents", "skills", "example", "SKILL.md")
	writeFile(t, target, "local")
	if err := manager.Install([]string{"example"}, ScopeUser, []string{"codex"}, ConflictAsk); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, target); got != "local" {
		t.Fatalf("target = %q, want local", got)
	}
}

func TestInstallRespectsConflictPolicy(t *testing.T) {
	manager, paths := testManager(t)
	writeSkill(t, paths.SourceRoot, "example", "first")
	if err := manager.Install([]string{"example"}, ScopeUser, []string{"codex"}, ConflictAsk); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(paths.Home, ".agents", "skills", "example", "SKILL.md")
	writeFile(t, target, "local")
	writeSkill(t, paths.SourceRoot, "example", "remote")
	if err := manager.Install([]string{"example"}, ScopeUser, []string{"codex"}, ConflictSkip); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, target); got != "local" {
		t.Fatalf("skipped target = %q, want local", got)
	}
	if err := manager.Install([]string{"example"}, ScopeUser, []string{"codex"}, ConflictOverwrite); err != nil {
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

	if err := manager.Install([]string{"example"}, ScopeUser, []string{"codex"}, ConflictSkip); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, target); got != "team-customized" {
		t.Fatalf("target = %q, want team-customized", got)
	}
	if err := manager.Install([]string{"example"}, ScopeUser, []string{"codex"}, ConflictSkip); err != nil {
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

func TestExpandHarnessesResolvesAll(t *testing.T) {
	got := strings.Join(ExpandHarnesses([]string{"codex", "all", "claude"}), ",")
	if want := "claude,codex,opencode"; got != want {
		t.Fatalf("harnesses = %q, want %q", got, want)
	}
}

func TestInstallAllInstallsEverySkill(t *testing.T) {
	manager, paths := testManager(t)
	writeSkill(t, paths.SourceRoot, "first", "one")
	writeSkill(t, paths.SourceRoot, "second", "two")

	if err := manager.Install([]string{"second", NameAll}, ScopeUser, []string{"claude"}, ConflictAsk); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{"first": "one", "second": "two"} {
		if got := readFile(t, filepath.Join(paths.Home, ".claude", "skills", name, "SKILL.md")); got != want {
			t.Fatalf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestInstallReportsCurrentInstallation(t *testing.T) {
	manager, paths := testManager(t)
	writeSkill(t, paths.SourceRoot, "example", "first")
	for range 2 {
		if err := manager.Install([]string{"example"}, ScopeUser, []string{"codex"}, ConflictAsk); err != nil {
			t.Fatal(err)
		}
	}
	if output := manager.Stdout.(*bytes.Buffer).String(); !strings.Contains(output, "Already current: example for codex") {
		t.Fatalf("output = %q, want an already-current report", output)
	}
}

func TestRemoveDeletesManagedInstallation(t *testing.T) {
	manager, paths := testManager(t)
	writeSkill(t, paths.SourceRoot, "example", "first")
	if err := manager.Install([]string{"example"}, ScopeUser, []string{"claude", "codex"}, ConflictAsk); err != nil {
		t.Fatal(err)
	}
	if err := manager.Remove([]string{"example"}, ScopeUser, []string{"claude"}, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(paths.Home, ".claude", "skills", "example")); !os.IsNotExist(err) {
		t.Fatalf("claude target still exists: %v", err)
	}
	if got := readFile(t, filepath.Join(paths.Home, ".agents", "skills", "example", "SKILL.md")); got != "first" {
		t.Fatalf("codex target = %q, want first", got)
	}
	if err := manager.Remove([]string{"example"}, ScopeUser, []string{"claude"}, false); err == nil {
		t.Fatal("removing an untracked installation succeeded")
	}
}

func TestRemoveKeepsLocalChangesUnlessForced(t *testing.T) {
	manager, paths := testManager(t)
	writeSkill(t, paths.SourceRoot, "example", "first")
	if err := manager.Install([]string{"example"}, ScopeUser, []string{"codex"}, ConflictAsk); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(paths.Home, ".agents", "skills", "example", "SKILL.md")
	writeFile(t, target, "local")
	if err := manager.Remove([]string{NameAll}, ScopeUser, Harnesses, false); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, target); got != "local" {
		t.Fatalf("target = %q, want local", got)
	}
	if err := manager.Remove([]string{NameAll}, ScopeUser, Harnesses, true); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target still exists: %v", err)
	}
	if baselines, _ := os.ReadDir(paths.BaselinesRoot()); len(baselines) != 0 {
		t.Fatalf("baselines = %d, want none", len(baselines))
	}
}

func TestInstallRestoresDeletedManagedTarget(t *testing.T) {
	manager, paths := testManager(t)
	writeSkill(t, paths.SourceRoot, "example", "first")
	if err := manager.Install([]string{"example"}, ScopeUser, []string{"codex"}, ConflictAsk); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(paths.Home, ".agents", "skills", "example")
	if err := os.RemoveAll(target); err != nil {
		t.Fatal(err)
	}
	if err := manager.Install([]string{"example"}, ScopeUser, []string{"codex"}, ConflictAsk); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(target, "SKILL.md")); got != "first" {
		t.Fatalf("target = %q, want first", got)
	}
}

func TestStatusClassifiesInstallations(t *testing.T) {
	manager, paths := testManager(t)
	writeSkill(t, paths.SourceRoot, "example", "first")
	if err := manager.Install([]string{"example"}, ScopeUser, []string{"claude", "codex", "opencode"}, ConflictAsk); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(paths.Home, ".claude", "skills", "example", "SKILL.md"), "local")
	if err := os.RemoveAll(filepath.Join(paths.ConfigHome, "opencode", "skills", "example")); err != nil {
		t.Fatal(err)
	}
	statuses, err := manager.Status(nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, status := range statuses {
		got[status.Installation.Harness] = status.Status
	}
	want := map[string]string{"claude": StatusModified, "codex": StatusCurrent, "opencode": StatusMissing}
	for harness, status := range want {
		if got[harness] != status {
			t.Fatalf("statuses = %v, want %v", got, want)
		}
	}
	writeSkill(t, paths.SourceRoot, "example", "second")
	if statuses, err = manager.Status([]string{"example"}); err != nil {
		t.Fatal(err)
	}
	for _, status := range statuses {
		got[status.Installation.Harness] = status.Status
	}
	if got["claude"] != StatusDiverged || got["codex"] != StatusOutdated {
		t.Fatalf("statuses after source change = %v", got)
	}
}

func TestPromptConflictAcceptsShortcuts(t *testing.T) {
	cases := map[string]ConflictPolicy{
		"o\n":             ConflictOverwrite,
		"O\n":             ConflictOverwrite,
		"overwrite\n":     ConflictOverwrite,
		"a\n":             ConflictAllOverwrite,
		"all-overwrite\n": ConflictAllOverwrite,
		"s\n":             ConflictSkip,
		"x\ns\n":          ConflictSkip,
	}
	for input, want := range cases {
		manager := Manager{Stdin: strings.NewReader(input), Stdout: &bytes.Buffer{}}
		got, err := manager.promptConflict("conflict", Installation{Kind: KindSkill}, "source", "cancelled")
		if err != nil || got != want {
			t.Errorf("input %q: got %q, %v; want %q", input, got, err, want)
		}
	}
	for _, input := range []string{"q\n", "quit\n", "\n"} {
		manager := Manager{Stdin: strings.NewReader(input), Stdout: &bytes.Buffer{}}
		if _, err := manager.promptConflict("conflict", Installation{Kind: KindSkill}, "source", "cancelled"); err == nil || err.Error() != "cancelled" {
			t.Errorf("input %q: err = %v, want cancelled", input, err)
		}
	}
}

func TestRuleBlockLeavesUserTextUntouched(t *testing.T) {
	manager, paths := testManager(t)
	writeFile(t, filepath.Join(paths.SourceRoot, "rules", "commits.md"), "# Commits\n\nfirst\n")
	target := filepath.Join(paths.Home, ".claude", "CLAUDE.md")
	original := "# Mine\n\nkeep this\n"
	writeFile(t, target, original)

	if err := manager.Install([]string{"commits"}, ScopeUser, []string{"claude"}, ConflictAsk); err != nil {
		t.Fatal(err)
	}
	want := original + "\n<!-- pa2-skills:begin commits -->\n# Commits\n\nfirst\n<!-- pa2-skills:end commits -->\n"
	if got := readFile(t, target); got != want {
		t.Fatalf("installed = %q, want %q", got, want)
	}

	writeFile(t, target, readFile(t, target)+"\nadded after\n")
	writeFile(t, filepath.Join(paths.SourceRoot, "rules", "commits.md"), "second\n")
	if err := manager.Install([]string{"commits"}, ScopeUser, []string{"claude"}, ConflictAsk); err != nil {
		t.Fatal(err)
	}
	want = original + "\n<!-- pa2-skills:begin commits -->\nsecond\n<!-- pa2-skills:end commits -->\n\nadded after\n"
	if got := readFile(t, target); got != want {
		t.Fatalf("updated = %q, want %q", got, want)
	}
	if statuses, err := manager.Status(nil); err != nil || len(statuses) != 1 || statuses[0].Status != StatusCurrent {
		t.Fatalf("statuses = %v, %v; want one current", statuses, err)
	}

	if err := manager.Remove([]string{"commits"}, ScopeUser, []string{"claude"}, false); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, target); got != original+"\nadded after\n" {
		t.Fatalf("after removal = %q", got)
	}
}

func TestProjectRuleSharedBySeveralHarnesses(t *testing.T) {
	manager, paths := testManager(t)
	writeFile(t, filepath.Join(paths.SourceRoot, "rules", "commits.md"), "rule")
	project := t.TempDir()
	if output, err := exec.Command("git", "init", "-q", project).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, output)
	}
	t.Chdir(project)
	agents := filepath.Join(project, "AGENTS.md")
	writeFile(t, agents, "# Project\n")
	if err := os.Symlink("AGENTS.md", filepath.Join(project, "CLAUDE.md")); err != nil {
		t.Fatal(err)
	}

	if err := manager.Install([]string{"commits"}, ScopeProject, Harnesses, ConflictAsk); err != nil {
		t.Fatal(err)
	}
	want := "# Project\n\n<!-- pa2-skills:begin commits -->\nrule\n<!-- pa2-skills:end commits -->\n"
	if got := readFile(t, agents); got != want {
		t.Fatalf("AGENTS.md = %q, want one block: %q", got, want)
	}
	if info, err := os.Lstat(filepath.Join(project, "CLAUDE.md")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("CLAUDE.md is no longer a symlink: %v", err)
	}

	if err := manager.Remove([]string{"commits"}, ScopeProject, []string{"claude", "codex"}, false); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, agents); got != want {
		t.Fatalf("AGENTS.md = %q, want the block kept for opencode", got)
	}
	if err := manager.Remove([]string{"commits"}, ScopeProject, []string{"opencode"}, false); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, agents); got != "# Project\n" {
		t.Fatalf("AGENTS.md = %q, want the block removed", got)
	}
}
