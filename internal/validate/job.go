package validate

import (
	"encoding/json"
	"strings"

	"github.com/nexrender/nexrender-cli/internal/api"
	"github.com/nexrender/nexrender-cli/internal/clierr"
)

func JobPayload(data []byte) (api.JobCreation, error) {
	var payload api.JobCreation
	if err := json.Unmarshal(data, &payload); err != nil {
		return api.JobCreation{}, clierr.Validation("job payload is not valid JSON: "+err.Error(), "Run nexrender schema createJob --json to inspect the request schema.")
	}
	id := trimmed(payload.Template.Id)
	src := trimmed(payload.Template.Src)
	switch {
	case id == "" && src == "":
		return api.JobCreation{}, clierr.Validation("template.id or template.src is required", "Select an uploaded template with nexrender template list --json, or put a template URL in template.src.")
	case id != "" && src != "":
		return api.JobCreation{}, clierr.Validation("template.id and template.src cannot be used together", "Keep template.id for an uploaded template, or template.src for a template URL.")
	case src != "" && trimmed(payload.Template.Name) == "":
		return api.JobCreation{}, clierr.Validation("template.name is required with template.src", "Set it to the .aep or .mogrt file name, or to the .aep path inside a .zip.")
	}
	if payload.Preview != nil && *payload.Preview && payload.Settings != nil {
		return api.JobCreation{}, clierr.Validation("preview and settings cannot be used together", "Remove settings for a preview job, or set preview to false.")
	}
	return payload, nil
}

func trimmed(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
