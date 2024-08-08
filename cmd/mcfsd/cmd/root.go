package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/apex/log"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/labstack/echo/v4"
	"github.com/materials-commons/hydra/pkg/config"
	"github.com/materials-commons/hydra/pkg/globus"
	"github.com/materials-commons/hydra/pkg/mcdb"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/mcfs/fs/mcfs/fsstate"
	"github.com/spf13/cobra"
	"github.com/subosito/gotenv"
)

var mcfsDir string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "mcfsd",
	Short: "Daemon for the mcfs file system",
	Long:  `Daemon for the mcfs file system`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		if err := Run(ctx, args, config.GetConfig()); err != nil {
			log.Fatalf("mcfsd: %s", err)
		}
	},
}

func Run(c context.Context, args []string, config config.Configer) error {
	readConfig()

	mountPath := os.Getenv("MCFSD_MOUNT_DIR")

	if len(args) == 1 {
		mountPath = args[0]
	}

	if mountPath == "" {
		return fmt.Errorf("no path specified for mount")
	}

	db := mcdb.MustConnectToDB()
	stors := stor.NewGormStors(db, mcfsDir)
	fsState := fsstate.NewFSState(fsstate.NewTransferStateTracker(),
		fsstate.NewTransferRequestCache(stors.TransferRequestStor),
		fsstate.NewActivityTracker())
	globusClient, _ := globus.CreateConfidentialClient("", "")

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	setupRoutes(RouteDependencies{
		e:            e,
		config:       config,
		stors:        stors,
		fsState:      fsState,
		globusClient: globusClient,
	})

	go func() {
		if err := e.Start("localhost:1350"); err != nil {
			log.Fatalf("Unable to start web server: %s", err)
		}
	}()

	fuseServer, err := createFS(FSDependencies{
		stors:     stors,
		fsState:   fsState,
		mcfsDir:   config.GetKey("MCFS_DIR"),
		mountPath: mountPath,
	})

	if err != nil {
		return err
	}

	go fuseServer.Serve()
	if err := fuseServer.WaitMount(); err != nil {
		log.Fatalf("Mount failed: %s", err)
	}

	go unmountOnSignal(fuseServer, mountPath)

	fuseServer.Wait()

	return nil
}

func unmountOnSignal(server *fuse.Server, path string) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	sig := <-c
	log.Infof("Got %s signal, unmounting %q...", sig, path)
	if err := server.Unmount(); err != nil {
		log.Errorf("Unmount failed: %s, trying umount...", err)
		cmd := exec.Command("/usr/bin/umount", path)
		if err := cmd.Run(); err != nil {
			log.Errorf("/usr/bin/umount failed: %s", err)
		}
	}

	os.Exit(0)
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func readConfig() {
	incompleteConfiguration := false

	dotenvFilePath := os.Getenv("MC_DOTENV_PATH")
	if dotenvFilePath == "" {
		log.Fatalf("MC_DOTENV_PATH not set or blank")
	}

	if err := gotenv.Load(dotenvFilePath); err != nil {
		log.Fatalf("Failed loading configuration file %s: %s", dotenvFilePath, err)
	}

	mcfsDir = os.Getenv("MCFS_DIR")
	if mcfsDir == "" {
		log.Errorf("MCFS_DIR is not set or blank")
		incompleteConfiguration = true
	}

	if incompleteConfiguration {
		log.Fatalf("One or more required variables not configured, exiting.")
	}

	log.Infof("MCFS Dir: %s", mcfsDir)
}
