package command

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nexrender/nexrender-cli/internal/api"
	"github.com/nexrender/nexrender-cli/internal/client"
	"github.com/nexrender/nexrender-cli/internal/clierr"
	"github.com/nexrender/nexrender-cli/internal/validate"
	"github.com/spf13/cobra"
)

func newBatchCommand(state *State) *cobra.Command {
	cmd := &cobra.Command{Use: "batch", Short: "Submit and track batches of render jobs"}
	cmd.AddCommand(newBatchSubmitCommand(state))
	cmd.AddCommand(newBatchShowCommand(state))
	cmd.AddCommand(newBatchWaitCommand(state))
	cmd.AddCommand(newBatchCancelCommand(state))
	return cmd
}

func newBatchSubmitCommand(state *State) *cobra.Command {
	var file string
	var dryRun bool
	cmd := &cobra.Command{
		Use: "submit", Short: "Validate and submit a batch", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := readInputFile(state.In, file)
			if err != nil {
				return err
			}
			var payload api.BatchCreation
			if err := json.Unmarshal(data, &payload); err != nil {
				return clierr.Validation("batch payload is not valid: "+err.Error(), "Run nexrender schema createBatch --json.")
			}
			if len(payload.Jobs) == 0 || len(payload.Jobs) > 1000 {
				return clierr.Validation("batch must contain between 1 and 1000 jobs", "Split larger submissions into several batches.")
			}
			for index, job := range payload.Jobs {
				encoded, _ := json.Marshal(job)
				if _, err := validate.JobPayload(encoded); err != nil {
					return clierr.Validation(fmt.Sprintf("batch job %d is invalid: %s", index, err), "Fix the indicated job before submitting the batch.")
				}
			}
			if dryRun {
				return state.Printer(state.Out).Success(payload, fmt.Sprintf("%d batch jobs are valid; no jobs were created", len(payload.Jobs)), func(w io.Writer) error { return printJSON(w, payload) })
			}
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			response, err := apiClient.V2.CreateBatchWithResponse(cmd.Context(), payload)
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() != http.StatusCreated || response.JSON201 == nil {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			return state.Printer(state.Out).Success(*response.JSON201, "Submitted batch "+response.JSON201.BatchId, nil)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "-", "batch JSON file, or - for stdin")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "validate without creating jobs")
	return cmd
}

func newBatchShowCommand(state *State) *cobra.Command {
	return &cobra.Command{
		Use: "show <id>", Short: "Show batch status", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			batch, err := getBatch(cmd.Context(), apiClient, args[0])
			if err != nil {
				return err
			}
			return state.Printer(state.Out).Success(batch, "Batch "+args[0]+" is "+string(batch.Status), func(w io.Writer) error { return printJSON(w, batch) })
		},
	}
}

func newBatchWaitCommand(state *State) *cobra.Command {
	var interval, waitTimeout time.Duration
	cmd := &cobra.Command{
		Use: "wait <id>", Short: "Wait until no batch jobs are pending", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			batch, err := waitForBatch(cmd.Context(), state, apiClient, args[0], interval, waitTimeout)
			if err != nil {
				return err
			}
			return state.Printer(state.Out).Success(batch, "Batch "+args[0]+" is "+string(batch.Status), func(w io.Writer) error { return printJSON(w, batch) })
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 3*time.Second, "polling interval")
	cmd.Flags().DurationVar(&waitTimeout, "wait-timeout", 2*time.Hour, "maximum time to wait")
	return cmd
}

func waitForBatch(ctx context.Context, state *State, apiClient *client.API, id string, interval, waitTimeout time.Duration) (api.BatchStatusResponse, error) {
	if interval <= 0 {
		return api.BatchStatusResponse{}, clierr.Validation("polling interval must be positive", "Use --interval 3s or another positive duration.")
	}
	ctx, cancel := context.WithTimeout(ctx, waitTimeout)
	defer cancel()
	lastStatus := ""
	for {
		batch, err := getBatch(ctx, apiClient, id)
		if err != nil {
			if ctx.Err() != nil {
				return api.BatchStatusResponse{}, clierr.Wrap("wait_timeout", "timed out waiting for batch "+id, clierr.ExitTimeout, ctx.Err())
			}
			return api.BatchStatusResponse{}, err
		}
		status := string(batch.Status)
		if status != lastStatus {
			state.Progressf("Batch %s: %s", id, status)
			lastStatus = status
		}
		if batch.Stats.Pending != nil && *batch.Stats.Pending == 0 {
			return batch, nil
		}
		select {
		case <-ctx.Done():
			return api.BatchStatusResponse{}, clierr.Wrap("wait_timeout", "timed out waiting for batch "+id, clierr.ExitTimeout, ctx.Err())
		case <-time.After(interval):
		}
	}
}

func getBatch(ctx context.Context, apiClient *client.API, id string) (api.BatchStatusResponse, error) {
	response, err := apiClient.V2.GetBatchStatusWithResponse(ctx, id)
	if err != nil {
		return api.BatchStatusResponse{}, client.NetworkError(err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return api.BatchStatusResponse{}, client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
	}
	return *response.JSON200, nil
}

func newBatchCancelCommand(state *State) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use: "cancel <id>", Short: "Cancel cancellable jobs in a batch", Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := state.Confirm("Cancel batch "+args[0]+"?", yes); err != nil {
				return err
			}
			apiClient, _, err := state.API()
			if err != nil {
				return err
			}
			response, err := apiClient.V2.CancelBatchWithResponse(cmd.Context(), args[0])
			if err != nil {
				return client.NetworkError(err)
			}
			if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
				return client.HTTPError(response.StatusCode(), response.Body, responseHeaders(response.HTTPResponse))
			}
			return state.Printer(state.Out).Success(*response.JSON200, "Batch cancellation is "+string(response.JSON200.Status), nil)
		},
	}
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "confirm cancellation")
	return cmd
}
