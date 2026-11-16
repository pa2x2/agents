package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"pa2-skills/internal/pa2skills"
)

// doctor prints one line per check: ok, note (worth knowing), warn (something
// will misbehave) or fail (pa2-skills cannot work). Only failures make it exit non-zero.
func doctor(manager pa2skills.Manager, stdout io.Writer) error {
	failed := false
	report := func(level, format string, arguments ...any) {
		failed = failed || level == "fail"
		fmt.Fprintf(stdout, "%-5s %s\n", level, fmt.Sprintf(format, arguments...))
	}
	paths := manager.Paths

	gitFound := true
	if _, err := exec.LookPath("git"); err != nil {
		gitFound = false
		report("fail", "git is not on PATH; pa2-skills needs it to manage the source checkout")
	}
	sourceFound := true
	if _, err := os.Stat(filepath.Join(paths.SourceRoot, ".git")); err != nil {
		sourceFound = false
		report("fail", "no managed source checkout at %s; run the bootstrap script", paths.Display(paths.SourceRoot))
	} else {
		report("ok", "source checkout at %s", paths.Display(paths.SourceRoot))
	}
	if gitFound && sourceFound {
		if clean, err := manager.SourceIsClean(); err != nil {
			report("warn", "%v", err)
		} else if !clean {
			report("warn", "the source checkout has uncommitted changes; update refuses to run until you commit or stash them (pa2-skills cd)")
		}
		if result, err := manager.CheckSource(); err != nil {
			report("warn", "could not reach the remote source: %v", err)
		} else if strings.HasPrefix(result, "update available") {
			report("note", "remote source: %s; run 'pa2-skills update'", result)
		} else {
			report("ok", "remote source: %s", result)
		}
	}

	checkBinary(paths, report)

	if sourceFound {
		checkInstallations(manager, report)
	}
	report("ok", "private state at %s", paths.Display(paths.StateRoot()))
	checkCompletion(paths, report)

	if failed {
		return errors.New("doctor found problems")
	}
	return nil
}

func checkBinary(paths pa2skills.Paths, report func(level, format string, arguments ...any)) {
	running, err := os.Executable()
	if err == nil {
		running, err = filepath.EvalSymlinks(running)
	}
	if err != nil {
		report("warn", "cannot locate the running binary: %v", err)
		return
	}
	onPath, err := exec.LookPath("pa2-skills")
	if err != nil {
		report("warn", "pa2-skills is not on PATH; add %s to PATH", paths.Display(filepath.Dir(running)))
		return
	}
	if resolved, err := filepath.EvalSymlinks(onPath); err == nil && resolved != running {
		report("warn", "PATH runs %s, not this binary at %s", paths.Display(onPath), paths.Display(running))
		return
	}
	report("ok", "binary %s at %s", version, paths.Display(onPath))
}

func checkInstallations(manager pa2skills.Manager, report func(level, format string, arguments ...any)) {
	statuses, err := manager.Status(pa2skills.Filter{})
	if err != nil {
		report("warn", "cannot read managed installations: %v", err)
		return
	}
	counts := map[string]int{}
	for _, status := range statuses {
		counts[status.Status]++
	}
	var parts []string
	for _, status := range pa2skills.Statuses {
		if status != pa2skills.StatusCurrent && counts[status] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[status], status))
		}
	}
	switch {
	case len(parts) == 0:
		report("ok", "%d managed installation(s), all current", len(statuses))
	case counts[pa2skills.StatusModified] == len(statuses)-counts[pa2skills.StatusCurrent]:
		report("ok", "%d managed installation(s): %s", len(statuses), strings.Join(parts, ", "))
	default:
		report("warn", "%d managed installation(s): %s; run 'pa2-skills status'", len(statuses), strings.Join(parts, ", "))
	}
}

// checkCompletion looks for the completion setup in the login shell's startup
// file. It is only a hint: completion can be loaded in other ways.
func checkCompletion(paths pa2skills.Paths, report func(level, format string, arguments ...any)) {
	shell := filepath.Base(os.Getenv("SHELL"))
	var files []string
	var setup string
	switch shell {
	case "zsh":
		directory := os.Getenv("ZDOTDIR")
		if directory == "" {
			directory = paths.Home
		}
		files, setup = []string{filepath.Join(directory, ".zshrc")}, `eval "$(pa2-skills completion zsh)"`
	case "bash":
		files, setup = []string{filepath.Join(paths.Home, ".bashrc")}, `eval "$(pa2-skills completion bash)"`
	case "fish":
		fish := filepath.Join(paths.ConfigHome, "fish")
		files, setup = []string{filepath.Join(fish, "config.fish"), filepath.Join(fish, "completions", "pa2-skills.fish")}, "pa2-skills completion fish | source"
	default:
		return
	}
	for _, file := range files {
		contents, err := os.ReadFile(file)
		if err == nil && (strings.Contains(string(contents), "pa2-skills completion") || strings.Contains(string(contents), "__pa2_skills")) {
			report("ok", "%s completion is set up in %s", shell, paths.Display(file))
			return
		}
	}
	report("note", "%s completion is not set up; add to %s: %s", shell, paths.Display(files[0]), setup)
}
