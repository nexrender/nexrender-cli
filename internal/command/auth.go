package command

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nexrender/nexrender-cli/internal/api"
	"github.com/nexrender/nexrender-cli/internal/client"
	"github.com/spf13/cobra"
)

type AuthStatus struct {
	Authenticated    bool   `json:"authenticated"`
	Profile          string `json:"profile"`
	Server           string `json:"server"`
	CredentialSource string `json:"credentialSource,omitempty"`
	Message          string `json:"message,omitempty"`
}

func newAuthCommand(state *State) *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Manage Nexrender API authentication"}
	cmd.AddCommand(newAuthStatusCommand(state))
	cmd.AddCommand(newAuthLoginCommand(state))
	cmd.AddCommand(newAuthLogoutCommand(state))
	return cmd
}

func newAuthStatusCommand(state *State) *cobra.Command {
	return &cobra.Command{
		Use: "status", Short: "Check the active API credential", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolved, err := state.Resolve(false)
			if err != nil {
				return err
			}
			status := AuthStatus{Profile: resolved.Profile, Server: resolved.Server, CredentialSource: resolved.CredentialSource}
			if resolved.Token == "" {
				status.Message = "No credential configured"
				return printAuthStatus(state, status)
			}
			if err := verifyToken(cmd.Context(), resolved.Server, resolved.Token, state.Timeout); err != nil {
				status.Message = err.Error()
				return printAuthStatus(state, status)
			}
			status.Authenticated = true
			status.Message = "Credential accepted"
			return printAuthStatus(state, status)
		},
	}
}

func printAuthStatus(state *State, status AuthStatus) error {
	summary := "Not authenticated"
	if status.Authenticated {
		summary = "Authenticated"
	}
	return state.Printer(state.Out).Success(status, summary, func(w io.Writer) error {
		fmt.Fprintf(w, "Profile: %s\nServer: %s\nAuthenticated: %t\n", status.Profile, status.Server, status.Authenticated)
		if status.CredentialSource != "" {
			fmt.Fprintf(w, "Credential source: %s\n", status.CredentialSource)
		}
		if status.Message != "" {
			fmt.Fprintf(w, "Status: %s\n", status.Message)
		}
		return nil
	})
}

func newAuthLoginCommand(state *State) *cobra.Command {
	var tokenStdin bool
	cmd := &cobra.Command{
		Use: "login", Short: "Store and verify an API token", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			resolved, err := state.Resolve(false)
			if err != nil {
				return err
			}
			token, err := state.ReadSecret(tokenStdin)
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
			result := AuthStatus{Authenticated: true, Profile: resolved.Profile, Server: resolved.Server, CredentialSource: source, Message: "Credential accepted"}
			return printAuthStatus(state, result)
		},
	}
	cmd.Flags().BoolVar(&tokenStdin, "token-stdin", false, "read the token from stdin")
	return cmd
}

func newAuthLogoutCommand(state *State) *cobra.Command {
	return &cobra.Command{
		Use: "logout", Short: "Remove the stored credential", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			resolved, err := state.Resolve(false)
			if err != nil {
				return err
			}
			if err := state.Credential.Delete(resolved.Profile); err != nil {
				return err
			}
			return state.Printer(state.Out).Success(map[string]string{"profile": resolved.Profile}, "Removed stored credential for "+resolved.Profile, nil)
		},
	}
}

func verifyToken(ctx context.Context, server, token string, timeout time.Duration) error {
	apiClient, err := client.New(server, token, timeout)
	if err != nil {
		return err
	}
	limit := 1
	minimal := api.ListJobsParamsMinimal("true")
	response, err := apiClient.V2.ListJobsWithResponse(ctx, &api.ListJobsParams{Limit: &limit, Minimal: &minimal})
	if err != nil {
		return client.NetworkError(err)
	}
	if response.StatusCode() != http.StatusOK {
		return client.HTTPError(response.StatusCode(), response.Body, response.HTTPResponse.Header)
	}
	return nil
}
