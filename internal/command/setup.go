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

type SetupResult struct {
	Profile          string               `json:"profile"`
	Server           string               `json:"server"`
	Authenticated    bool                 `json:"authenticated"`
	CredentialSource string               `json:"credentialSource,omitempty"`
	Skills           []SkillInstallResult `json:"skills"`
}

const apiTokenSettingsURL = "https://app.nexrender.com/settings/api-tokens"

func newSetupCommand(state *State) *cobra.Command {
	var skipAuth, skipAgents, tokenStdin, replace bool
	var agents []string
	cmd := &cobra.Command{
		Use: "setup", Short: "Configure authentication and coding-agent integration", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolved, err := state.Resolve(false)
			if err != nil {
				return err
			}
			result := SetupResult{Profile: resolved.Profile, Server: resolved.Server, Skills: []SkillInstallResult{}}
			if !skipAuth {
				token := resolved.Token
				if token == "" {
					if !tokenStdin {
						fmt.Fprintf(state.Err, "Create an API token at %s\n\n", apiTokenSettingsURL)
					}
					token, err = state.ReadSecret(tokenStdin)
					if err != nil {
						return err
					}
					if err := verifyToken(cmd.Context(), resolved.Server, token, state.Timeout); err != nil {
						return err
					}
					source, err := state.Credential.Set(resolved.Profile, token)
					if err != nil {
						return err
					}
					resolved.CredentialSource = source
				} else if err := verifyToken(cmd.Context(), resolved.Server, token, state.Timeout); err != nil {
					return err
				}
				result.Authenticated = true
				result.CredentialSource = resolved.CredentialSource
			}
			if !skipAgents {
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				selected, err := selectedAgents(home, agents)
				if err != nil {
					return clierr.Usage(err.Error())
				}
				for _, agent := range selected {
					if err := skills.Install(agent, buildinfo.Version, replace); err != nil {
						return &clierr.Error{Code: "skill_install_failed", Message: err.Error(), Hint: "Use --replace only when you intend to replace an unmanaged skill directory.", ExitCode: clierr.ExitConflict}
					}
					result.Skills = append(result.Skills, SkillInstallResult{Agent: agent.Name, Path: agent.Path})
				}
			}
			return state.Printer(state.Out).Success(result, "Nexrender setup complete", func(w io.Writer) error {
				fmt.Fprintf(w, "Profile: %s\nServer: %s\nAuthenticated: %t\n", result.Profile, result.Server, result.Authenticated)
				for _, installed := range result.Skills {
					fmt.Fprintf(w, "Skill: %s at %s\n", installed.Agent, installed.Path)
				}
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&skipAuth, "skip-auth", false, "leave authentication unchanged")
	cmd.Flags().BoolVar(&skipAgents, "skip-agents", false, "leave coding-agent skills unchanged")
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "read a missing token from stdin")
	cmd.Flags().StringSliceVar(&agents, "agent", nil, "agent to configure: codex, claude, agents, or all")
	cmd.Flags().BoolVar(&replace, "replace-skill", false, "replace an unmanaged skill directory")
	return cmd
}
