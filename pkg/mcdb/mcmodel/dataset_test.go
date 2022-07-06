package mcmodel

import (
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestQueryDataset(t *testing.T) {
	dsn := os.Getenv("MC_DB_DSN")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Errorf("Failed to open db: %s", err)
	}

	var ds Dataset
	result := db.Find(&ds, 1)

	require.NoError(t, result.Error, "Query returned error: %s", result.Error)
	fmt.Printf("%+v\n", ds)
	fs, err := ds.GetFileSelection()
	require.NoError(t, err, "GetFileSelection returned error: %s", err)
	fmt.Printf("%+v\n", fs)
}

func TestBuildingEntitiesQuery(t *testing.T) {
	dsn := "mc:mcpw@tcp(127.0.0.1:3306)/mc?charset=utf8mb4&parseTime=True&loc=Local"
	//dsn := os.Getenv("MC_DB_DSN")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Errorf("Failed to open db: %s", err)
	}

	d := Dataset{ID: 1}

	experimentIdsSubquery := db.Table("item2entity_selection").
		Select("experiment_id").
		Where("item_id = ?", d.ID).
		Where("item_type = ?", "App\\Models\\Dataset")

	entityIdsFromExperimentSubquery := db.Table("experiment2entity").
		Select("entity_id").
		Where("experiment_id in (?)", experimentIdsSubquery)

	entityNamesFromExperimentSubquery := db.Table("item2entity_selection").
		Select("entity_name").
		Where("item_id = ?", d.ID).
		Where("item_type = ?", "App\\Models\\Dataset").
		Where("experiment_id in (?)", experimentIdsSubquery)

	entityIdSubquery := db.Table("item2entity_selection").
		Select("entity_id").
		Where("item_id = ?", d.ID).
		Where("item_type = ?", "App\\Models\\Dataset")

	var entities []Entity
	stmt := db.Preload("Files.Directory").
		Where("id in (?)", entityIdsFromExperimentSubquery).
		Where("name in (?)", entityNamesFromExperimentSubquery).
		Or("id in (?)", entityIdSubquery).
		Find(&entities).Statement
	fmt.Println(stmt.SQL.String())
}

func TestDataset_GetEntitiesFromTemplate(t *testing.T) {
	dsn := "mc:mcpw@tcp(127.0.0.1:3306)/mc?charset=utf8mb4&parseTime=True&loc=Local"
	//dsn := os.Getenv("MC_DB_DSN")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Errorf("Failed to open db: %s", err)
	}

	var ds Dataset
	result := db.Find(&ds, 6)
	require.NoError(t, result.Error, "Query returned error: %s", result.Error)
	entities, err := ds.GetEntitiesFromTemplate(db)
	require.NoError(t, err, "GetEntitiesFromTemplate failed: %s\n", err)
	require.NotEmpty(t, entities, "Entities is empty")
	for _, entity := range entities {
		if entity.ID == 2324 {
			require.NotEmpty(t, entity.Files, "entity %d has empty files", entity.ID)
		}
	}
}

func TestDatasetTime(t *testing.T) {
	dsn := "mc:mcpw@tcp(127.0.0.1:3306)/mc?charset=utf8mb4&parseTime=True&loc=Local"
	//dsn := os.Getenv("MC_DB_DSN")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Errorf("Failed to open db: %s", err)
	}

	var ds Dataset
	result := db.Find(&ds, 3)
	require.NoError(t, result.Error, "Query returned error: %s", result.Error)
	fmt.Printf("%+v\n", ds)

	fmt.Println("time is: ", ds.PublishedAt.IsZero())
}

func TestLoadGlobusRequest(t *testing.T) {
	dsn := "mc:mcpw@tcp(127.0.0.1:3306)/mc?charset=utf8mb4&parseTime=True&loc=Local"
	//dsn := os.Getenv("MC_DB_DSN")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Errorf("Failed to open db: %s", err)
	}

	var globusTransfer GlobusTransfer
	result := db.Preload("Owner").Find(&globusTransfer, 2)
	require.NoError(t, result.Error, "Unable to find globus request 2: %s", result.Error)
	fmt.Printf("%+v\n", globusTransfer)
}

func TestGormDoesNotExist(t *testing.T) {
	dsn := "mc:mcpw@tcp(127.0.0.1:3306)/mc?charset=utf8mb4&parseTime=True&loc=Local"
	//dsn := os.Getenv("MC_DB_DSN")
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Errorf("Failed to open db: %s", err)
	}

	var f File

	result := db.Preload("Directory").
		Where("directory_id = ?", 20).
		Where("name = ?", "blah").
		Where("current = ?", true).
		First(&f)
	fmt.Println("err:", result.Error)

	require.True(t, errors.Is(result.Error, gorm.ErrRecordNotFound))
}

func TestLoadTransferRequest(t *testing.T) {
	dsn := "mc:mcpw@tcp(127.0.0.1:3306)/mc?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Errorf("Failed to open db: %s", err)
	}

	var tr TransferRequest

	result := db.Preload("GlobusTransfer").First(&tr, 6)
	if result.Error != nil {
		t.Fatalf("Couldn't locate TransferRequest: %s", result.Error)
	}

	if tr.GlobusTransfer == nil {
		t.Fatalf("GlobusTransfer should NOT be nil")
	}

	fmt.Printf("%+v\n", tr.GlobusTransfer)
}

func TestLoadPojects(t *testing.T) {
	dsn := "mc:mcpw@tcp(127.0.0.1:3306)/mc?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Errorf("Failed to open db: %s", err)
	}

	var projects []Project
	result := db.Where("team_id in (select team_id from team2admin where user_id = ?)", 130).
		Or("team_id in (select team_id from team2member where user_id = ?)", 130).
		Find(&projects)
	if result.Error != nil {
		t.Fatalf("Query failed: %s", result.Error)
	}

	if result.RowsAffected == 0 {
		t.Fatalf("Query returned no results")
	}

	fmt.Printf("%+v\n", projects)
}

func TestFirstNoMatch(t *testing.T) {
	dsn := "mc:mcpw@tcp(127.0.0.1:3306)/mc?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Errorf("Failed to open db: %s", err)
	}

	var user User
	result := db.Where("api_token = ?", "no-such-token").First(&user)

	if result.Error == nil {
		t.Fatalf("Should have gotten an error looking for an api_token that doesn't exist")
	}

	fmt.Printf("result.Error = %s\n", result.Error)
}

func TestProjectFilesType(t *testing.T) {
	dsn := "mc:mcpw@tcp(127.0.0.1:3306)/mc?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Errorf("Failed to open db: %s", err)
	}

	var project Project
	result := db.First(&project, 1)

	if result.Error != nil {
		t.Fatalf("Error retrieving project 1: %s", result.Error)
	}

	fmt.Printf("%+v\n", project)

	fileTypes, err := project.GetFileTypes()
	if err != nil {
		t.Fatalf("Error converting file types to map: %s", err)
	}

	fmt.Printf("%+v\n", fileTypes)

	fileTypes["Text"] = 2
	fileTypesAsStr, err := project.ToFileTypeAsString(fileTypes)
	if err != nil {
		t.Fatalf("Unable to convert file types to str: %s", err)
	}

	result = db.Model(project).Update("file_types", fileTypesAsStr)
	if result.Error != nil {
		t.Fatalf("Error updating file types: %s", result.Error)
	}

	result = db.First(&project, 1)

	if result.Error != nil {
		t.Fatalf("Error retrieving project 1: %s", result.Error)
	}

	fmt.Printf("%+v\n", project)

	fileTypes, err = project.GetFileTypes()
	if err != nil {
		t.Fatalf("Error converting file types to map: %s", err)
	}
	fmt.Printf("%+v\n", fileTypes)
}
