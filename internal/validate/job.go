package validate

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nexrender/nexrender-cli/internal/api"
	"github.com/nexrender/nexrender-cli/internal/clierr"
)

func JobPayload(data []byte) (api.JobCreation, error) {
	var payload api.JobCreation
	if err := json.Unmarshal(data, &payload); err != nil {
		return api.JobCreation{}, clierr.Validation("job payload is not valid JSON: "+err.Error(), "Run nexrender schema createJob --json to inspect the request schema.")
	}
	if strings.TrimSpace(payload.Template.Id) == "" {
		return api.JobCreation{}, clierr.Validation("template.id is required", "Select a template with nexrender template list --json.")
	}
	if payload.Preview != nil && *payload.Preview && payload.Settings != nil {
		return api.JobCreation{}, clierr.Validation("preview and settings cannot be used together", "Remove settings for a preview job, or set preview to false.")
	}
	if payload.Settings != nil && payload.Settings.Codec != nil {
		codec := strings.TrimSpace(*payload.Settings.Codec)
		if codec != "" && !strings.HasPrefix(codec, "video_") && !strings.HasPrefix(codec, "image_") {
			return api.JobCreation{}, clierr.Validation(
				fmt.Sprintf("codec %q is missing its output prefix", codec),
				"Use a full identifier such as video_h264_vbr_15mbps, video_prores_422, or image_png.",
			)
		}
	}
	return payload, nil
}
