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
	shortCodec := []byte(`{"template":{"id":"tpl"},"settings":{"type":"video","codec":"h264"}}`)
	if _, err := JobPayload(shortCodec); err == nil {
		t.Fatal("expected short codec to fail")
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
