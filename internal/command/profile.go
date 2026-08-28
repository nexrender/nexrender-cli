package command

import (
	"fmt"
	"io"
	"sort"

	"github.com/nexrender/nexrender-cli/internal/clierr"
	"github.com/nexrender/nexrender-cli/internal/config"
	"github.com/spf13/cobra"
)

type ProfileView struct {
	Name   string `json:"name"`
	Server string `json:"server"`
	Active bool   `json:"active"`
}

func newProfileCommand(state *State) *cobra.Command {
	cmd := &cobra.Command{Use: "profile", Short: "Manage team and server profiles"}
	cmd.AddCommand(&cobra.Command{
		Use: "list", Short: "List configured profiles", Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			cfg, err := state.Config.Load()
			if err != nil {
				return err
			}
			views := make([]ProfileView, 0, len(cfg.Profiles))
			for name, profile := range cfg.Profiles {
				views = append(views, ProfileView{Name: name, Server: profile.Server, Active: name == cfg.ActiveProfile})
			}
			sort.Slice(views, func(i, j int) bool { return views[i].Name < views[j].Name })
			return state.Printer(state.Out).Success(views, fmt.Sprintf("%d profiles", len(views)), func(w io.Writer) error {
				for _, view := range views {
					marker := " "
					if view.Active {
						marker = "*"
					}
					fmt.Fprintf(w, "%s %-20s %s\n", marker, view.Name, view.Server)
				}
				return nil
			})
		},
	})
	var server string
	add := &cobra.Command{
		Use: "add <name>", Short: "Add or update a profile", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if err := config.ValidateProfileName(args[0]); err != nil {
				return clierr.Validation(err.Error(), "Use letters, numbers, hyphens, or underscores.")
			}
			cfg, err := state.Config.Load()
			if err != nil {
				return err
			}
			server = config.NormalizeServer(server)
			if server == "" {
				server = config.DefaultServer
			}
			cfg.Profiles[args[0]] = config.Profile{Server: server}
			if err := state.Config.Save(cfg); err != nil {
				return err
			}
			view := ProfileView{Name: args[0], Server: server, Active: args[0] == cfg.ActiveProfile}
			return state.Printer(state.Out).Success(view, "Saved profile "+args[0], nil)
		},
	}
	add.Flags().StringVar(&server, "server", config.DefaultServer, "API base URL for this profile")
	cmd.AddCommand(add)
	cmd.AddCommand(&cobra.Command{
		Use: "use <name>", Short: "Set the active profile", Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			cfg, err := state.Config.Load()
			if err != nil {
				return err
			}
			if _, ok := cfg.Profiles[args[0]]; !ok {
				return clierr.New("profile_not_found", "profile not found: "+args[0], clierr.ExitNotFound)
			}
			cfg.ActiveProfile = args[0]
			if err := state.Config.Save(cfg); err != nil {
				return err
			}
			return state.Printer(state.Out).Success(map[string]string{"activeProfile": args[0]}, "Using profile "+args[0], nil)
		},
	})
	return cmd
}
