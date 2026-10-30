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
	shortCodec := []byte(`{"template":{"id":"tpl"},"settings":{"type":"video","codec":"h264"}}`)
	if _, err := JobPayload(shortCodec); err == nil {
		t.Fatal("expected short codec to fail")
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
