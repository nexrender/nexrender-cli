package command

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	"github.com/nexrender/nexrender-cli/internal/buildinfo"
	"github.com/nexrender/nexrender-cli/internal/clierr"
	"github.com/nexrender/nexrender-cli/skills"
	"github.com/spf13/cobra"
)

type SkillInstallResult struct {
	Agent string `json:"agent"`
	Path  string `json:"path"`
}

func newSkillCommand(state *State) *cobra.Command {
	cmd := &cobra.Command{Use: "skill", Short: "Print or install the embedded Nexrender agent skill"}
	cmd.AddCommand(&cobra.Command{
		Use: "print", Short: "Print the embedded SKILL.md", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			data, err := skills.SkillText()
			if err != nil {
				return err
			}
			_, err = state.Out.Write(data)
			return err
		},
	})
	var agents []string
	var replace bool
	install := &cobra.Command{
		Use: "install", Short: "Install the managed skill for coding agents", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			selected, err := selectedAgents(home, agents)
			if err != nil {
				return clierr.Usage(err.Error())
			}
			if len(selected) == 0 {
				return clierr.New("no_agents_detected", "no supported coding agents were detected", clierr.ExitNotFound)
			}
			results := make([]SkillInstallResult, 0, len(selected))
			for _, agent := range selected {
				if err := skills.Install(agent, buildinfo.Version, replace); err != nil {
					return &clierr.Error{Code: "skill_install_failed", Message: err.Error(), Hint: "Use --replace only when you intend to replace an unmanaged skill directory.", ExitCode: clierr.ExitConflict}
				}
				results = append(results, SkillInstallResult{Agent: agent.Name, Path: agent.Path})
			}
			return state.Printer(state.Out).Success(results, "Installed the Nexrender skill for "+agentCount(len(results)), func(w io.Writer) error {
				for _, result := range results {
					fmt.Fprintf(w, "Installed for %s at %s\n", result.Agent, result.Path)
				}
				return nil
			})
		},
	}
	install.Flags().StringSliceVar(&agents, "agent", nil, "agent to configure: codex, claude, agents, or all")
	install.Flags().BoolVar(&replace, "replace", false, "replace an unmanaged skill directory")
	cmd.AddCommand(install)
	cmd.AddCommand(&cobra.Command{
		Use: "update", Short: "Refresh skills that the CLI installed earlier", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			results := []SkillInstallResult{}
			for _, agent := range skills.Managed(home) {
				if err := skills.Install(agent, buildinfo.Version, false); err != nil {
					return &clierr.Error{Code: "skill_install_failed", Message: err.Error(), ExitCode: clierr.ExitConflict}
				}
				results = append(results, SkillInstallResult{Agent: agent.Name, Path: agent.Path})
			}
			summary := "Updated the Nexrender skill for " + agentCount(len(results))
			if len(results) == 0 {
				summary = "No CLI-installed skills to update"
			}
			return state.Printer(state.Out).Success(results, summary, func(w io.Writer) error {
				for _, result := range results {
					fmt.Fprintf(w, "Updated for %s at %s\n", result.Agent, result.Path)
				}
				return nil
			})
		},
	})
	return cmd
}

// selectedAgents resolves --agent names. Without names it returns the detected agents
// plus every agent whose skill the CLI installed earlier, so those copies stay current.
func selectedAgents(home string, names []string) ([]skills.Agent, error) {
	if len(names) > 0 {
		return skills.ResolveAgents(home, names)
	}
	selected := skills.Detect(home)
	for _, managed := range skills.Managed(home) {
		if !slices.ContainsFunc(selected, func(agent skills.Agent) bool { return agent.Name == managed.Name }) {
			selected = append(selected, managed)
		}
	}
	slices.SortFunc(selected, func(a, b skills.Agent) int { return strings.Compare(a.Name, b.Name) })
	return selected, nil
}

func agentCount(n int) string {
	if n == 1 {
		return "1 agent"
	}
	return fmt.Sprintf("%d agents", n)
}
