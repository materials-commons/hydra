package cmd

import (
	"net/http"
	"os"

	"github.com/apex/log"
	"github.com/materials-commons/hydra/pkg/mcdb"
	"github.com/materials-commons/hydra/pkg/mctus/handler"
	"github.com/materials-commons/hydra/pkg/mctus/hook"
	"github.com/spf13/cobra"
	"github.com/subosito/gotenv"
	"github.com/tus/tusd/v2/pkg/filelocker"
	tusd "github.com/tus/tusd/v2/pkg/handler"
	"github.com/tus/tusd/v2/pkg/hooks"
)

var rootCmd = &cobra.Command{
	Use:   "mctusd",
	Short: "Run a tus file upload server",
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
		filestor := handler.NewMCFileStore(db, os.Getenv("MCFS_DIR"))
		locker := filelocker.New("/tmp") // TODO - This should be passed in
		composer := tusd.NewStoreComposer()
		filestor.UseIn(composer)
		locker.UseIn(composer)

		hook := hook.NewMCHookHandler(db)

		handler, err := hooks.NewHandlerWithHooks(
			&tusd.Config{
				BasePath:              "/files/",
				StoreComposer:         composer,
				NotifyCompleteUploads: false,
			},
			hook,
			//&plugin.PluginHook{
			//	Path: "/usr/local/bin/tus/mctus_hook",
			//},
			[]hooks.HookType{hooks.HookPreCreate})

		if err != nil {
			log.Fatalf("unable to create handler: %s", err)
		}

		//go func() {
		//	for {
		//		event := <-handler.CompleteUploads
		//		log.Printf("Upload %s finished\n", event.Upload.ID)
		//	}
		//}()

		http.Handle("/files/", http.StripPrefix("/files/", handler))
		http.Handle("/files", http.StripPrefix("/files", handler))
		err = http.ListenAndServe(":8558", nil)
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
