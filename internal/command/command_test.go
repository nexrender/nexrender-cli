package command

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nexrender/nexrender-cli/internal/api"
	"github.com/nexrender/nexrender-cli/internal/auth"
	"github.com/nexrender/nexrender-cli/internal/config"
)

type memoryCredentials struct {
	token string
}

func (m *memoryCredentials) Get(string) (auth.Credential, error) {
	if m.token == "" {
		return auth.Credential{}, os.ErrNotExist
	}
	return auth.Credential{Token: m.token, Source: "test"}, nil
}

func (m *memoryCredentials) Set(_ string, token string) (string, error) {
	m.token = token
	return "test", nil
}

func (m *memoryCredentials) Delete(string) error {
	m.token = ""
	return nil
}

func testState(t *testing.T, server string, input io.Reader) (*State, *bytes.Buffer, *bytes.Buffer) {
	t.Helper()
	var out, stderr bytes.Buffer
	store := &config.Store{Path: filepath.Join(t.TempDir(), "config.json")}
	cfg := config.Defaults()
	cfg.Profiles[config.DefaultProfile] = config.Profile{Server: server}
	if err := store.Save(cfg); err != nil {
		t.Fatal(err)
	}
	return &State{
		In:         input,
		Out:        &out,
		Err:        &stderr,
		Timeout:    0,
		AutoJSON:   false,
		Config:     store,
		Credential: &memoryCredentials{token: "test-token"},
	}, &out, &stderr
}

func TestJobListUsesV2AndBearerAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/jobs" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[{"id":"job-1","status":"queued","outputUrl":null,"templateId":"tpl-1"}]`)
	}))
	defer server.Close()
	state, out, _ := testState(t, server.URL+"/api/v2", strings.NewReader(""))
	if code := Execute(context.Background(), state, []string{"--json", "job", "list"}); code != 0 {
		t.Fatalf("exit = %d, output = %s", code, out.String())
	}
	if !strings.Contains(out.String(), `"job-1"`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestTemplateUploadDoesNotForwardBearer(t *testing.T) {
	var uploaded []byte
	uploadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("bearer token was forwarded to upload host: %q", got)
		}
		var err error
		uploaded, err = io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer uploadServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("missing API authorization for %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/v2/templates":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(map[string]any{
				"template": map[string]any{
					"id": "tpl-1", "displayName": "Example", "type": "aep", "status": "awaiting_upload", "error": nil,
				},
				"uploadInfo": map[string]any{"url": uploadServer.URL, "method": "PUT", "key": "key", "expiresIn": 3600},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v3/templates/tpl-1":
			json.NewEncoder(w).Encode(map[string]any{
				"id": "tpl-1", "displayName": "Example", "type": "aep", "status": "uploaded",
				"createdAt": "2026-08-28T00:00:00Z", "updatedAt": "2026-08-28T00:00:01Z",
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiServer.Close()

	templatePath := filepath.Join(t.TempDir(), "example.aep")
	if err := os.WriteFile(templatePath, []byte("after-effects-project"), 0o600); err != nil {
		t.Fatal(err)
	}
	state, out, _ := testState(t, apiServer.URL+"/api/v2", strings.NewReader(""))
	code := Execute(context.Background(), state, []string{"--json", "template", "upload", templatePath, "--interval", "1ms", "--wait-timeout", "1s"})
	if code != 0 {
		t.Fatalf("exit = %d, output = %s", code, out.String())
	}
	if string(uploaded) != "after-effects-project" {
		t.Fatalf("uploaded body = %q", uploaded)
	}
}

func TestTemplateDownloadDownloadsByDefaultWithoutForwardingBearer(t *testing.T) {
	var downloaded bool
	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloaded = true
		if got := r.Header.Get("Authorization"); got != "" {
			t.Fatalf("bearer token was forwarded to download host: %q", got)
		}
		io.WriteString(w, "after-effects-project")
	}))
	defer downloadServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("missing API authorization for %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v3/templates/tpl-1":
			io.WriteString(w, `{"id":"tpl-1","displayName":"Sponsor Endcard","type":"aep","status":"uploaded","createdAt":"2026-08-28T00:00:00Z","updatedAt":"2026-08-28T00:00:01Z"}`)
		case "/api/v2/templates/tpl-1/download":
			json.NewEncoder(w).Encode(map[string]string{"url": downloadServer.URL})
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiServer.Close()

	downloadDir := t.TempDir()
	t.Chdir(downloadDir)
	state, out, _ := testState(t, apiServer.URL+"/api/v2", strings.NewReader(""))
	if code := Execute(context.Background(), state, []string{"--json", "template", "download", "tpl-1"}); code != 0 {
		t.Fatalf("exit = %d, output = %s", code, out.String())
	}
	if !downloaded {
		t.Fatal("presigned download URL was not requested")
	}
	path := filepath.Join(downloadDir, "Sponsor Endcard.aep")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "after-effects-project" {
		t.Fatalf("downloaded body = %q", data)
	}
	if !strings.Contains(out.String(), `"path": "Sponsor Endcard.aep"`) {
		t.Fatalf("download path missing from output: %s", out.String())
	}
}

func TestTemplateDownloadURLFlagDoesNotDownload(t *testing.T) {
	var downloaded bool
	downloadServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		downloaded = true
		io.WriteString(w, "should-not-download")
	}))
	defer downloadServer.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("missing API authorization for %s", r.URL.Path)
		}
		if r.URL.Path != "/api/v2/templates/tpl-1/download" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"url": downloadServer.URL})
	}))
	defer apiServer.Close()

	state, out, _ := testState(t, apiServer.URL+"/api/v2", strings.NewReader(""))
	if code := Execute(context.Background(), state, []string{"--json", "template", "download", "tpl-1", "--url"}); code != 0 {
		t.Fatalf("exit = %d, output = %s", code, out.String())
	}
	if downloaded {
		t.Fatal("--url should not download the template")
	}
	if !strings.Contains(out.String(), downloadServer.URL) {
		t.Fatalf("download URL missing from output: %s", out.String())
	}
}

func TestTemplateDownloadFilename(t *testing.T) {
	tests := []struct {
		name     string
		template api.TemplateV3
		want     string
	}{
		{name: "adds extension", template: api.TemplateV3{Id: "tpl-1", DisplayName: "Sponsor Endcard", Type: "aep"}, want: "Sponsor Endcard.aep"},
		{name: "keeps extension", template: api.TemplateV3{Id: "tpl-1", DisplayName: "Sponsor Endcard.AEP", Type: "aep"}, want: "Sponsor Endcard.AEP"},
		{name: "removes path characters", template: api.TemplateV3{Id: "tpl-1", DisplayName: `Client/Project\Final`, Type: "zip"}, want: "Client_Project_Final.zip"},
		{name: "falls back to id", template: api.TemplateV3{Id: "tpl-1", DisplayName: "...", Type: "mogrt"}, want: "tpl-1.mogrt"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := templateDownloadFilename(test.template); got != test.want {
				t.Fatalf("filename = %q, want %q", got, test.want)
			}
		})
	}
}

func TestTemplateLayersShowCompositionNameInsteadOfID(t *testing.T) {
	var listedLayers, listedCompositions bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Fatalf("missing API authorization for %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v3/templates/tpl-1/layers":
			listedLayers = true
			io.WriteString(w, `[{"aeid":7,"composition_id":42,"data":{},"height":100,"in_point":0,"layer_type":"TextLayer","left":0,"name":"Headline","out_point":5,"parent_id":null,"source_comp_id":null,"source_type":null,"start_time":0,"top":0,"width":400}]`)
		case "/api/v3/templates/tpl-1/compositions":
			listedCompositions = true
			io.WriteString(w, `[{"aeid":"42","data":{},"duration":5,"frame_rate":30,"height":1080,"name":"Main","width":1920}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	state, out, _ := testState(t, server.URL+"/api/v2", strings.NewReader(""))
	if code := Execute(context.Background(), state, []string{"--json", "template", "layers", "tpl-1"}); code != 0 {
		t.Fatalf("exit = %d, output = %s", code, out.String())
	}
	if !listedLayers || !listedCompositions {
		t.Fatalf("expected layer and composition requests, layers=%t compositions=%t", listedLayers, listedCompositions)
	}
	if !strings.Contains(out.String(), `"composition_name": "Main"`) {
		t.Fatalf("composition name missing from output: %s", out.String())
	}
	if strings.Contains(out.String(), `"composition_id"`) {
		t.Fatalf("composition id should not be shown: %s", out.String())
	}

	humanState, humanOut, _ := testState(t, server.URL+"/api/v2", strings.NewReader(""))
	if code := Execute(context.Background(), humanState, []string{"template", "layers", "tpl-1"}); code != 0 {
		t.Fatalf("human output exit = %d, output = %s", code, humanOut.String())
	}
	if !strings.Contains(humanOut.String(), "COMPOSITION NAME") || !strings.Contains(humanOut.String(), "Main") {
		t.Fatalf("composition name missing from human output: %s", humanOut.String())
	}
	if strings.Contains(humanOut.String(), "42") {
		t.Fatalf("composition id should not be shown in human output: %s", humanOut.String())
	}
}

func TestDryRunRejectsPreviewSettingsWithoutCallingAPI(t *testing.T) {
	state, out, _ := testState(t, "http://127.0.0.1:1/api/v2", strings.NewReader(`{"template":{"id":"tpl"},"preview":true,"settings":{"type":"video"}}`))
	code := Execute(context.Background(), state, []string{"--json", "job", "submit", "--dry-run", "--file", "-"})
	if code == 0 {
		t.Fatalf("expected validation failure: %s", out.String())
	}
	if !strings.Contains(out.String(), "preview and settings") {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestSchemaCommandFindsReferencedSchemas(t *testing.T) {
	state, out, _ := testState(t, "https://api.nexrender.com/api/v2", strings.NewReader(""))
	state.Credential = &memoryCredentials{}
	if code := Execute(context.Background(), state, []string{"--json", "schema", "createJob"}); code != 0 {
		t.Fatalf("exit = %d, output = %s", code, out.String())
	}
	if !strings.Contains(out.String(), `"JobCreation"`) || !strings.Contains(out.String(), `"POST"`) {
		t.Fatalf("unexpected schema output: %s", out.String())
	}
}

func TestMemoryCredentialDelete(t *testing.T) {
	store := &memoryCredentials{token: "value"}
	if err := store.Delete("default"); err != nil {
		t.Fatal(err)
	}
	_, err := store.Get("default")
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected missing credential, got %v", err)
	}
}

func TestMissingArgumentUsesUsageExitCode(t *testing.T) {
	state, out, _ := testState(t, "https://api.nexrender.com/api/v2", strings.NewReader(""))
	code := Execute(context.Background(), state, []string{"--json", "job", "show"})
	if code != 2 {
		t.Fatalf("exit = %d, want 2; output = %s", code, out.String())
	}
	if !strings.Contains(out.String(), `"code": "usage_error"`) {
		t.Fatalf("unexpected output: %s", out.String())
	}
}

func TestSetupShowsWhereToCreateAPIToken(t *testing.T) {
	state, out, stderr := testState(t, "https://api.nexrender.com/api/v2", strings.NewReader(""))
	state.Credential = &memoryCredentials{}

	if code := Execute(context.Background(), state, []string{"setup", "--skip-agents"}); code == 0 {
		t.Fatalf("expected non-interactive setup to require token input: %s", out.String())
	}
	if !strings.Contains(stderr.String(), apiTokenSettingsURL) {
		t.Fatalf("setup did not show API token URL: %s", stderr.String())
	}
}
