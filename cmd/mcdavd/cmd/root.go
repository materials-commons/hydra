package cmd

import (
	"net/http"
	"os"

	"github.com/apex/log"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/materials-commons/hydra/pkg/mcdav"
	"github.com/materials-commons/hydra/pkg/mcdav/fs"
	"github.com/materials-commons/hydra/pkg/mcdav/webapi"
	"github.com/materials-commons/hydra/pkg/mcdb"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/webdav"
	"github.com/spf13/cobra"
	"github.com/subosito/gotenv"
)

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
		users := mcdav.NewUsers(userStor)

		e := echo.New()
		e.HideBanner = true
		e.HidePort = true

		e.Use(middleware.Recover())
		g := e.Group("/api")

		fsRestApiHandler := webapi.NewFSRestAPI(userStor, users)
		g.POST("/reset-user-state", fsRestApiHandler.ResetUserStateHandler)

		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			username, password, _ := r.BasicAuth()

			userEntry, err := users.GetOrCreateValidatedUser(username, password)
			if err != nil {
				w.Header().Set("WWW-Authenticate", `Basic realm="BASIC WebDAV REALM"`)
				w.WriteHeader(401)
				_, _ = w.Write([]byte("401 Unauthorized\n"))
				return
			}

			if userEntry.Server == nil {
				userFS := fs.NewUserFS(&fs.UserFSOpts{
					MCFSRoot:       os.Getenv("MCFS_DIR"),
					ProjectStor:    stor.NewGormProjectStor(db),
					FileStor:       stor.NewGormFileStor(db, os.Getenv("MCFS_DIR")),
					ConversionStor: stor.NewGormConversionStor(db),
					User:           userEntry.User,
				})

				userEntry.Server = &webdav.Handler{
					Prefix:     "/webdav",
					FileSystem: userFS,
					LockSystem: webdav.NewMemLS(),
				}
			}

			w.Header().Set("Timeout", "99999999")
			userEntry.Server.ServeHTTP(w, r)
			return
		})

		go func() {
			if err := e.Start("localhost:8556"); err != nil {
				log.Fatalf("Unable to start API server: %s", err)
			}
		}()

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
