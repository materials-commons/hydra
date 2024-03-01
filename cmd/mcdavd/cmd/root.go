/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/net/webdav"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mcdavd",
	Short: "Run a WebDav server for Materials Commons",
	Long:  `Run a WebDav server for Materials Commons`,
	Run: func(cmd *cobra.Command, args []string) {
		webdavSrv := &webdav.Handler{
			Prefix:     "/webdav",
			FileSystem: webdav.Dir("/home/gtarcea/Downloads"),
			LockSystem: webdav.NewMemLS(),
			Logger: func(r *http.Request, err error) {
				if err != nil {
					fmt.Printf("WebDAV %s: %s, ERROR: %s\n", r.Method, r.URL, err)
				} else {
					fmt.Printf("WebDAV %s: %s \n", r.Method, r.URL)
				}
			},
		}

		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			fmt.Println("In HandleFunc")
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
