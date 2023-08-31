package mc

import (
	"github.com/materials-commons/hydra/pkg/mcdb/stor"
	"gorm.io/gorm"
)

// Stores is a place to consolidate the various stores that are used by the handlers. It
// allows the stores to be easily created and cleans up the number of parameters that need
// to be passed in to create a mcscp.Handler or mcsftp.Handler.
type Stores struct {
	FileStore       stor.FileStor
	ProjectStore    stor.ProjectStor
	ConversionStore stor.ConversionStor
}

func NewGormStores(db *gorm.DB, mcfsRoot string) *Stores {
	return &Stores{
		FileStore:       stor.NewGormFileStor(db, mcfsRoot),
		ProjectStore:    stor.NewGormProjectStor(db),
		ConversionStore: stor.NewGormConversionStor(db),
	}
}
