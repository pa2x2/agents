package pa2skills

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Discovery struct {
	Status string
	Path   string
	Name   string
	Detail string
}

var skippedDiscoveryDirectories = map[string]bool{
	".git": true, ".hg": true, ".svn": true,
	"node_modules": true, "vendor": true, "build": true, "dist": true,
}

var traversedHiddenDirectories = map[string]bool{
	".agents": true, ".claude": true, ".opencode": true,
}

func (m Manager) Discover(root string) ([]Discovery, error) {
	resolvedRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve discovery path: %w", err)
	}
	info, err := os.Stat(resolvedRoot)
	if err != nil {
		return nil, fmt.Errorf("inspect discovery path: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("discovery path is not a directory: %s", resolvedRoot)
	}

	var findings []Discovery
	err = filepath.WalkDir(resolvedRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() {
			return nil
		}
		if path != resolvedRoot && shouldSkipDiscoveryDirectory(entry.Name()) {
			return filepath.SkipDir
		}
		skillFile := filepath.Join(path, "SKILL.md")
		skillInfo, err := os.Stat(skillFile)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		name := entry.Name()
		finding := Discovery{Status: "FOUND", Path: path, Name: name}
		if !skillInfo.Mode().IsRegular() {
			finding.Status = "INVALID"
			finding.Detail = "SKILL.md is not a regular file"
		} else if err := ValidateSkillName(name); err != nil {
			finding.Status = "INVALID"
			finding.Detail = err.Error()
		} else if declared, err := declaredSkillName(skillFile); err != nil {
			finding.Status = "INVALID"
			finding.Detail = err.Error()
		} else if declared != "" && declared != name {
			finding.Status = "INVALID"
			finding.Detail = fmt.Sprintf("frontmatter name %q differs from directory name", declared)
		} else if status, detail, err := m.trackedStatus(path, name); err != nil {
			return err
		} else {
			finding.Status = status
			finding.Detail = detail
		}
		findings = append(findings, finding)
		return filepath.SkipDir
	})
	if err != nil {
		return nil, fmt.Errorf("discover skills: %w", err)
	}

	counts := map[string]int{}
	for _, finding := range findings {
		if finding.Status != "INVALID" {
			counts[finding.Name]++
		}
	}
	for index := range findings {
		if findings[index].Status != "INVALID" && counts[findings[index].Name] > 1 {
			findings[index].Status = "CONFLICT"
			findings[index].Detail = "multiple discovered skills use this name"
		}
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Path < findings[j].Path })
	return findings, nil
}

func (m Manager) AddSkill(source, requestedName string) (string, error) {
	resolvedSource, err := filepath.Abs(source)
	if err != nil {
		return "", fmt.Errorf("resolve skill path: %w", err)
	}
	info, err := os.Stat(resolvedSource)
	if err != nil {
		return "", fmt.Errorf("inspect skill path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("skill path is not a directory: %s", resolvedSource)
	}
	skillFile := filepath.Join(resolvedSource, "SKILL.md")
	skillInfo, err := os.Stat(skillFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%s does not contain SKILL.md", resolvedSource)
		}
		return "", err
	}
	if !skillInfo.Mode().IsRegular() {
		return "", errors.New("SKILL.md must be a regular file")
	}

	name := requestedName
	if name == "" {
		name = filepath.Base(resolvedSource)
	}
	if err := ValidateSkillName(name); err != nil {
		return "", err
	}
	declared, err := declaredSkillName(skillFile)
	if err != nil {
		return "", err
	}
	if declared != "" && declared != name {
		return "", fmt.Errorf("frontmatter name %q differs from skill name %q", declared, name)
	}
	if err := validateSkillSymlinks(resolvedSource); err != nil {
		return "", err
	}

	target := filepath.Join(m.Paths.SourceRoot, "skills", name)
	if _, err := os.Lstat(target); err == nil {
		return "", fmt.Errorf("skill %q is already tracked at %s", name, target)
	} else if !os.IsNotExist(err) {
		return "", err
	}
	if err := copyTree(resolvedSource, target); err != nil {
		return "", fmt.Errorf("add skill: %w", err)
	}
	return target, nil
}

func (m Manager) trackedStatus(path, name string) (string, string, error) {
	target := filepath.Join(m.Paths.SourceRoot, "skills", name)
	trackedHash, err := treeHash(target)
	if err != nil {
		return "", "", err
	}
	if trackedHash == "" {
		return "FOUND", "", nil
	}
	discoveredHash, err := treeHash(path)
	if err != nil {
		return "", "", err
	}
	if discoveredHash == trackedHash {
		return "TRACKED", "identical to managed skill", nil
	}
	return "CONFLICT", "tracked skill has different contents", nil
}

func shouldSkipDiscoveryDirectory(name string) bool {
	if skippedDiscoveryDirectories[name] {
		return true
	}
	return strings.HasPrefix(name, ".") && !traversedHiddenDirectories[name]
}

func ValidateSkillName(name string) error {
	if name == "" || name == "." || name == ".." || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return fmt.Errorf("invalid skill name %q", name)
	}
	return nil
}

func declaredSkillName(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return "", scanner.Err()
	}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "---" {
			return "", nil
		}
		key, value, found := strings.Cut(line, ":")
		if found && strings.TrimSpace(key) == "name" {
			return strings.Trim(strings.TrimSpace(value), `"'`), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", errors.New("SKILL.md frontmatter is not terminated")
}

func validateSkillSymlinks(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.Type()&os.ModeSymlink == 0 {
			return nil
		}
		target, err := filepath.EvalSymlinks(path)
		if err != nil {
			return fmt.Errorf("resolve symlink %s: %w", path, err)
		}
		relative, err := filepath.Rel(root, target)
		if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return fmt.Errorf("skill contains symlink outside its directory: %s", path)
		}
		return nil
	})
}
