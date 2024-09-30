/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"

	"github.com/apex/log"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/materials-commons/hydra/pkg/config"
	"github.com/materials-commons/hydra/pkg/mcdb"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mcapid",
	Short: "Run the mcapid API server",
	Long:  ``,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	Run: func(cmd *cobra.Command, args []string) {
		e := echo.New()
		e.HideBanner = true
		e.HidePort = true
		e.Use(middleware.Recover())
		db := mcdb.MustConnectToDB()
		c := config.MustLoadFromMCDotenv()
		mcfsDir := c.MustGetKey("MCFS_DIR")
		log.Infof("MCFS Dir: %s", mcfsDir)
		fileStor := stor.NewGormFileStor(db, mcfsDir)

		setupRoutes(e, RouteOpts{
			fileStor: fileStor,
		})

		if err := e.Start(":" + c.GetKeyWithDefault("MCAPID_PORT", "1352")); err != nil {
			log.Fatalf("Unable to start server: %v", err)
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
