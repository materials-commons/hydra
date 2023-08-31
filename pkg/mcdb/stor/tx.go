package stor

import (
	"github.com/materials-commons/hydra/pkg/mcdb/config"
	"gorm.io/gorm"
)

func WithTxRetry(db *gorm.DB, fn func(tx *gorm.DB) error) error {
	var err error

	retryCount := config.GetTxRetry()

	if retryCount < 3 {
		retryCount = 3
	}

	for i := 0; i < retryCount; i++ {
		err = db.Transaction(fn)
		if err == nil {
			break
		}
	}

	return err
}
