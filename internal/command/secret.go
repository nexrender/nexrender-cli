package command

import (
	"fmt"
	"io"
	"net/http"
	"text/tabwriter"

	"github.com/nexrender/nexrender-cli/internal/api"
	"github.com/nexrender/nexrender-cli/internal/client"
	"github.com/spf13/cobra"
)

func newSecretCommand(state *State) *cobra.Command {
	cmd := &cobra.Command{Use: "secret", Short: "Manage encrypted team secrets"}
	cmd.AddCommand(newSecretListCommand(state))
	cmd.AddCommand(newSecretSetCommand(state))
	cmd.AddCommand(newSecretDeleteCommand(state))
	return cmd
}

func newSecretListCommand(state *State) *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List secret names and ids", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			response, err := apiClient.V2.ListSecretsWithResponse(cmd.Context())
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			secrets := *response.JSON200
			return state.Printer(state.Out).Success(secrets, fmt.Sprintf("%d secrets", len(secrets)), func(w io.Writer) error {
				table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
				fmt.Fprintln(table, "ID\tNAME\tCREATED")
				for _, secret := range secrets {
					fmt.Fprintf(table, "%s\t%s\t%s\n", secret.Id, secret.Name, secret.CreatedAt.Format("2006-01-02"))
				}
				return table.Flush()
			})
		},
	}
}

func newSecretSetCommand(state *State) *cobra.Command {
	var valueStdin bool
	cmd := &cobra.Command{
		Use: "set <name>", Short: "Create a secret without exposing its value", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			value, err := state.ReadHidden("Secret value", valueStdin)
			if err != nil {
				return err
			}
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			response, err := apiClient.V2.CreateOrUpdateSecretWithResponse(cmd.Context(), api.SecretCreation{Name: args[0], Value: value})
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() != http.StatusCreated {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			return state.Printer(state.Out).Success(map[string]string{"name": args[0]}, "Created secret "+args[0], nil)
		},
	}
	cmd.Flags().BoolVar(&valueStdin, "value-stdin", false, "read the secret value from stdin")
	return cmd
}

func newSecretDeleteCommand(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use: "delete <id>", Short: "Delete a secret", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := state.Confirm("Delete secret "+args[0]+"?", yes); err != nil {
				return err
			}
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			response, err := apiClient.V2.DeleteSecretWithResponse(cmd.Context(), args[0])
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() < 200 || response.StatusCode() >= 300 {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			return state.Printer(state.Out).Success(map[string]string{"id": args[0]}, "Deleted secret "+args[0], nil)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm deletion")
	return cmd
}
