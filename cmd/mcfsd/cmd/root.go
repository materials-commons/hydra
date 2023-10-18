/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"os"
	"time"

	"github.com/apex/log"
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
	"github.com/materials-commons/hydra/pkg/mcdb"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"github.com/materials-commons/hydra/pkg/mcfs/fs/mcfs"
	"github.com/materials-commons/hydra/pkg/mcfs/fs/mcfs/mcpath"
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
		if len(args) != 1 {
			log.Fatalf("No path specified for mount")
		}

		readConfig()

		log.SetLevel(log.DebugLevel)

		db := mcdb.MustConnectToDB()
		knownFilesTracker := mcfs.NewKnownFilesTracker()
		stors := stor.NewGormStors(db, mcfsDir)
		pathParser := mcpath.NewTransferPathParser(stors)
		mcapi := mcfs.NewLocalMCFSApi(stors, knownFilesTracker, pathParser, mcfsDir)
		handleFactory := mcfs.NewMCFileHandlerFactory(mcapi, knownFilesTracker, pathParser, time.Hour*12)
		newFileHandleFunc := func(fd, flags int, path string, file *mcmodel.File) fs.FileHandle {
			return handleFactory.NewFileHandle(fd, flags, path, file)
		}

		mcfs, err := mcfs.CreateFS(mcfsDir, mcapi, newFileHandleFunc)
		if err != nil {
			log.Fatalf("Unable to create filesystem: %s", err)
		}

		rawfs := fs.NewNodeFS(mcfs, &fs.Options{})
		fuseServer, err := fuse.NewServer(rawfs, args[0], &fuse.MountOptions{Name: "mcfs"})
		if err != nil {
			log.Fatalf("Unable to fuse server: %s", err)
		}

		go fuseServer.Serve()
		if err := fuseServer.WaitMount(); err != nil {
			log.Fatalf("Mount failed: %s", err)
		}

		log.Info("Past WaitMount")
		fuseServer.Wait()
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
