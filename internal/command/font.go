package command

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/nexrender/nexrender-cli/internal/client"
	"github.com/nexrender/nexrender-cli/internal/clierr"
	"github.com/spf13/cobra"
)

func newFontCommand(state *State) *cobra.Command {
	cmd := &cobra.Command{Use: "font", Short: "Manage team fonts"}
	cmd.AddCommand(newFontListCommand(state))
	cmd.AddCommand(newFontShowCommand(state))
	cmd.AddCommand(newFontUploadCommand(state))
	cmd.AddCommand(newFontDeleteCommand(state))
	return cmd
}

func newFontListCommand(state *State) *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List uploaded fonts", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			response, err := apiClient.V2.ListFontsWithResponse(cmd.Context())
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			fonts := *response.JSON200
			return state.Printer(state.Out).Success(fonts, fmt.Sprintf("%d fonts", len(fonts)), func(w io.Writer) error {
				table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
				fmt.Fprintln(table, "ID\tFAMILY\tFILE")
				for _, font := range fonts {
					fmt.Fprintf(table, "%s\t%s\t%s\n", stringPointer(font.Id), stringPointer(font.FamilyName), stringPointer(font.FileName))
				}
				return table.Flush()
			})
		},
	}
}

func newFontShowCommand(state *State) *cobra.Command {
	return &cobra.Command{
		Use: "show <id>", Short: "Show font metadata", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			response, err := apiClient.V2.GetFontWithResponse(cmd.Context(), args[0])
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			return state.Printer(state.Out).Success(*response.JSON200, "Font "+args[0], func(w io.Writer) error { return printJSON(w, response.JSON200) })
		},
	}
}

func newFontUploadCommand(state *State) *cobra.Command {
	var familyName string
	cmd := &cobra.Command{
		Use: "upload <file.ttf>", Short: "Upload a TrueType font", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.ToLower(filepath.Ext(args[0])) != ".ttf" {
				return clierr.Validation("font must be a .ttf file", "Convert or select a TrueType font before uploading.")
			}
			fileInfo, err := os.Stat(args[0])
			if err != nil {
				return clierr.Wrap("input_error", "could not inspect font file", clierr.ExitUsage, err)
			}
			if fileInfo.Size() > 40*1024*1024 {
				return clierr.Validation("font exceeds the 40 MiB upload limit", "Choose a smaller TrueType font file.")
			}
			file, err := os.Open(args[0])
			if err != nil {
				return clierr.Wrap("input_error", "could not open font file", clierr.ExitUsage, err)
			}
			defer file.Close()
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			part, err := writer.CreateFormFile("font", filepath.Base(args[0]))
			if err != nil {
				return err
			}
			if _, err := io.Copy(part, io.LimitReader(file, 40*1024*1024+1)); err != nil {
				return err
			}
			if familyName != "" {
				if err := writer.WriteField("familyName", familyName); err != nil {
					return err
				}
			}
			if err := writer.Close(); err != nil {
				return err
			}
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			response, err := apiClient.V2.UploadFontWithBodyWithResponse(cmd.Context(), writer.FormDataContentType(), &body)
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() != http.StatusCreated || response.JSON201 == nil {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			return state.Printer(state.Out).Success(*response.JSON201, "Uploaded font "+stringPointer(response.JSON201.FileName), nil)
		},
	}
	cmd.Flags().StringVar(&familyName, "family-name", "", "override the detected font family")
	return cmd
}

func newFontDeleteCommand(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use: "delete <id>", Short: "Delete a font", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := state.Confirm("Delete font "+args[0]+"?", yes); err != nil {
				return err
			}
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			response, err := apiClient.V2.DeleteFontWithResponse(cmd.Context(), args[0])
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() < 200 || response.StatusCode() >= 300 {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			return state.Printer(state.Out).Success(map[string]string{"id": args[0]}, "Deleted font "+args[0], nil)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm deletion")
	return cmd
}
