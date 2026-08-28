package command

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/nexrender/nexrender-cli/internal/auth"
	"github.com/nexrender/nexrender-cli/internal/client"
	"github.com/nexrender/nexrender-cli/internal/clierr"
	"github.com/nexrender/nexrender-cli/internal/config"
	"github.com/nexrender/nexrender-cli/internal/output"
	"golang.org/x/term"
)

type State struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer

	JSON    bool
	Quiet   bool
	JQ      string
	Profile string
	Server  string
	Timeout time.Duration

	AutoJSON   bool
	Config     *config.Store
	Credential auth.Store
}

type Resolved struct {
	Profile          string `json:"profile"`
	Server           string `json:"server"`
	CredentialSource string `json:"credentialSource,omitempty"`
	Token            string `json:"-"`
}

func DefaultState() (*State, error) {
	configStore, err := config.NewDefaultStore()
	if err != nil {
		return nil, err
	}
	credentialStore, err := auth.NewDefaultStore()
	if err != nil {
		return nil, err
	}
	return &State{
		In:         os.Stdin,
		Out:        os.Stdout,
		Err:        os.Stderr,
		Timeout:    30 * time.Second,
		AutoJSON:   true,
		Config:     configStore,
		Credential: credentialStore,
	}, nil
}

func (s *State) Structured() bool {
	if s.JSON || s.Quiet || s.JQ != "" {
		return true
	}
	if !s.AutoJSON {
		return false
	}
	file, ok := s.Out.(*os.File)
	return ok && !term.IsTerminal(int(file.Fd()))
}

func (s *State) Printer(out io.Writer) output.Printer {
	return output.Printer{Out: out, JSON: s.Structured(), Quiet: s.Quiet, JQ: s.JQ}
}

func (s *State) Resolve(requireToken bool) (Resolved, error) {
	cfg, err := s.Config.Load()
	if err != nil {
		return Resolved{}, clierr.Wrap("config_error", "could not load Nexrender configuration", clierr.ExitServer, err)
	}
	profileName := strings.TrimSpace(s.Profile)
	if profileName == "" {
		profileName = strings.TrimSpace(os.Getenv("NEXRENDER_PROFILE"))
	}
	if profileName == "" {
		profileName = cfg.ActiveProfile
	}
	profile, ok := cfg.Profiles[profileName]
	if !ok {
		return Resolved{}, clierr.Validation("unknown profile "+profileName, "Run nexrender profile list to see configured profiles.")
	}
	server := config.NormalizeServer(s.Server)
	if server == "" {
		server = config.NormalizeServer(os.Getenv("NEXRENDER_API_URL"))
	}
	if server == "" {
		server = config.NormalizeServer(profile.Server)
	}
	if server == "" {
		server = config.DefaultServer
	}
	resolved := Resolved{Profile: profileName, Server: server}
	credential, err := s.Credential.Get(profileName)
	if err == nil {
		resolved.Token = credential.Token
		resolved.CredentialSource = credential.Source
		return resolved, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return Resolved{}, clierr.Wrap("credential_error", "could not read Nexrender credentials", clierr.ExitAuth, err)
	}
	if requireToken {
		return Resolved{}, &clierr.Error{
			Code:     "authentication_required",
			Message:  "no Nexrender API credential is available for profile " + profileName,
			Hint:     "Run nexrender auth login or set NEXRENDER_API_KEY.",
			ExitCode: clierr.ExitAuth,
		}
	}
	return resolved, nil
}

func (s *State) API() (*client.API, Resolved, error) {
	resolved, err := s.Resolve(true)
	if err != nil {
		return nil, Resolved{}, err
	}
	apiClient, err := client.New(resolved.Server, resolved.Token, s.Timeout)
	if err != nil {
		return nil, Resolved{}, err
	}
	return apiClient, resolved, nil
}

func (s *State) Progressf(format string, args ...any) {
	if s.Structured() {
		return
	}
	fmt.Fprintf(s.Err, format+"\n", args...)
}

func (s *State) Confirm(prompt string, yes bool) error {
	if yes {
		return nil
	}
	file, ok := s.In.(*os.File)
	if !ok || !term.IsTerminal(int(file.Fd())) {
		return clierr.Usage("confirmation required; rerun with --yes")
	}
	fmt.Fprintf(s.Err, "%s [y/N] ", prompt)
	line, err := bufio.NewReader(s.In).ReadString('\n')
	if err != nil {
		return fmt.Errorf("read confirmation: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	if answer != "y" && answer != "yes" {
		return clierr.New("cancelled", "operation cancelled", clierr.ExitConflict)
	}
	return nil
}

func (s *State) ReadSecret(fromStdin bool) (string, error) {
	return s.ReadHidden("Nexrender API token", fromStdin)
}

func (s *State) ReadHidden(label string, fromStdin bool) (string, error) {
	if fromStdin {
		data, err := io.ReadAll(io.LimitReader(s.In, 64*1024))
		if err != nil {
			return "", fmt.Errorf("read token from stdin: %w", err)
		}
		value := strings.TrimSpace(string(data))
		if value == "" {
			return "", clierr.Validation("token from stdin was empty", "Pipe the API token to nexrender auth login --token-stdin.")
		}
		return value, nil
	}
	file, ok := s.In.(*os.File)
	if !ok || !term.IsTerminal(int(file.Fd())) {
		return "", clierr.Usage("a terminal is required; use --token-stdin for non-interactive login")
	}
	fmt.Fprintf(s.Err, "%s: ", label)
	data, err := term.ReadPassword(int(file.Fd()))
	fmt.Fprintln(s.Err)
	if err != nil {
		return "", fmt.Errorf("read API token: %w", err)
	}
	value := strings.TrimSpace(string(data))
	if value == "" {
		return "", clierr.Validation("token cannot be empty", "Generate a token in Nexrender team settings.")
	}
	return value, nil
}
