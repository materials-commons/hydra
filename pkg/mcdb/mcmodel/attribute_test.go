package mcmodel

import (
	"fmt"
	"testing"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestRetrieveAttributeValues(t *testing.T) {
	dsn := "mc:mcpw@tcp(127.0.0.1:3306)/mc?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Errorf("Failed to open db: %s", err)
	}

	var allProcessAttributes []*Attribute

	err = db.Debug().Preload("AttributeValues").Where("attributable_type = ?", "App\\Models\\Activity").
		Where("attributable_id in (select id from activities where project_id = ?)", 77).Limit(10).
		Find(&allProcessAttributes).Error
	if err != nil {
		t.Fatalf("Query to get attributes failed: %s", err)
	}

	for i := range allProcessAttributes {
		fmt.Printf("%+v\n", allProcessAttributes[i])
	}

	for _, attr := range allProcessAttributes {
		if err := attr.LoadValues(); err != nil {
			t.Fatalf("Unable to load values: %s", err)
		}
		fmt.Printf("%+v\n", attr.AttributeValues)
	}
}
