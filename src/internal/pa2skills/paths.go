package pa2skills

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Paths struct {
	Home       string
	DataHome   string
	StateHome  string
	ConfigHome string
	SourceRoot string
}

func ResolvePaths() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolve home directory: %w", err)
	}
	dataHome := firstEnvironment("PA2_SKILLS_DATA_HOME", "XDG_DATA_HOME")
	if dataHome == "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	stateHome := firstEnvironment("PA2_SKILLS_STATE_HOME", "XDG_STATE_HOME")
	if stateHome == "" {
		stateHome = filepath.Join(home, ".local", "state")
	}
	configHome := firstEnvironment("XDG_CONFIG_HOME")
	if configHome == "" {
		configHome = filepath.Join(home, ".config")
	}
	sourceRoot := os.Getenv("PA2_SKILLS_SOURCE_DIR")
	if sourceRoot == "" {
		sourceRoot = filepath.Join(dataHome, "pa2-skills", "agents")
	}
	return Paths{
		Home:       home,
		DataHome:   dataHome,
		StateHome:  stateHome,
		ConfigHome: configHome,
		SourceRoot: sourceRoot,
	}, nil
}

// Display shortens a path below the home directory to start with ~.
func (p Paths) Display(path string) string {
	if p.Home == "" {
		return path
	}
	if path == p.Home {
		return "~"
	}
	if rest, found := strings.CutPrefix(path, p.Home+string(filepath.Separator)); found {
		return "~/" + rest
	}
	return path
}

// harnessHome is the directory a harness keeps its user configuration in; it
// exists once the harness has been used on this machine.
func (p Paths) harnessHome(harness string) string {
	switch harness {
	case "claude":
		return filepath.Join(p.Home, ".claude")
	case "codex":
		return filepath.Join(p.Home, ".codex")
	case "opencode":
		return filepath.Join(p.ConfigHome, "opencode")
	}
	return ""
}

func (p Paths) StateRoot() string {
	return filepath.Join(p.StateHome, "pa2-skills")
}

func (p Paths) InstallStatePath(key string) string {
	return filepath.Join(p.StateRoot(), "installations", key+".json")
}

func (p Paths) InstallationsRoot() string {
	return filepath.Join(p.StateRoot(), "installations")
}

func (p Paths) BaselinesRoot() string {
	return filepath.Join(p.StateRoot(), "baselines")
}

func (p Paths) BaselinePath(hash string) string {
	return filepath.Join(p.BaselinesRoot(), hash)
}

// projectStateKey leaves out the kind: names are unique across kinds, and keys
// of installations recorded before rules existed must stay the same.
func projectStateKey(projectRoot, scope, harness, name string) string {
	sum := sha256.Sum256([]byte(projectRoot + "\x00" + scope + "\x00" + harness + "\x00" + name))
	return hex.EncodeToString(sum[:])
}

func firstEnvironment(names ...string) string {
	for _, name := range names {
		if value := os.Getenv(name); value != "" {
			return value
		}
	}
	return ""
}
