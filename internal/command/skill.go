package command

import (
	"fmt"
	"io"
	"os"

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
			return state.Printer(state.Out).Success(results, fmt.Sprintf("Installed the Nexrender skill for %d agents", len(results)), func(w io.Writer) error {
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
	return cmd
}

func selectedAgents(home string, names []string) ([]skills.Agent, error) {
	if len(names) == 0 {
		return skills.Detect(home), nil
	}
	return skills.ResolveAgents(home, names)
}
