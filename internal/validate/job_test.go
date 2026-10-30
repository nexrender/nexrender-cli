package validate

import "testing"

func TestJobPayloadRules(t *testing.T) {
	valid := []byte(`{"template":{"id":"tpl","composition":"Main"},"settings":{"type":"video","codec":"video_h264_vbr_15mbps"}}`)
	if _, err := JobPayload(valid); err != nil {
		t.Fatalf("valid payload failed: %v", err)
	}
	previewWithSettings := []byte(`{"template":{"id":"tpl"},"preview":true,"settings":{"type":"video"}}`)
	if _, err := JobPayload(previewWithSettings); err == nil {
		t.Fatal("expected preview/settings conflict")
	}
}

// The API matches codecs by suffix, and its OpenAPI examples use short names, so the
// CLI must not reject them. Unknown codecs are left to the API, which lists the options.
func TestJobPayloadAcceptsShortAndFullCodecNames(t *testing.T) {
	for _, codec := range []string{"png", "jpeg", "h264_vbr_5mbps", "prores_4444", "gif", "image_png", "video_h264_vbr_15mbps"} {
		payload := []byte(`{"template":{"id":"tpl","composition":"Main"},"settings":{"type":"video","codec":"` + codec + `"}}`)
		if _, err := JobPayload(payload); err != nil {
			t.Fatalf("codec %q was rejected: %v", codec, err)
		}
	}
}
