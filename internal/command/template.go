package command

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/nexrender/nexrender-cli/internal/api"
	"github.com/nexrender/nexrender-cli/internal/client"
	"github.com/nexrender/nexrender-cli/internal/clierr"
	"github.com/spf13/cobra"
)

type templateLayerResult struct {
	Aeid            int                    `json:"aeid"`
	CompositionName string                 `json:"composition_name"`
	Data            map[string]interface{} `json:"data"`
	Height          float32                `json:"height"`
	InPoint         float32                `json:"in_point"`
	LayerType       string                 `json:"layer_type"`
	Left            float32                `json:"left"`
	Name            string                 `json:"name"`
	OutPoint        float32                `json:"out_point"`
	ParentId        *int                   `json:"parent_id"`
	SourceCompId    *int                   `json:"source_comp_id"`
	SourceType      *string                `json:"source_type"`
	StartTime       float32                `json:"start_time"`
	Top             float32                `json:"top"`
	Width           float32                `json:"width"`
}

func newTemplateCommand(state *State) *cobra.Command {
	cmd := &cobra.Command{Use: "template", Short: "Upload and inspect After Effects templates"}
	cmd.AddCommand(newTemplateListCommand(state))
	cmd.AddCommand(newTemplateShowCommand(state))
	cmd.AddCommand(newTemplateCompositionsCommand(state))
	cmd.AddCommand(newTemplateLayersCommand(state))
	cmd.AddCommand(newTemplateUploadCommand(state))
	cmd.AddCommand(newTemplateWaitCommand(state))
	cmd.AddCommand(newTemplateRenameCommand(state))
	cmd.AddCommand(newTemplateDownloadCommand(state))
	cmd.AddCommand(newTemplateDeleteCommand(state))
	return cmd
}

func newTemplateListCommand(state *State) *cobra.Command {
	return &cobra.Command{
		Use: "list", Short: "List templates", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			response, err := apiClient.V3.ListTemplatesV3WithResponse(cmd.Context())
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			templates := *response.JSON200
			return state.Printer(state.Out).Success(templates, fmt.Sprintf("%d templates", len(templates)), func(w io.Writer) error {
				table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
				fmt.Fprintln(table, "ID\tSTATUS\tTYPE\tNAME")
				for _, template := range templates {
					fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", template.Id, template.Status, template.Type, template.DisplayName)
				}
				return table.Flush()
			})
		},
	}
}

func newTemplateShowCommand(state *State) *cobra.Command {
	return &cobra.Command{
		Use: "show <id>", Short: "Show template metadata", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			template, err := getTemplate(cmd.Context(), state, args[0])
			if err != nil {
				return err
			}
			return state.Printer(state.Out).Success(template, "Template "+args[0], func(w io.Writer) error { return printJSON(w, template) })
		},
	}
}

func newTemplateCompositionsCommand(state *State) *cobra.Command {
	var limit, offset int
	cmd := &cobra.Command{
		Use: "compositions <id>", Short: "List template compositions", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			response, err := apiClient.V3.ListTemplateCompositionsV3WithResponse(cmd.Context(), args[0], &api.ListTemplateCompositionsV3Params{Limit: &limit, Offset: &offset})
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			items := *response.JSON200
			return state.Printer(state.Out).Success(items, fmt.Sprintf("%d compositions", len(items)), func(w io.Writer) error {
				table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
				fmt.Fprintln(table, "AE ID\tNAME\tSIZE\tFPS\tDURATION")
				for _, item := range items {
					fmt.Fprintf(table, "%s\t%s\t%.0fx%.0f\t%.2f\t%.2fs\n", item.Aeid, item.Name, item.Width, item.Height, item.FrameRate, item.Duration)
				}
				return table.Flush()
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 300, "maximum compositions to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "pagination offset")
	return cmd
}

func newTemplateLayersCommand(state *State) *cobra.Command {
	var limit, offset int
	cmd := &cobra.Command{
		Use: "layers <id>", Short: "List exact case-sensitive template layer names", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			response, err := apiClient.V3.ListTemplateLayersV3WithResponse(cmd.Context(), args[0], &api.ListTemplateLayersV3Params{Limit: &limit, Offset: &offset})
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			compositions, err := listAllTemplateCompositions(cmd.Context(), apiClient, args[0])
			if err != nil {
				return err
			}
			compositionNames := make(map[string]string, len(compositions))
			for _, composition := range compositions {
				compositionNames[composition.Aeid] = composition.Name
			}
			layers := *response.JSON200
			items := make([]templateLayerResult, 0, len(layers))
			for _, layer := range layers {
				items = append(items, templateLayerResult{
					Aeid:            layer.Aeid,
					CompositionName: compositionNames[fmt.Sprint(layer.CompositionId)],
					Data:            layer.Data,
					Height:          layer.Height,
					InPoint:         layer.InPoint,
					LayerType:       layer.LayerType,
					Left:            layer.Left,
					Name:            layer.Name,
					OutPoint:        layer.OutPoint,
					ParentId:        layer.ParentId,
					SourceCompId:    layer.SourceCompId,
					SourceType:      layer.SourceType,
					StartTime:       layer.StartTime,
					Top:             layer.Top,
					Width:           layer.Width,
				})
			}
			return state.Printer(state.Out).Success(items, fmt.Sprintf("%d layers", len(items)), func(w io.Writer) error {
				table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
				fmt.Fprintln(table, "AE ID\tCOMPOSITION NAME\tTYPE\tSOURCE\tNAME")
				for _, item := range items {
					fmt.Fprintf(table, "%d\t%s\t%s\t%s\t%s\n", item.Aeid, item.CompositionName, item.LayerType, stringPointer(item.SourceType), item.Name)
				}
				return table.Flush()
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 300, "maximum layers to return")
	cmd.Flags().IntVar(&offset, "offset", 0, "pagination offset")
	return cmd
}

func listAllTemplateCompositions(ctx context.Context, apiClient *client.API, templateID string) ([]api.TemplateComposition, error) {
	const pageSize = 1000
	var compositions []api.TemplateComposition
	for offset := 0; ; offset += pageSize {
		limit := pageSize
		pageOffset := offset
		response, err := apiClient.V3.ListTemplateCompositionsV3WithResponse(ctx, templateID, &api.ListTemplateCompositionsV3Params{
			Limit:  &limit,
			Offset: &pageOffset,
		})
		if err != nil {
			return nil, client.NetworkError(err)
		}
		if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
			return nil, client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
		}
		page := *response.JSON200
		compositions = append(compositions, page...)
		if len(page) < pageSize {
			return compositions, nil
		}
	}
}

func newTemplateUploadCommand(state *State) *cobra.Command {
	var (
		name         string
		templateType string
		noWait       bool
		interval     time.Duration
		waitTimeout  time.Duration
	)
	cmd := &cobra.Command{
		Use: "upload <file>", Short: "Create, upload, and process a template", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			info, err := os.Stat(path)
			if err != nil {
				return clierr.Wrap("input_error", "could not read template file", clierr.ExitUsage, err)
			}
			if !info.Mode().IsRegular() {
				return clierr.Validation("template path is not a regular file", "Pass an .aep, .zip, or .mogrt file.")
			}
			if templateType == "" {
				templateType = strings.TrimPrefix(strings.ToLower(filepath.Ext(path)), ".")
			}
			switch templateType {
			case "aep", "zip", "mogrt":
			default:
				return clierr.Validation("unsupported template type "+templateType, "Use --type aep, --type zip, or --type mogrt.")
			}
			if name == "" {
				name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			}
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			createdResponse, err := apiClient.V2.CreateOrUpdateTemplateWithResponse(cmd.Context(), api.TemplateCreation{DisplayName: name, Type: api.TemplateCreationType(templateType)})
			if err != nil {
				return client.NetworkError(err)
			}
			if createdResponse.StatusCode() != http.StatusCreated {
				return client.HTTPError(createdResponse.StatusCode(), createdResponse.Body, responseHeaders(createdResponse.HTTPResponse))
			}
			created, err := parseTemplateCreationResponse(createdResponse)
			if err != nil {
				return err
			}
			uploadInfo, err := templateUploadInfo(created, createdResponse.Body)
			if err != nil {
				return err
			}
			state.Progressf("Created template %s", *created.Id)
			if err := uploadTemplateFile(cmd.Context(), apiClient, path, uploadInfo); err != nil {
				return err
			}
			state.Progressf("Uploaded %s", filepath.Base(path))
			if noWait {
				return state.Printer(state.Out).Success(created, "Uploaded template "+*created.Id+"; processing continues", nil)
			}
			template, err := waitForTemplate(cmd.Context(), state, apiClient, *created.Id, interval, waitTimeout)
			if err != nil {
				return err
			}
			return state.Printer(state.Out).Success(template, "Template "+template.Id+" is "+string(template.Status), func(w io.Writer) error { return printJSON(w, template) })
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "template display name")
	cmd.Flags().StringVar(&templateType, "type", "", "template type: aep, zip, or mogrt")
	cmd.Flags().BoolVar(&noWait, "no-wait", false, "return after the binary upload")
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "processing poll interval")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", 15*time.Minute, "maximum processing wait")
	return cmd
}

func parseTemplateCreationResponse(response *api.CreateOrUpdateTemplateResponse) (api.Template, error) {
	var envelope struct {
		Template   *api.Template           `json:"template"`
		UploadInfo *api.TemplateUploadInfo `json:"uploadInfo"`
	}
	if err := json.Unmarshal(response.Body, &envelope); err != nil {
		return api.Template{}, clierr.Wrap("invalid_api_response", "could not read the template creation response", clierr.ExitServer, err)
	}
	if envelope.Template != nil {
		created := *envelope.Template
		if envelope.UploadInfo != nil {
			created.UploadInfo = envelope.UploadInfo
		}
		if created.Id != nil {
			return created, nil
		}
	}
	if response.JSON201 != nil && response.JSON201.Id != nil {
		return *response.JSON201, nil
	}
	return api.Template{}, clierr.New("invalid_api_response", "template creation response did not include an id", clierr.ExitServer)
}

func templateUploadInfo(template api.Template, body []byte) (api.TemplateUploadInfo, error) {
	if template.UploadInfo != nil && template.UploadInfo.Url != "" {
		return *template.UploadInfo, nil
	}
	var legacy struct {
		UploadURL string `json:"uploadUrl"`
	}
	if json.Unmarshal(body, &legacy) == nil && legacy.UploadURL != "" {
		return api.TemplateUploadInfo{Method: api.TemplateUploadInfoMethod(http.MethodPut), Url: legacy.UploadURL}, nil
	}
	return api.TemplateUploadInfo{}, clierr.New("upload_url_missing", "template creation did not return an upload URL", clierr.ExitServer)
}

func uploadTemplateFile(ctx context.Context, apiClient *client.API, path string, info api.TemplateUploadInfo) error {
	file, err := os.Open(path)
	if err != nil {
		return clierr.Wrap("input_error", "could not open template file", clierr.ExitUsage, err)
	}
	defer file.Close()
	method := strings.ToUpper(strings.TrimSpace(string(info.Method)))
	if method == "" {
		method = http.MethodPut
	}
	if method != http.MethodPut {
		return clierr.New("unsupported_upload_method", "presigned template upload requested method "+method, clierr.ExitServer)
	}
	request, err := http.NewRequestWithContext(ctx, method, info.Url, file)
	if err != nil {
		return fmt.Errorf("create template upload request: %w", err)
	}
	request.Header.Set("Content-Type", "application/octet-stream")
	if fileInfo, statErr := file.Stat(); statErr == nil {
		request.ContentLength = fileInfo.Size()
	}
	response, err := apiClient.Upload.Do(request)
	if err != nil {
		return client.NetworkError(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 1024*1024))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return &clierr.Error{Code: "template_upload_failed", Message: "template storage upload failed with " + response.Status, Details: map[string]any{"response": strings.TrimSpace(string(body))}, ExitCode: clierr.ExitServer}
	}
	return nil
}

func newTemplateWaitCommand(state *State) *cobra.Command {
	var interval, waitTimeout time.Duration
	cmd := &cobra.Command{
		Use: "wait <id>", Short: "Wait for template processing", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			template, err := waitForTemplate(cmd.Context(), state, apiClient, args[0], interval, waitTimeout)
			if err != nil {
				return err
			}
			return state.Printer(state.Out).Success(template, "Template "+template.Id+" is "+string(template.Status), func(w io.Writer) error { return printJSON(w, template) })
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "processing poll interval")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", 15*time.Minute, "maximum processing wait")
	return cmd
}

func waitForTemplate(ctx context.Context, state *State, apiClient *client.API, id string, interval, waitTimeout time.Duration) (api.TemplateV3, error) {
	if interval <= 0 {
		return api.TemplateV3{}, clierr.Validation("polling interval must be positive", "Use --interval 2s or another positive duration.")
	}
	ctx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()
	lastStatus := ""
	for {
		response, err := apiClient.V3.GetTemplateV3WithResponse(ctx, id)
		if err != nil {
			if ctx.Err() != nil {
				return api.TemplateV3{}, clierr.Wrap("wait_timeout", "timed out waiting for template "+id, clierr.ExitTimeout, ctx.Err())
			}
			return api.TemplateV3{}, client.NetworkError(err)
		}
		if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
			return api.TemplateV3{}, client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
		}
		template := *response.JSON200
		status := string(template.Status)
		if status != lastStatus {
			state.Progressf("Template %s: %s", id, status)
			lastStatus = status
		}
		switch status {
		case "uploaded":
			return template, nil
		case "error":
			return api.TemplateV3{}, &clierr.Error{Code: "template_processing_failed", Message: "template processing failed", Details: map[string]any{"template": template}, ExitCode: clierr.ExitConflict}
		}
		select {
		case <-ctx.Done():
			return api.TemplateV3{}, clierr.Wrap("wait_timeout", "timed out waiting for template "+id, clierr.ExitTimeout, ctx.Err())
		case <-time.After(interval):
		}
	}
}

func newTemplateRenameCommand(state *State) *cobra.Command {
	return &cobra.Command{
		Use: "rename <id> <name>", Short: "Rename a template", Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			body, _ := json.Marshal(api.TemplateUpdate{DisplayName: &args[1]})
			response, err := apiClient.V2.UpdateTemplateWithBodyWithResponse(cmd.Context(), args[0], "application/json", bytes.NewReader(body))
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			return state.Printer(state.Out).Success(*response.JSON200, "Renamed template "+args[0], nil)
		},
	}
}

func newTemplateDownloadCommand(state *State) *cobra.Command {
	var outputPath string
	var force bool
	var urlOnly bool
	cmd := &cobra.Command{
		Use: "download <id>", Short: "Download the template project file", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if urlOnly && (outputPath != "" || force) {
				return clierr.Validation("--url cannot be combined with --output or --force", "Choose a file download or URL-only output.")
			}
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			if !urlOnly && outputPath == "" {
				templateResponse, err := apiClient.V3.GetTemplateV3WithResponse(cmd.Context(), args[0])
				if err != nil {
					return client.NetworkError(err)
				}
				if templateResponse.StatusCode() != http.StatusOK || templateResponse.JSON200 == nil {
					return client.HTTPError(templateResponse.StatusCode(), templateResponse.Body, responseHeaders(templateResponse.HTTPResponse))
				}
				outputPath = templateDownloadFilename(*templateResponse.JSON200)
			}
			response, err := apiClient.V2.GetTemplateDownloadUrlWithResponse(cmd.Context(), args[0])
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() != http.StatusOK || response.JSON200 == nil || response.JSON200.Url == nil {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			url := *response.JSON200.Url
			if urlOnly {
				return state.Printer(state.Out).Success(map[string]string{"id": args[0], "url": url}, "Download URL for "+args[0], func(w io.Writer) error { _, err := fmt.Fprintln(w, url); return err })
			}
			flags := os.O_CREATE | os.O_WRONLY
			if force {
				flags |= os.O_TRUNC
			} else {
				flags |= os.O_EXCL
			}
			file, err := os.OpenFile(outputPath, flags, 0o600)
			if err != nil {
				return clierr.Wrap("output_error", "could not create "+outputPath, clierr.ExitUsage, err)
			}
			succeeded := false
			defer func() {
				file.Close()
				if !succeeded {
					_ = os.Remove(outputPath)
				}
			}()
			downloadRequest, err := http.NewRequestWithContext(cmd.Context(), http.MethodGet, url, nil)
			if err != nil {
				return err
			}
			downloadResponse, err := apiClient.Upload.Do(downloadRequest)
			if err != nil {
				return client.NetworkError(err)
			}
			defer downloadResponse.Body.Close()
			if downloadResponse.StatusCode < 200 || downloadResponse.StatusCode >= 300 {
				return clierr.New("template_download_failed", "template download failed with "+downloadResponse.Status, clierr.ExitServer)
			}
			written, err := io.Copy(file, downloadResponse.Body)
			if err != nil {
				return clierr.Wrap("output_error", "could not write template download", clierr.ExitServer, err)
			}
			if err := file.Close(); err != nil {
				return err
			}
			succeeded = true
			return state.Printer(state.Out).Success(map[string]any{"id": args[0], "path": outputPath, "bytes": written}, "Downloaded template to "+outputPath, nil)
		},
	}
	cmd.Flags().StringVarP(&outputPath, "output", "o", "", "download destination")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite an existing destination")
	cmd.Flags().BoolVar(&urlOnly, "url", false, "print the presigned download URL instead of downloading")
	return cmd
}

func templateDownloadFilename(template api.TemplateV3) string {
	name := strings.TrimSpace(template.DisplayName)
	name = strings.Map(func(r rune) rune {
		if r < 32 || strings.ContainsRune(`<>:"/\\|?*`, r) {
			return '_'
		}
		return r
	}, name)
	name = strings.Trim(name, " .")
	if name == "" {
		name = template.Id
	}
	extension := strings.TrimSpace(string(template.Type))
	if extension != "" && !strings.EqualFold(filepath.Ext(name), "."+extension) {
		name += "." + extension
	}
	return name
}

func newTemplateDeleteCommand(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use: "delete <id>", Short: "Delete a template", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := state.Confirm("Delete template "+args[0]+"?", yes); err != nil {
				return err
			}
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			response, err := apiClient.V2.DeleteTemplateWithResponse(cmd.Context(), args[0])
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() < 200 || response.StatusCode() >= 300 {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			return state.Printer(state.Out).Success(map[string]string{"id": args[0]}, "Deleted template "+args[0], nil)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm deletion")
	return cmd
}

func getTemplate(ctx context.Context, state *State, id string) (api.TemplateV3, error) {
	apiClient, _, err := state.API()
	if err != nil {
		return api.TemplateV3{}, err
	}
	response, err := apiClient.V3.GetTemplateV3WithResponse(ctx, id)
	if err != nil {
		return api.TemplateV3{}, client.NetworkError(err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return api.TemplateV3{}, client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
	}
	return *response.JSON200, nil
}
