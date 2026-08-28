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
