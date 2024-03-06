/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/apex/log"
	"github.com/materials-commons/hydra/pkg/mcdav"
	"github.com/materials-commons/hydra/pkg/mcdb"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/webdav"
	"github.com/spf13/cobra"
	"github.com/subosito/gotenv"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mcdavd",
	Short: "Run a WebDav server for Materials Commons",
	Long:  `Run a WebDav server for Materials Commons`,
	Run: func(cmd *cobra.Command, args []string) {
		dotenvFilePath := os.Getenv("MC_DOTENV_PATH")
		if dotenvFilePath == "" {
			log.Fatalf("MC_DOTENV_PATH not set or blank")
		}

		if err := gotenv.Load(dotenvFilePath); err != nil {
			log.Fatalf("Failed loading configuration file %s: %s", dotenvFilePath, err)
		}

		db := mcdb.MustConnectToDB()
		userStor := stor.NewGormUserStor(db)
		user, err := userStor.GetUserByEmail("gtarcea@umich.edu")
		if err != nil {
			log.Fatalf("Failed getting user gtarcea@umich.edu: %s", err)
		}
		userFS := mcdav.NewUserFS(&mcdav.UserFSOpts{
			MCFSRoot:    os.Getenv("MCFS_DIR"),
			ProjectStor: stor.NewGormProjectStor(db),
			FileStor:    stor.NewGormFileStor(db, os.Getenv("MCFS_DIR")),
			User:        user,
		})
		_ = userFS
		webdavSrv := &webdav.Handler{
			Prefix: "/webdav",
			//FileSystem: webdav.Dir("/home/gtarcea/Downloads"),
			FileSystem: userFS,
			LockSystem: webdav.NewMemLS(),
			//Logger: func(r *http.Request, err error) {
			//	if err != nil {
			//		fmt.Printf("WebDAV %s: %s, ERROR: %s\n", r.Method, r.URL, err)
			//	} else {
			//		//b, _ := io.ReadAll(r.Body)
			//		fmt.Printf("WebDAV %s: %s\n", r.Method, r.URL)
			//	}
			//},
		}

		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			r.Body = io.NopCloser(bytes.NewBuffer(body))
			fmt.Printf("In HandleFunc, %s body = '%s'\n", r.Method, string(body))
			username, password, _ := r.BasicAuth()
			if username == "webdav@umich.edu" && password == "abc123" {
				w.Header().Set("Timeout", "99999999")
				webdavSrv.ServeHTTP(w, r)
				return
			}

			w.Header().Set("WWW-Authenticate", `Basic realm="BASIC WebDAV REALM"`)
			w.WriteHeader(401)
			_, _ = w.Write([]byte("401 Unauthorized\n"))
		})

		_ = http.ListenAndServe(":8555", nil)
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
