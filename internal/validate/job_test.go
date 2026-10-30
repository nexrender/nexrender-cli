package validate

import (
	"encoding/json"
	"strings"
	"testing"
)

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

func TestJobPayloadTemplateSource(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		valid   bool
	}{
		{"uploaded template", `{"template":{"id":"tpl","composition":"Main"}}`, true},
		{"template URL", `{"template":{"src":"https://nx1-assets-eu.nexrender.com/example/Test%20AEP.zip","name":"LTExample.aep","composition":"Final 2"}}`, true},
		{"neither", `{"template":{"composition":"Main"}}`, false},
		{"both", `{"template":{"id":"tpl","src":"https://nx1-assets-eu.nexrender.com/example/Test%20AEP.zip","name":"LTExample.aep","composition":"Main"}}`, false},
		{"src without name", `{"template":{"src":"https://nx1-assets-eu.nexrender.com/example/Test%20AEP.zip","composition":"Final 2"}}`, false},
	}
	for _, tc := range cases {
		_, err := JobPayload([]byte(tc.payload))
		if tc.valid && err != nil {
			t.Errorf("%s: unexpected error %v", tc.name, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("%s: expected a validation error", tc.name)
		}
	}
}

func TestJobPayloadKeepsMultiFrame(t *testing.T) {
	for _, tc := range []struct {
		name    string
		payload string
		want    string
	}{
		{"off", `{"template":{"id":"tpl","composition":"Main"},"settings":{"multiFrame":false}}`, `"multiFrame":false`},
		{"on", `{"template":{"id":"tpl","composition":"Main"},"settings":{"multiFrame":true}}`, `"multiFrame":true`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			payload, err := JobPayload([]byte(tc.payload))
			if err != nil {
				t.Fatalf("payload failed: %v", err)
			}
			encoded, err := json.Marshal(payload)
			if err != nil {
				t.Fatalf("marshal payload: %v", err)
			}
			if !strings.Contains(string(encoded), tc.want) {
				t.Fatalf("submitted payload %s does not contain %s", encoded, tc.want)
			}
		})
	}

	omitted, err := JobPayload([]byte(`{"template":{"id":"tpl","composition":"Main"},"settings":{"type":"video"}}`))
	if err != nil {
		t.Fatalf("payload failed: %v", err)
	}
	encoded, err := json.Marshal(omitted)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	if strings.Contains(string(encoded), "multiFrame") {
		t.Fatalf("submitted payload %s should leave multiFrame out when it is not set", encoded)
	}
}
