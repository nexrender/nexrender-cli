package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const managedMarker = ".nexrender-cli-managed"

type Agent struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

func KnownAgents(home string) []Agent {
	return []Agent{
		{Name: "codex", Path: filepath.Join(home, ".codex", "skills", "nexrender")},
		{Name: "claude", Path: filepath.Join(home, ".claude", "skills", "nexrender")},
		{Name: "agents", Path: filepath.Join(home, ".agents", "skills", "nexrender")},
	}
}

func Detect(home string) []Agent {
	known := KnownAgents(home)
	var detected []Agent
	for _, agent := range known {
		binary := agent.Name
		if agent.Name == "agents" {
			continue
		}
		_, pathErr := os.Stat(filepath.Dir(filepath.Dir(agent.Path)))
		_, commandErr := exec.LookPath(binary)
		if pathErr == nil || commandErr == nil {
			detected = append(detected, agent)
		}
	}
	return detected
}

func ResolveAgents(home string, names []string) ([]Agent, error) {
	known := KnownAgents(home)
	byName := make(map[string]Agent, len(known))
	for _, agent := range known {
		byName[agent.Name] = agent
	}
	seen := map[string]bool{}
	var result []Agent
	for _, name := range names {
		name = strings.ToLower(strings.TrimSpace(name))
		if name == "all" {
			return known, nil
		}
		agent, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("unknown agent %q; choose codex, claude, agents, or all", name)
		}
		if !seen[name] {
			result = append(result, agent)
			seen[name] = true
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func Install(agent Agent, version string, replace bool) error {
	if info, err := os.Stat(agent.Path); err == nil && info.IsDir() {
		marker := filepath.Join(agent.Path, managedMarker)
		if _, markerErr := os.Stat(marker); markerErr != nil && !replace {
			return fmt.Errorf("refusing to overwrite unmanaged skill at %s", agent.Path)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("inspect skill destination: %w", err)
	}
	if err := os.MkdirAll(agent.Path, 0o755); err != nil {
		return fmt.Errorf("create skill destination: %w", err)
	}
	err := fs.WalkDir(Files, "nexrender", func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel("nexrender", path)
		if err != nil || rel == "." {
			return err
		}
		destination := filepath.Join(agent.Path, filepath.FromSlash(rel))
		if entry.IsDir() {
			return os.MkdirAll(destination, 0o755)
		}
		data, err := Files.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(destination, data, 0o644)
	})
	if err != nil {
		return fmt.Errorf("install skill: %w", err)
	}
	marker := fmt.Sprintf("managed-by=nexrender-cli\nversion=%s\n", version)
	if err := os.WriteFile(filepath.Join(agent.Path, managedMarker), []byte(marker), 0o644); err != nil {
		return fmt.Errorf("write skill marker: %w", err)
	}
	return nil
}

func SkillText() ([]byte, error) {
	return Files.ReadFile("nexrender/SKILL.md")
}
