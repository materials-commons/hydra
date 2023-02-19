package mqldb

import (
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestLoadingFromSQLDB(t *testing.T) {
	dsn := "mc:mcpw@tcp(127.0.0.1:3306)/mc?charset=utf8mb4&parseTime=True&loc=Local"
	mysqldb, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Errorf("Failed to open db: %s", err)
	}

	db := NewDB(77, mysqldb)
	if err := db.Load(); err != nil {
		t.Fatalf("Failed loading database: %s", err)
	}
}
