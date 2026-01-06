package cmd

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/apex/log"
	"github.com/materials-commons/hydra/pkg/mcdb"
	"github.com/materials-commons/hydra/pkg/mctus2"
	"github.com/materials-commons/hydra/pkg/mctus2/wserv"
	"github.com/spf13/cobra"
	"github.com/subosito/gotenv"
	"github.com/tus/tusd/v2/pkg/filelocker"
	tusd "github.com/tus/tusd/v2/pkg/handler"
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

		mcfsDir := os.Getenv("MCFS_DIR")

		tusChunksDir := filepath.Join(mcfsDir, "__tus", "chunks")
		fmt.Printf("tusChunksDir: %s\n", tusChunksDir)
		if err := os.MkdirAll(tusChunksDir, 0755); err != nil {
			log.Fatalf("Unable to create directory %s: %s", tusChunksDir, err)
		}

		progressCache := mctus2.NewUploadProgressCache()
		app := mctus2.NewApp(mctus2.LocalFileStore{Path: tusChunksDir}, db, mcfsDir, progressCache)

		tusLockDir := filepath.Join(mcfsDir, "__tus", "locks")
		fmt.Printf("tusLockDir: %s\n", tusLockDir)
		if err := os.MkdirAll(tusLockDir, 0755); err != nil {
			log.Fatalf("Unable to create directory %s: %s", tusLockDir, err)
		}
		locker := filelocker.New(tusLockDir)

		composer := tusd.NewStoreComposer()
		app.TusFileStore.UseIn(composer)
		locker.UseIn(composer)

		config := tusd.Config{
			BasePath:                "/files/",
			StoreComposer:           composer,
			NotifyCompleteUploads:   true,
			RespectForwardedHeaders: true,
			DisableDownload:         true,
			NotifyUploadProgress:    true,
		}

		handler, err := tusd.NewHandler(config)
		if err != nil {
			log.Fatalf("unable to create handler: %s", err)
		}

		app.TusHandler = handler

		go app.OnFileComplete()
		go app.OnUploadProgress()

		progressController := mctus2.NewProgressController(progressCache)

		http.Handle("/files/", app.AccessMiddleware(http.StripPrefix("/files/", handler)))
		http.HandleFunc("/tus-upload-progress", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
				return
			}
			progressController.GetUploadProgressHandler(w, r)
		})

		hub := wserv.NewHub(db, mcfsDir)
		go hub.Run()

		http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
			hub.ServeWS(hub, w, r)
		})

		hubMux := http.NewServeMux()
		hubMux.HandleFunc("/send-command", hub.HandleSendCommand)
		hubMux.HandleFunc("/list-clients", hub.HandleListClients)
		hubMux.HandleFunc("/list-clients-for-user/{id}", hub.HandleListClientsForUser)
		hubMux.HandleFunc("/submit-test-upload/{client_id}", hub.HandleSubmitTestUpload)
		hubMux.HandleFunc("/sse", hub.HandleSSE)

		// Start the hub REST API server on port 8559
		go func() {
			fmt.Printf("Hub REST API listening on port 8559\n")
			if err := http.ListenAndServe(":8559", hubMux); err != nil {
				log.Fatalf("unable to start hub REST API server: %s", err)
			}
		}()

		fmt.Printf("Listening on port 8558\n")
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
