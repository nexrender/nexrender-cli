package command

import (
	"io"
	"net/http"

	"github.com/nexrender/nexrender-cli/internal/client"
	"github.com/spf13/cobra"
)

func newTeamCommand(state *State) *cobra.Command {
	cmd := &cobra.Command{Use: "team", Short: "Inspect the team that owns the API token"}
	cmd.AddCommand(newTeamShowCommand(state))
	return cmd
}

func newTeamShowCommand(state *State) *cobra.Command {
	return &cobra.Command{
		Use: "show", Short: "Show the team's id, name and trial allowance", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			response, err := apiClient.V2.GetTeamWithResponse(cmd.Context())
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			team := response.JSON200
			return state.Printer(state.Out).Success(team, "Team "+team.Name, func(w io.Writer) error { return printJSON(w, team) })
		},
	}
}
