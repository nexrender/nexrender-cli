package command

import (
	"fmt"
	"io"

	"github.com/nexrender/nexrender-cli/internal/buildinfo"
	"github.com/spf13/cobra"
)

type DoctorCheck struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type DoctorResult struct {
	Healthy bool           `json:"healthy"`
	Version buildinfo.Info `json:"version"`
	Checks  []DoctorCheck  `json:"checks"`
}

func newDoctorCommand(state *State) *cobra.Command {
	return &cobra.Command{
		Use: "doctor", Short: "Check local configuration and API access", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			result := DoctorResult{Healthy: true, Version: buildinfo.Current(), Checks: []DoctorCheck{}}
			resolved, err := state.Resolve(false)
			if err != nil {
				result.Healthy = false
				result.Checks = append(result.Checks, DoctorCheck{Name: "configuration", Message: err.Error()})
			} else {
				result.Checks = append(result.Checks, DoctorCheck{Name: "configuration", OK: true, Message: resolved.Profile + " at " + resolved.Server})
				if resolved.Token == "" {
					result.Healthy = false
					result.Checks = append(result.Checks, DoctorCheck{Name: "credential", Message: "No API credential configured"})
				} else if err := verifyToken(cmd.Context(), resolved.Server, resolved.Token, state.Timeout); err != nil {
					result.Healthy = false
					result.Checks = append(result.Checks, DoctorCheck{Name: "api", Message: err.Error()})
				} else {
					result.Checks = append(result.Checks, DoctorCheck{Name: "credential", OK: true, Message: "Loaded from " + resolved.CredentialSource})
					result.Checks = append(result.Checks, DoctorCheck{Name: "api", OK: true, Message: "API accepted the credential"})
				}
			}
			summary := "Nexrender CLI is healthy"
			if !result.Healthy {
				summary = "Nexrender CLI needs attention"
			}
			return state.Printer(state.Out).Success(result, summary, func(w io.Writer) error {
				fmt.Fprintln(w, summary)
				for _, check := range result.Checks {
					status := "FAIL"
					if check.OK {
						status = "OK"
					}
					fmt.Fprintf(w, "[%s] %s: %s\n", status, check.Name, check.Message)
				}
				return nil
			})
		},
	}
}
