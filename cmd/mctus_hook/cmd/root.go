package cmd

import (
	"os"

	"github.com/apex/log"
	hplugin "github.com/hashicorp/go-plugin"
	"github.com/materials-commons/hydra/pkg/mcdb"
	"github.com/materials-commons/hydra/pkg/mctus/hook"
	"github.com/spf13/cobra"
	"github.com/subosito/gotenv"
	"github.com/tus/tusd/v2/pkg/hooks/plugin"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mctus_hook",
	Short: "Runs a plugin hook for TUS",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		dotenvFilePath := os.Getenv("MC_DOTENV_PATH")
		if dotenvFilePath == "" {
			log.Fatalf("MC_DOTENV_PATH not set or blank")
		}

		if err := gotenv.Load(dotenvFilePath); err != nil {
			log.Fatalf("Failed loading configuration file %s: %s", dotenvFilePath, err)
		}

		db := mcdb.MustConnectToDB()
		hook := hook.NewMCHookHandler(db)

		var handshakeConfig = hplugin.HandshakeConfig{
			ProtocolVersion:  1,
			MagicCookieKey:   "tusd",
			MagicCookieValue: "yes",
		}

		var pluginMap = map[string]hplugin.Plugin{
			"hookHandler": &plugin.HookHandlerPlugin{Impl: hook},
		}

		hplugin.Serve(&hplugin.ServeConfig{
			HandshakeConfig: handshakeConfig,
			Plugins:         pluginMap,
		})
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.hydra.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
