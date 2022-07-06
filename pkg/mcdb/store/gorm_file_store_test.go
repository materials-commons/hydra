package store

import (
	"fmt"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestFindFileByPath(t *testing.T) {
	dsn := "mc:mcpw@tcp(127.0.0.1:3306)/mc?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Errorf("Failed to open db: %s", err)
	}

	fileStore := NewGormFileStore(db, "/")

	f, err := fileStore.FindDirByPath(509, "/D1")
	if err != nil {
		t.Fatalf("FindDirByPath failed: %s", err)
	}
	fmt.Printf("%+v\n", f)
}
