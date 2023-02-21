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
	"github.com/apex/log"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/materials-commons/hydra/pkg/mcbridgefs"
	"github.com/materials-commons/hydra/pkg/mcdb"
	"github.com/spf13/cobra"
	"github.com/subosito/gotenv"
	"os"
)

var (
	cfgFile string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mcbridgefsd",
	Short: "Server for launching bridges",
	Long:  `The mcbridgefsd is responsible for launching new mcbridgefs and monitoring if they exit prematurely.`,
	Run: func(cmd *cobra.Command, args []string) {
		db := mcdb.MustConnectToDB()

		e := echo.New()
		e.HideBanner = true
		e.HidePort = true
		e.Use(middleware.Recover())

		g := e.Group("/api")

		s := mcbridgefs.NewServer(g, db)

		if err := s.Init(); err != nil {
			log.Fatalf("Unable to initalize mcbridgefsd: %s", err)
		}

		if err := e.Start("localhost:1323"); err != nil {
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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.mcbridefsd.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	dotenvFilePath := os.Getenv("MC_DOTENV_PATH")
	if dotenvFilePath == "" {
		log.Fatalf("MC_DOTENV_PATH not set")
	}

	if err := gotenv.Load(dotenvFilePath); err != nil {
		log.Fatalf("Loading %s failed: %s", dotenvFilePath, err)
	}
}
