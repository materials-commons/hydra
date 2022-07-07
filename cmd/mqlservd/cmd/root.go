// Copyright © 2021 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"fmt"
	"os"

	"github.com/apex/log"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/materials-commons/hydra/pkg/mcdb"
	"github.com/materials-commons/hydra/internal/mql/web/api"
	"github.com/spf13/cobra"
	"github.com/subosito/gotenv"
)

var (
	cfgFile    string
	dotenvPath string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mqlservd",
	Short: "MQL Query Execution Server",
	Long: `The mqlservd server implements a REST API for the (M)aterials (Q)uery (L)anguage (MQL). It allows users
to query their materials data and find matching samples and processes.`,
	Run: func(cmd *cobra.Command, args []string) {
		e := echo.New()
		e.HideBanner = true
		e.HidePort = true
		e.Use(middleware.Recover())

		db := mcdb.MustConnectToDB()

		api.Init(db)

		g := e.Group("/api")
		g.POST("/load-project", api.LoadProjectController)
		g.POST("/reload-project", api.ReloadProjectController)
		g.POST("/execute-query", api.ExecuteQueryController)

		if err := e.Start("localhost:1324"); err != nil {
			log.Fatalf("Unable to start web server: %s", err)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.mqlservd.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if err := gotenv.Load(MustGetDotenvPath()); err != nil {
		log.Fatalf("Loading dotenv file path %s failed: %s", dotenvPath, err)
	}
}

func MustGetDotenvPath() string {
	if dotenvPath != "" {
		return dotenvPath
	}

	dotenvPath = os.Getenv("MC_DOTENV_PATH")
	if dotenvPath == "" {
		log.Fatal("MC_DOTENV_PATH not set")
	}

	return dotenvPath
}
