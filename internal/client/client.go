package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/nexrender/nexrender-cli/internal/api"
	"github.com/nexrender/nexrender-cli/internal/clierr"
)

type API struct {
	V2     *api.ClientWithResponses
	V3     *api.ClientWithResponses
	Upload *http.Client
}

func New(server, token string, timeout time.Duration) (*API, error) {
	server = strings.TrimRight(server, "/")
	parsed, err := url.Parse(server)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, clierr.Validation("invalid API server URL", "Use a full URL such as https://api.nexrender.com/api/v2.")
	}
	httpClient := &http.Client{Timeout: timeout}
	editor := func(_ context.Context, req *http.Request) error {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("User-Agent", "nexrender-cli")
		return nil
	}
	v2, err := api.NewClientWithResponses(server, api.WithHTTPClient(httpClient), api.WithRequestEditorFn(editor))
	if err != nil {
		return nil, fmt.Errorf("create v2 client: %w", err)
	}
	v3, err := api.NewClientWithResponses(v3Root(server), api.WithHTTPClient(httpClient), api.WithRequestEditorFn(editor))
	if err != nil {
		return nil, fmt.Errorf("create v3 client: %w", err)
	}
	uploadTimeout := 30 * time.Minute
	if timeout > uploadTimeout {
		uploadTimeout = timeout
	}
	return &API{V2: v2, V3: v3, Upload: &http.Client{Timeout: uploadTimeout}}, nil
}

func v3Root(server string) string {
	trimmed := strings.TrimRight(server, "/")
	if strings.HasSuffix(trimmed, "/v2") {
		return strings.TrimSuffix(trimmed, "/v2")
	}
	return trimmed
}

func HTTPError(status int, body []byte, headers http.Header) error {
	message := strings.TrimSpace(http.StatusText(status))
	var payload map[string]any
	if json.Unmarshal(body, &payload) == nil {
		if value, ok := payload["error"].(string); ok && strings.TrimSpace(value) != "" {
			message = value
		}
	}
	code := "api_error"
	exitCode := clierr.ExitServer
	hint := ""
	switch status {
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		code, exitCode = "api_validation_failed", clierr.ExitValidation
	case http.StatusUnauthorized, http.StatusForbidden:
		code, exitCode = "authentication_failed", clierr.ExitAuth
		hint = "Run nexrender auth status, then nexrender auth login if the credential is missing or invalid."
	case http.StatusNotFound, http.StatusGone:
		code, exitCode = "not_found", clierr.ExitNotFound
	case http.StatusConflict:
		code, exitCode = "state_conflict", clierr.ExitConflict
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		code, exitCode = "api_timeout", clierr.ExitTimeout
	}
	details := map[string]any{"status": status}
	if requestID := headers.Get("X-Request-Id"); requestID != "" {
		details["requestId"] = requestID
	}
	if payload != nil {
		details["response"] = payload
	}
	return &clierr.Error{Code: code, Message: message, Hint: hint, Details: details, ExitCode: exitCode}
}

func NetworkError(err error) error {
	if err == nil {
		return nil
	}
	return clierr.Wrap("network_error", "could not reach the Nexrender API", clierr.ExitNetwork, err)
}
