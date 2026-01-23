/*
Copyright © 2026 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/apex/log"
	"github.com/feather-lang/feather"
	"github.com/materials-commons/hydra/pkg/mcdb"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/mctus2/wserv"
	"github.com/materials-commons/hydra/pkg/mqld/mql"
	"github.com/spf13/cobra"
	"github.com/subosito/gotenv"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mchubd",
	Short: "The Material Commons Communications Hub Daemon",
	Long: `The Material Commons Communications Hub Daemon provides a
variety of services to the Materials Commons system. This includes websocket based
file transfers, real time event notifications through SSE, and a repl end point
to control these services and provide scripting capabilities in the web interface.`,
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

		mcfsDir := os.Getenv("MCFS_DIR")

		db := mcdb.MustConnectToDB()
		projectStor := stor.NewGormProjectStor(db)
		userStor := stor.NewGormUserStor(db)

		proj, err := projectStor.GetProjectByID(438)
		if err != nil {
			log.Fatalf("Unable to load project: %s", err)
		}

		user, err := userStor.GetUserByEmail("gtarcea@umich.edu")
		if err != nil {
			log.Fatalf("Unable to load user: %s", err)
		}

		hub := wserv.NewHub(db, mcfsDir)
		go hub.Run()

		interp := feather.New()
		mqlCmd := mql.NewMQLCommands(proj, user, db, interp, hub)

		hubMux := http.NewServeMux()

		hubMux.HandleFunc("/mql", func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Query string `json:"query"`
			}

			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}

			res := mqlCmd.Run(req.Query, w)
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, res)
		})

		hubMux.HandleFunc("/send-command", hub.HandleSendCommand)
		hubMux.HandleFunc("/list-clients", hub.HandleListClients)
		hubMux.HandleFunc("/list-clients-for-user/{id}", hub.HandleListClientsForUser)
		hubMux.HandleFunc("/submit-test-upload/{client_id}", hub.HandleSubmitTestUpload)
		hubMux.HandleFunc("/sse", hub.HandleSSE)
		hubMux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			hub.ServeWS(w, r)
		})

		fmt.Printf("Listening on port 8558\n")
		err = http.ListenAndServe(":8558", hubMux)
		if err != nil {
			log.Fatalf("unable to listen: %s", err)
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
