/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"net/http"
	"os"
	"sync"

	"github.com/apex/log"
	"github.com/materials-commons/hydra/pkg/mcdav"
	"github.com/materials-commons/hydra/pkg/mcdb"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/webdav"
	"github.com/spf13/cobra"
	"github.com/subosito/gotenv"
	"golang.org/x/crypto/bcrypt"
)

type UserEntry struct {
	webdavSrv *webdav.Handler
	password  string
}

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

		var userEntryByEmail sync.Map

		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			username, password, _ := r.BasicAuth()

			var userEntry *UserEntry

			entry, ok := userEntryByEmail.Load(username)
			if ok {
				userEntry = entry.(*UserEntry)
				if password == userEntry.password {
					// same password so we don't need to check it, so just serve
					w.Header().Set("Timeout", "99999999")
					userEntry.webdavSrv.ServeHTTP(w, r)
					return
				} else {
					// Wrong password
					w.Header().Set("WWW-Authenticate", `Basic realm="BASIC WebDAV REALM"`)
					w.WriteHeader(401)
					_, _ = w.Write([]byte("401 Unauthorized\n"))
				}
			} else {
				// Didn't find the entry, so lets get everything setup.
				user, err := userStor.GetUserByEmail(username)
				if err != nil {
					log.Errorf("Failed getting user %s: %s", username, err)
					w.Header().Set("WWW-Authenticate", `Basic realm="BASIC WebDAV REALM"`)
					w.WriteHeader(401)
					_, _ = w.Write([]byte("401 Unauthorized\n"))
					return
				}

				// make sure that the password is correct
				if err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
					w.Header().Set("WWW-Authenticate", `Basic realm="BASIC WebDAV REALM"`)
					w.WriteHeader(401)
					_, _ = w.Write([]byte("401 Unauthorized\n"))
					return
				}

				userFS := mcdav.NewUserFS(&mcdav.UserFSOpts{
					MCFSRoot:       os.Getenv("MCFS_DIR"),
					ProjectStor:    stor.NewGormProjectStor(db),
					FileStor:       stor.NewGormFileStor(db, os.Getenv("MCFS_DIR")),
					ConversionStor: stor.NewGormConversionStor(db),
					User:           user,
				})

				webdavSrv := &webdav.Handler{
					Prefix:     "/webdav",
					FileSystem: userFS,
					LockSystem: webdav.NewMemLS(),
				}

				userEntry := &UserEntry{
					webdavSrv: webdavSrv,
					password:  password,
				}

				userEntryByEmail.Store(username, userEntry)

				w.Header().Set("Timeout", "99999999")
				userEntry.webdavSrv.ServeHTTP(w, r)
				return
			}
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
