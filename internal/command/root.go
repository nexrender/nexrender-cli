package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/nexrender/nexrender-cli/internal/buildinfo"
	"github.com/nexrender/nexrender-cli/internal/clierr"
	openapispec "github.com/nexrender/nexrender-cli/openapi"
	"github.com/spf13/cobra"
)

type CommandInfo struct {
	Command string `json:"command"`
	Use     string `json:"use"`
	Summary string `json:"summary"`
}

type SchemaResult struct {
	OperationID string                     `json:"operationId"`
	Method      string                     `json:"method"`
	Path        string                     `json:"path"`
	Operation   map[string]any             `json:"operation"`
	Schemas     map[string]json.RawMessage `json:"schemas,omitempty"`
}

func NewRoot(state *State) *cobra.Command {
	root := &cobra.Command{
		Use:           "nexrender",
		Short:         "Operate Nexrender Cloud from the terminal",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetIn(state.In)
	root.SetOut(state.Out)
	root.SetErr(state.Err)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return clierr.Usage(err.Error())
	})
	flags := root.PersistentFlags()
	flags.BoolVar(&state.JSON, "json", false, "print a structured JSON envelope")
	flags.BoolVar(&state.Quiet, "quiet", false, "omit the envelope and print only result data")
	flags.StringVar(&state.JQ, "jq", "", "filter structured output with a jq expression")
	flags.StringVar(&state.Profile, "profile", "", "use a configured Nexrender profile")
	flags.StringVar(&state.Server, "server", "", "override the Nexrender API base URL")
	flags.DurationVar(&state.Timeout, "timeout", state.Timeout, "HTTP request timeout")

	root.AddCommand(newVersionCommand(state))
	root.AddCommand(newCommandsCommand(state, root))
	root.AddCommand(newSchemaCommand(state))
	root.AddCommand(newCompletionCommand(root))
	root.AddCommand(newAuthCommand(state))
	root.AddCommand(newProfileCommand(state))
	root.AddCommand(newJobCommand(state))
	root.AddCommand(newTemplateCommand(state))
	root.AddCommand(newBatchCommand(state))
	root.AddCommand(newFontCommand(state))
	root.AddCommand(newSecretCommand(state))
	root.AddCommand(newSkillCommand(state))
	root.AddCommand(newSetupCommand(state))
	root.AddCommand(newDoctorCommand(state))
	return root
}

func Execute(ctx context.Context, state *State, args []string) int {
	root := NewRoot(state)
	root.SetArgs(args)
	err := root.ExecuteContext(ctx)
	if err == nil {
		return clierr.ExitOK
	}
	var structured *clierr.Error
	if !errors.As(err, &structured) && looksLikeUsageError(err.Error()) {
		err = clierr.Usage(err.Error())
	}
	writer := state.Err
	if state.Structured() {
		writer = state.Out
	}
	if printErr := state.Printer(writer).Failure(err); printErr != nil {
		fmt.Fprintf(state.Err, "Error: %v\n", printErr)
	}
	return clierr.Code(err)
}

func looksLikeUsageError(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	return strings.HasPrefix(message, "unknown command") ||
		strings.HasPrefix(message, "accepts ") ||
		strings.HasPrefix(message, "requires ") ||
		strings.HasPrefix(message, "invalid argument")
}

func newVersionCommand(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version and build information",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			info := buildinfo.Current()
			return state.Printer(state.Out).Success(info, "nexrender "+info.Version, func(w io.Writer) error {
				_, err := fmt.Fprintf(w, "nexrender %s\ncommit: %s\nbuilt: %s\n", info.Version, info.Commit, info.Date)
				return err
			})
		},
	}
}

func newCommandsCommand(state *State, root *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:   "commands",
		Short: "List the executable command catalog",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			var commands []CommandInfo
			var visit func(*cobra.Command)
			visit = func(cmd *cobra.Command) {
				if cmd != root && cmd.IsAvailableCommand() && !cmd.Hidden && cmd.Name() != "help" {
					commands = append(commands, CommandInfo{Command: cmd.CommandPath(), Use: cmd.UseLine(), Summary: cmd.Short})
				}
				for _, child := range cmd.Commands() {
					visit(child)
				}
			}
			visit(root)
			sort.Slice(commands, func(i, j int) bool { return commands[i].Command < commands[j].Command })
			return state.Printer(state.Out).Success(commands, fmt.Sprintf("%d commands", len(commands)), func(w io.Writer) error {
				for _, item := range commands {
					if _, err := fmt.Fprintf(w, "%-38s %s\n", item.Command, item.Summary); err != nil {
						return err
					}
				}
				return nil
			})
		},
	}
}

func newSchemaCommand(state *State) *cobra.Command {
	return &cobra.Command{
		Use:   "schema <operation-id>",
		Short: "Print an OpenAPI operation and its referenced schemas",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			result, err := findOperation(openapispec.Document, args[0])
			if err != nil {
				return err
			}
			return state.Printer(state.Out).Success(result, result.Method+" "+result.Path, nil)
		},
	}
}

func findOperation(document []byte, operationID string) (SchemaResult, error) {
	var spec struct {
		Paths      map[string]map[string]json.RawMessage `json:"paths"`
		Components struct {
			Schemas map[string]json.RawMessage `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(document, &spec); err != nil {
		return SchemaResult{}, fmt.Errorf("parse embedded OpenAPI document: %w", err)
	}
	for path, item := range spec.Paths {
		for method, raw := range item {
			if !isHTTPMethod(method) {
				continue
			}
			var operation map[string]any
			if err := json.Unmarshal(raw, &operation); err != nil {
				continue
			}
			if operation["operationId"] != operationID {
				continue
			}
			result := SchemaResult{OperationID: operationID, Method: strings.ToUpper(method), Path: path, Operation: operation, Schemas: map[string]json.RawMessage{}}
			refs := map[string]bool{}
			collectRefs(operation, refs)
			for len(refs) > len(result.Schemas) {
				for name := range refs {
					if _, seen := result.Schemas[name]; seen {
						continue
					}
					rawSchema, ok := spec.Components.Schemas[name]
					if !ok {
						continue
					}
					result.Schemas[name] = rawSchema
					var decoded any
					if json.Unmarshal(rawSchema, &decoded) == nil {
						collectRefs(decoded, refs)
					}
				}
			}
			return result, nil
		}
	}
	return SchemaResult{}, clierr.New("operation_not_found", "OpenAPI operation not found: "+operationID, clierr.ExitNotFound)
}

func collectRefs(value any, refs map[string]bool) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			if key == "$ref" {
				if ref, ok := child.(string); ok {
					const prefix = "#/components/schemas/"
					if strings.HasPrefix(ref, prefix) {
						refs[strings.TrimPrefix(ref, prefix)] = true
					}
				}
			}
			collectRefs(child, refs)
		}
	case []any:
		for _, child := range typed {
			collectRefs(child, refs)
		}
	}
}

func isHTTPMethod(method string) bool {
	switch strings.ToLower(method) {
	case "get", "post", "put", "patch", "delete", "head", "options":
		return true
	default:
		return false
	}
}

func newCompletionCommand(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{Use: "completion", Short: "Generate shell completion scripts"}
	cmd.AddCommand(&cobra.Command{Use: "bash", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return root.GenBashCompletion(cmd.OutOrStdout()) }})
	cmd.AddCommand(&cobra.Command{Use: "zsh", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return root.GenZshCompletion(cmd.OutOrStdout()) }})
	cmd.AddCommand(&cobra.Command{Use: "fish", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return root.GenFishCompletion(cmd.OutOrStdout(), true) }})
	cmd.AddCommand(&cobra.Command{Use: "powershell", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error { return root.GenPowerShellCompletion(cmd.OutOrStdout()) }})
	return cmd
}
