package mcdb

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func MakeDSNFromEnv() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		os.Getenv("DB_USERNAME"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_DATABASE"))
}

const SqliteInMemoryDSN = "file::memory:?cache=shared"

const maxDBRetries = 5

var dbInstance *gorm.DB

// MustConnectToDB will attempt to connect to the database maxDBRetries times. If it isn't successful
// after that number of retries then it will call log.Fatalf(), which will cause the server to exit.
// Between retry attempts it will sleep for 3 seconds.
func MustConnectToDB() *gorm.DB {
	var (
		err error
	)

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	}

	retryCount := 1
	for {
		dbInstance, err = gorm.Open(mysql.Open(MakeDSNFromEnv()), gormConfig)
		switch {
		case err == nil:
			// Connected to db, yay!
			return dbInstance
		case retryCount >= maxDBRetries:
			// Retry limit exceeded :-(
			log.Fatalf("Failed to open db (%s): %s", MakeDSNFromEnv(), err)
		default:
			// Couldn't connect, so increment count, then wait a bit before trying again.
			retryCount++
			time.Sleep(3 * time.Second)
		}
	}
}

func RunMigrations(db *gorm.DB) error {
	return db.AutoMigrate(&mcmodel.File{}, &mcmodel.Project{}, &mcmodel.User{}, &mcmodel.Conversion{},
		&mcmodel.TransferRequest{}, &mcmodel.TransferRequestFile{}, &mcmodel.GlobusTransfer{}, &mcmodel.Team{})
}

func GetDBInstance() *gorm.DB {
	return dbInstance
}
