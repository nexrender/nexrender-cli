package diagnostics

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/nexrender/nexrender-cli/internal/api"
	"github.com/nexrender/nexrender-cli/internal/client"
)

type Finding struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
}

type Report struct {
	Job          api.Job                   `json:"job"`
	Logs         string                    `json:"logs,omitempty"`
	MissingFonts []string                  `json:"missingFonts,omitempty"`
	Template     *api.TemplateV3           `json:"template,omitempty"`
	Compositions []api.TemplateComposition `json:"compositions,omitempty"`
	Layers       []api.TemplateLayer       `json:"layers,omitempty"`
	Findings     []Finding                 `json:"findings"`
}

func Job(ctx context.Context, apiClient *client.API, id string) (Report, error) {
	response, err := apiClient.V2.GetJobWithResponse(ctx, id)
	if err != nil {
		return Report{}, client.NetworkError(err)
	}
	if response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		return Report{}, client.HTTPError(response.StatusCode(), response.Body, response.HTTPResponse.Header)
	}
	report := Report{Job: *response.JSON200, Findings: []Finding{}}
	report.MissingFonts = missingFonts(response.Body)

	logsResponse, logsErr := apiClient.V2.GetJobLogsWithResponse(ctx, id)
	if logsErr == nil && logsResponse.StatusCode() >= 200 && logsResponse.StatusCode() < 300 {
		report.Logs = strings.TrimSpace(string(logsResponse.Body))
	} else {
		report.Findings = append(report.Findings, Finding{Severity: "info", Code: "logs_unavailable", Message: "Render logs were not available."})
	}

	status := stringValue(report.Job.Status)
	if status == "error" {
		message := "The render job failed."
		if report.Job.Stats != nil && report.Job.Stats.Error != nil && strings.TrimSpace(*report.Job.Stats.Error) != "" {
			message = strings.TrimSpace(*report.Job.Stats.Error)
		}
		report.Findings = append(report.Findings, Finding{Severity: "error", Code: "job_failed", Message: message, Hint: "Inspect the logs and payload before submitting another paid render."})
	}
	if status == "manually_cancelled" {
		report.Findings = append(report.Findings, Finding{Severity: "info", Code: "job_cancelled", Message: "The job was cancelled manually."})
	}
	if len(report.MissingFonts) > 0 {
		report.Findings = append(report.Findings, Finding{
			Severity: "warning",
			Code:     "missing_fonts",
			Message:  fmt.Sprintf("The render reports %d missing font entries.", len(report.MissingFonts)),
			Hint:     "Upload or repair the fonts and submit a new job only after checking the visual result.",
		})
	}

	if report.Job.TemplateId != nil && strings.TrimSpace(*report.Job.TemplateId) != "" {
		loadTemplateContext(ctx, apiClient, strings.TrimSpace(*report.Job.TemplateId), &report)
	}
	if strings.Contains(strings.ToLower(report.Logs), "layer") {
		report.Findings = append(report.Findings, Finding{
			Severity: "info",
			Code:     "check_layer_names",
			Message:  "The render logs mention a layer.",
			Hint:     "Compare payload layerName values with the exact case-sensitive names in the layers result.",
		})
	}
	if len(report.Findings) == 0 {
		report.Findings = append(report.Findings, Finding{Severity: "info", Code: "no_known_issue", Message: "No known Nexrender issue was detected from the available data."})
	}
	return report, nil
}

func loadTemplateContext(ctx context.Context, apiClient *client.API, id string, report *Report) {
	response, err := apiClient.V3.GetTemplateV3WithResponse(ctx, id)
	if err != nil || response.StatusCode() != http.StatusOK || response.JSON200 == nil {
		report.Findings = append(report.Findings, Finding{Severity: "info", Code: "template_unavailable", Message: "Template metadata could not be loaded."})
		return
	}
	report.Template = response.JSON200
	status := string(response.JSON200.Status)
	if status != "uploaded" {
		report.Findings = append(report.Findings, Finding{Severity: "warning", Code: "template_not_ready", Message: "Template status is " + status + ".", Hint: "Wait for an uploaded status or inspect the template processing error."})
	}
	limit := 1000
	offset := 0
	compositions, err := apiClient.V3.ListTemplateCompositionsV3WithResponse(ctx, id, &api.ListTemplateCompositionsV3Params{Limit: &limit, Offset: &offset})
	if err == nil && compositions.StatusCode() == http.StatusOK && compositions.JSON200 != nil {
		report.Compositions = *compositions.JSON200
	}
	layers, err := apiClient.V3.ListTemplateLayersV3WithResponse(ctx, id, &api.ListTemplateLayersV3Params{Limit: &limit, Offset: &offset})
	if err == nil && layers.StatusCode() == http.StatusOK && layers.JSON200 != nil {
		report.Layers = *layers.JSON200
	}
}

func missingFonts(body []byte) []string {
	var payload map[string]any
	if json.Unmarshal(body, &payload) != nil {
		return nil
	}
	values, ok := payload["missingFonts"].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
