package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/nexrender/nexrender-cli/internal/api"
	"github.com/nexrender/nexrender-cli/internal/client"
	"github.com/nexrender/nexrender-cli/internal/clierr"
	"github.com/nexrender/nexrender-cli/internal/diagnostics"
	"github.com/nexrender/nexrender-cli/internal/validate"
	"github.com/spf13/cobra"
)

func newJobCommand(state *State) *cobra.Command {
	cmd := &cobra.Command{Use: "job", Short: "Submit, inspect, wait for, and diagnose render jobs"}
	cmd.AddCommand(newJobListCommand(state))
	cmd.AddCommand(newJobShowCommand(state))
	cmd.AddCommand(newJobLogsCommand(state))
	cmd.AddCommand(newJobSubmitCommand(state))
	cmd.AddCommand(newJobWaitCommand(state))
	cmd.AddCommand(newJobDiagnoseCommand(state))
	cmd.AddCommand(newJobCancelCommand(state))
	cmd.AddCommand(newJobCancelManyCommand(state))
	cmd.AddCommand(newJobJoinCommand(state))
	return cmd
}

func newJobCancelManyCommand(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use: "cancel-many <id> [id...]", Short: "Cancel several queued or pending jobs", Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := state.Confirm(fmt.Sprintf("Cancel %d jobs?", len(args)), yes); err != nil {
				return err
			}
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			response, err := apiClient.V2.CancelJobsWithResponse(cmd.Context(), api.JobCancellationRequest{Jobs: args})
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			return state.Printer(state.Out).Success(*response.JSON200, fmt.Sprintf("Cancelled %d jobs", len(*response.JSON200)), nil)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm cancellation")
	return cmd
}

func newJobJoinCommand(state *State) *cobra.Command {
	var file string
	var dryRun bool
	cmd := &cobra.Command{
		Use: "join", Short: "Create a job that renders or stitches ordered videos", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := readInputFile(state.In, file)
			if err != nil {
				return err
			}
			var payload api.JoinJobCreation
			if err := json.Unmarshal(data, &payload); err != nil {
				return clierr.Validation("join payload is not valid: "+err.Error(), "Run nexrender schema createJoinJob --json.")
			}
			if len(payload.Assets) == 0 {
				return clierr.Validation("join payload requires at least one asset", "Add ordered video or job assets.")
			}
			if dryRun {
				return state.Printer(state.Out).Success(payload, "Join payload is valid; no job was created", func(w io.Writer) error { return printJSON(w, payload) })
			}
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			response, err := apiClient.V2.CreateJoinJobWithResponse(cmd.Context(), payload)
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() != http.StatusCreated || response.JSON201 == nil {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			return state.Printer(state.Out).Success(*response.JSON201, "Submitted join job "+response.JSON201.Id, nil)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "-", "join JSON file, or - for stdin")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate without creating a job")
	return cmd
}

func newJobListCommand(state *State) *cobra.Command {
	var (
		limit, from             int
		states, exclude, sortBy string
		tags, fromDate, toDate  string
		full                    bool
	)
	cmd := &cobra.Command{
		Use: "list", Short: "List render jobs", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			minimalValue := "true"
			if full {
				minimalValue = "false"
			}
			minimal := api.ListJobsParamsMinimal(minimalValue)
			params := &api.ListJobsParams{Limit: &limit, From: &from, Minimal: &minimal}
			if states != "" {
				params.States = &states
			}
			if exclude != "" {
				params.ExcludeStates = &exclude
			}
			if tags != "" {
				params.Tags = &tags
			}
			if sortBy != "" {
				value := api.ListJobsParamsSort(sortBy)
				params.Sort = &value
			}
			if fromDate != "" {
				value, err := time.Parse(time.RFC3339, fromDate)
				if err != nil {
					return clierr.Validation("invalid --from-date", "Use an RFC3339 timestamp.")
				}
				params.FromDate = &value
			}
			if toDate != "" {
				value, err := time.Parse(time.RFC3339, toDate)
				if err != nil {
					return clierr.Validation("invalid --to-date", "Use an RFC3339 timestamp.")
				}
				params.ToDate = &value
			}
			response, err := apiClient.V2.ListJobsWithResponse(cmd.Context(), params)
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			jobs := *response.JSON200
			return state.Printer(state.Out).Success(jobs, fmt.Sprintf("%d jobs", len(jobs)), func(w io.Writer) error {
				table := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
				fmt.Fprintln(table, "ID\tSTATUS\tPROGRESS\tTEMPLATE")
				for _, job := range jobs {
					fmt.Fprintf(table, "%s\t%s\t%s\t%s\n", stringPointer(job.Id), stringPointer(job.Status), floatPointer(job.Progress), stringPointer(job.TemplateId))
				}
				return table.Flush()
			})
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum jobs to return")
	cmd.Flags().IntVar(&from, "from", 0, "pagination offset")
	cmd.Flags().StringVar(&states, "states", "", "comma-separated states to include")
	cmd.Flags().StringVar(&exclude, "exclude-states", "", "comma-separated states to exclude")
	cmd.Flags().StringVar(&sortBy, "sort", "newest_first", "sort order")
	cmd.Flags().StringVar(&tags, "tags", "", "comma-separated tags")
	cmd.Flags().StringVar(&fromDate, "from-date", "", "include jobs created after an RFC3339 time")
	cmd.Flags().StringVar(&toDate, "to-date", "", "include jobs created before an RFC3339 time")
	cmd.Flags().BoolVar(&full, "full", false, "request full job records")
	return cmd
}

func newJobShowCommand(state *State) *cobra.Command {
	return &cobra.Command{
		Use: "show <id>", Short: "Show one render job", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			response, err := apiClient.V2.GetJobWithResponse(cmd.Context(), args[0])
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			return state.Printer(state.Out).Success(*response.JSON200, "Job "+args[0], func(w io.Writer) error { return printJSON(w, response.JSON200) })
		},
	}
}

func newJobLogsCommand(state *State) *cobra.Command {
	return &cobra.Command{
		Use: "logs <id>", Short: "Get render logs for a job", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			response, err := apiClient.V2.GetJobLogsWithResponse(cmd.Context(), args[0])
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() < 200 || response.StatusCode() >= 300 {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			data := map[string]string{"id": args[0], "logs": string(response.Body)}
			return state.Printer(state.Out).Success(data, "Render logs for "+args[0], func(w io.Writer) error {
				_, err := w.Write(response.Body)
				if err == nil && len(response.Body) > 0 && response.Body[len(response.Body)-1] != '\n' {
					_, err = fmt.Fprintln(w)
				}
				return err
			})
		},
	}
}

func newJobSubmitCommand(state *State) *cobra.Command {
	var (
		file        string
		dryRun      bool
		wait        bool
		interval    time.Duration
		waitTimeout time.Duration
	)
	cmd := &cobra.Command{
		Use: "submit", Short: "Validate and submit a render job", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := readInputFile(state.In, file)
			if err != nil {
				return err
			}
			payload, err := validate.JobPayload(data)
			if err != nil {
				return err
			}
			if dryRun {
				return state.Printer(state.Out).Success(payload, "Job payload is valid; no job was created", func(w io.Writer) error { return printJSON(w, payload) })
			}
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			response, err := apiClient.V2.CreateJobWithResponse(cmd.Context(), payload)
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() != http.StatusCreated || response.JSON201 == nil {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			created := *response.JSON201
			if !wait {
				return state.Printer(state.Out).Success(created, "Submitted job "+created.Id, func(w io.Writer) error {
					fmt.Fprintf(w, "Submitted job %s\nStatus: %s\n", created.Id, created.Status)
					if created.MissingFonts != nil && len(*created.MissingFonts) > 0 {
						fmt.Fprintf(w, "Warning: %d missing font entries\n", len(*created.MissingFonts))
					}
					return nil
				})
			}
			job, err := waitForJob(cmd.Context(), state, apiClient, created.Id, interval, waitTimeout)
			if err != nil {
				return err
			}
			return state.Printer(state.Out).Success(job, "Job "+created.Id+" reached "+stringPointer(job.Status), func(w io.Writer) error { return printJSON(w, job) })
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "-", "job JSON file, or - for stdin")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate without creating a job")
	cmd.Flags().BoolVar(&wait, "wait", false, "wait for a terminal job state")
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "polling interval when waiting")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", 30*time.Minute, "maximum time to wait")
	return cmd
}

func newJobWaitCommand(state *State) *cobra.Command {
	var interval, waitTimeout time.Duration
	cmd := &cobra.Command{
		Use: "wait <id>", Short: "Wait for a terminal job state", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			job, err := waitForJob(cmd.Context(), state, apiClient, args[0], interval, waitTimeout)
			if err != nil {
				return err
			}
			return state.Printer(state.Out).Success(job, "Job "+args[0]+" reached "+stringPointer(job.Status), func(w io.Writer) error { return printJSON(w, job) })
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 2*time.Second, "polling interval")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", 30*time.Minute, "maximum time to wait")
	return cmd
}

func waitForJob(ctx context.Context, state *State, apiClient *client.API, id string, interval, waitTimeout time.Duration) (api.Job, error) {
	if interval <= 0 {
		return api.Job{}, clierr.Validation("polling interval must be positive", "Use --interval 2s or another positive duration.")
	}
	ctx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()
	lastStatus := ""
	for {
		response, err := apiClient.V2.GetJobWithResponse(ctx, id)
		if err != nil {
			if ctx.Err() != nil {
				return api.Job{}, clierr.Wrap("wait_timeout", "timed out waiting for job "+id, clierr.ExitTimeout, ctx.Err())
			}
			return api.Job{}, client.NetworkError(err)
		}
		if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
			return api.Job{}, client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
		}
		job := *response.JSON200
		status := stringPointer(job.Status)
		if status != lastStatus {
			state.Progressf("Job %s: %s", id, status)
			lastStatus = status
		}
		switch status {
		case "finished", "error", "manually_cancelled":
			return job, nil
		}
		select {
		case <-ctx.Done():
			return api.Job{}, clierr.Wrap("wait_timeout", "timed out waiting for job "+id, clierr.ExitTimeout, ctx.Err())
		case <-time.After(interval):
		}
	}
}

func newJobDiagnoseCommand(state *State) *cobra.Command {
	return &cobra.Command{
		Use: "diagnose <id>", Short: "Collect read-only evidence about a render job", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			report, err := diagnostics.Job(cmd.Context(), apiClient, args[0])
			if err != nil {
				return err
			}
			return state.Printer(state.Out).Success(report, fmt.Sprintf("Diagnosis found %d observations", len(report.Findings)), func(w io.Writer) error {
				fmt.Fprintf(w, "Job: %s\nStatus: %s\n\n", args[0], stringPointer(report.Job.Status))
				for _, finding := range report.Findings {
					fmt.Fprintf(w, "[%s] %s\n", strings.ToUpper(finding.Severity), finding.Message)
					if finding.Hint != "" {
						fmt.Fprintf(w, "  %s\n", finding.Hint)
					}
				}
				return nil
			})
		},
	}
}

func newJobCancelCommand(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use: "cancel <id>", Short: "Cancel a queued or pending job", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := state.Confirm("Cancel job "+args[0]+"?", yes); err != nil {
				return err
			}
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			response, err := apiClient.V2.CancelJobWithResponse(cmd.Context(), args[0])
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			return state.Printer(state.Out).Success(*response.JSON200, "Cancelled job "+args[0], nil)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm cancellation")
	return cmd
}

func readInputFile(stdin io.Reader, path string) ([]byte, error) {
	var reader io.Reader
	if path == "" || path == "-" {
		reader = stdin
	} else {
		file, err := os.Open(path)
		if err != nil {
			return nil, clierr.Wrap("input_error", "could not open "+path, clierr.ExitUsage, err)
		}
		defer file.Close()
		reader = file
	}
	data, err := io.ReadAll(io.LimitReader(reader, 10*1024*1024))
	if err != nil {
		return nil, clierr.Wrap("input_error", "could not read job payload", clierr.ExitUsage, err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, clierr.Validation("job payload is empty", "Pass a JSON file with --file or pipe JSON to stdin.")
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, clierr.Validation("job payload is not valid JSON: "+err.Error(), "Run nexrender schema createJob --json for the expected shape.")
	}
	return data, nil
}
