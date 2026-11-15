package pa2skills

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ruleHandler deploys a rule as a block between marker comments inside the
// harness instruction file (CLAUDE.md or AGENTS.md). Text outside the markers
// belongs to the user and is never changed. Hashes cover only the block body,
// so edits elsewhere in the file do not mark the rule as modified.
//
// Codex and opencode read the same project AGENTS.md, so their project-scope
// installations of a rule share one block.
type ruleHandler struct{}

func (ruleHandler) kind() Kind { return KindRule }

func (ruleHandler) names(sourceRoot string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(sourceRoot, "rules"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read rules source: %w", err)
	}
	var names []string
	for _, entry := range entries {
		name, isMarkdown := strings.CutSuffix(entry.Name(), ".md")
		if isMarkdown && entry.Type().IsRegular() {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

func (ruleHandler) sourcePath(sourceRoot, name string) string {
	return filepath.Join(sourceRoot, "rules", name+".md")
}

func (ruleHandler) targetPath(paths Paths, _ string, scope Scope, harness, projectRoot string) (string, error) {
	if scope == ScopeUser {
		switch harness {
		case "codex":
			return filepath.Join(paths.Home, ".codex", "AGENTS.md"), nil
		case "claude":
			return filepath.Join(paths.Home, ".claude", "CLAUDE.md"), nil
		case "opencode":
			return filepath.Join(paths.ConfigHome, "opencode", "AGENTS.md"), nil
		}
	}
	if scope == ScopeProject {
		switch harness {
		case "codex", "opencode":
			return filepath.Join(projectRoot, "AGENTS.md"), nil
		case "claude":
			return filepath.Join(projectRoot, "CLAUDE.md"), nil
		}
	}
	return "", fmt.Errorf("unsupported harness %q", harness)
}

func (ruleHandler) sourceHash(source string) (string, error) {
	body, err := ruleBody(source)
	if err != nil {
		return "", err
	}
	return bodyHash(body), nil
}

func (ruleHandler) targetHash(target, name string) (string, error) {
	body, found, err := installedRuleBody(target, name)
	if err != nil || !found {
		return "", err
	}
	return bodyHash(body), nil
}

func (ruleHandler) write(source, target, name string) error {
	body, err := ruleBody(source)
	if err != nil {
		return err
	}
	if target, err = resolveSymlink(target); err != nil {
		return err
	}
	lines, mode, err := readLines(target)
	if err != nil {
		return err
	}
	begin, end, err := findRuleBlock(lines, target, name)
	if err != nil {
		return err
	}
	beginMarker, endMarker := ruleMarkers(name)
	block := append(append([]string{beginMarker}, strings.Split(body, "\n")...), endMarker)
	if begin >= 0 {
		lines = append(lines[:begin], append(block, lines[end+1:]...)...)
	} else {
		lines = trimTrailingBlankLines(lines)
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, block...)
	}
	return writeLines(target, lines, mode)
}

func (ruleHandler) remove(target, name string) error {
	target, err := resolveSymlink(target)
	if err != nil {
		return err
	}
	lines, mode, err := readLines(target)
	if err != nil {
		return err
	}
	begin, end, err := findRuleBlock(lines, target, name)
	if err != nil || begin < 0 {
		return err
	}
	after := lines[end+1:]
	// Drop the blank line that separated the block from the text before it.
	if begin > 0 && strings.TrimSpace(lines[begin-1]) == "" && (len(after) == 0 || strings.TrimSpace(after[0]) == "") {
		begin--
	}
	lines = trimTrailingBlankLines(append(lines[:begin], after...))
	if len(lines) == 0 {
		return os.Remove(target)
	}
	return writeLines(target, lines, mode)
}

func (ruleHandler) saveBaseline(source, destination string) error {
	body, err := ruleBody(source)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return err
	}
	return writeFileAtomic(destination, []byte(body+"\n"), 0o600)
}

func (ruleHandler) diff(output io.Writer, target, source, name string) error {
	body, _, err := installedRuleBody(target, name)
	if err != nil {
		return err
	}
	directory, err := os.MkdirTemp("", "pa2-skills-rule-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	installed := filepath.Join(directory, name+".md")
	if err := os.WriteFile(installed, []byte(body+"\n"), 0o600); err != nil {
		return err
	}
	return showDiff(output, installed, source)
}

func ruleMarkers(name string) (string, string) {
	return "<!-- pa2-skills:begin " + name + " -->", "<!-- pa2-skills:end " + name + " -->"
}

// ruleBody returns the source rule without trailing whitespace, which is the
// exact text placed between the markers.
func ruleBody(source string) (string, error) {
	contents, err := os.ReadFile(source)
	if err != nil {
		return "", err
	}
	return normalizeRuleBody(string(contents)), nil
}

func installedRuleBody(target, name string) (string, bool, error) {
	lines, _, err := readLines(target)
	if err != nil {
		return "", false, err
	}
	begin, end, err := findRuleBlock(lines, target, name)
	if err != nil || begin < 0 {
		return "", false, err
	}
	return normalizeRuleBody(strings.Join(lines[begin+1:end], "\n")), true, nil
}

func normalizeRuleBody(text string) string {
	return strings.TrimRight(strings.ReplaceAll(text, "\r\n", "\n"), " \t\r\n")
}

func bodyHash(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

// findRuleBlock returns the line indexes of the rule's markers, or -1, -1 when
// the file has no block for it.
func findRuleBlock(lines []string, target, name string) (int, int, error) {
	beginMarker, endMarker := ruleMarkers(name)
	begin, end := -1, -1
	for index, line := range lines {
		switch strings.TrimSpace(line) {
		case beginMarker:
			if begin >= 0 {
				return 0, 0, fmt.Errorf("%s has more than one %s block; remove the duplicate markers", target, name)
			}
			begin = index
		case endMarker:
			if begin < 0 || end >= 0 {
				return 0, 0, fmt.Errorf("%s has an unmatched end marker for %s; fix the markers by hand", target, name)
			}
			end = index
		}
	}
	if begin >= 0 && end < 0 {
		return 0, 0, fmt.Errorf("%s has no end marker for %s; fix the markers by hand", target, name)
	}
	return begin, end, nil
}

// readLines returns the file's lines without the final newline, and its mode.
// A missing file reads as empty.
func readLines(path string) ([]string, os.FileMode, error) {
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, 0o644, nil
	}
	if err != nil {
		return nil, 0, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, 0, err
	}
	text := strings.TrimSuffix(strings.ReplaceAll(string(contents), "\r\n", "\n"), "\n")
	if text == "" {
		return nil, info.Mode().Perm(), nil
	}
	return strings.Split(text, "\n"), info.Mode().Perm(), nil
}

func writeLines(path string, lines []string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return writeFileAtomic(path, []byte(strings.Join(lines, "\n")+"\n"), mode)
}

// resolveSymlink follows a symlinked instruction file (such as CLAUDE.md
// pointing at AGENTS.md) so that atomic writes replace the file it points at
// instead of the link.
func resolveSymlink(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if os.IsNotExist(err) {
		return path, nil
	}
	return resolved, err
}

func trimTrailingBlankLines(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
