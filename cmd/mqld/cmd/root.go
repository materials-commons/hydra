/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"github.com/apex/log"
	"github.com/feather-lang/feather"
	"github.com/labstack/echo/v4"
	"github.com/materials-commons/hydra/pkg/mcdb"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/mqld/mql"
	"github.com/spf13/cobra"
	"github.com/subosito/gotenv"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mqld",
	Short: "The Material Commons Query Language Daemon",
	Long: `The Material Commons Query Language Daemon is a service that provides a
query language for accessing and manipulating data in the Material Commons
database. It is designed to be used by other applications and services to
perform complex queries and operations on the data in the database.`,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		dotenvFilePath := os.Getenv("MC_DOTENV_PATH")
		if dotenvFilePath == "" {
			log.Fatalf("MC_DOTENV_PATH not set or blank")
		}

		if err := gotenv.Load(dotenvFilePath); err != nil {
			log.Fatalf("Failed loading configuration file %s: %s", dotenvFilePath, err)
		}

		db := mcdb.MustConnectToDB()
		projectStor := stor.NewGormProjectStor(db)
		userStor := stor.NewGormUserStor(db)

		proj, err := projectStor.GetProjectByID(428)
		if err != nil {
			log.Fatalf("Unable to load project: %s", err)
		}

		user, err := userStor.GetUserByEmail("gtarcea@umich.edu")
		if err != nil {
			log.Fatalf("Unable to load user: %s", err)
		}

		e := echo.New()
		e.HideBanner = true
		e.HidePort = true

		interp := feather.New()
		mqlCmd := mql.NewMQLCommands(proj, user, db, interp)
		_ = mqlCmd
		e.POST("/mql", func(c echo.Context) error {
			var req struct {
				Query string `json:"query"`
			}

			if err := c.Bind(&req); err != nil {
				return err
			}

			res := mqlCmd.Run(req.Query)
			//fmt.Println("query results =", res)
			return c.String(200, res)
		})

		if err := e.Start("localhost:8561"); err != nil {
			log.Fatalf("Unable to start web server: %s", err)
		}
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
