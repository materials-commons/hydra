package stor

import (
	"time"

	"github.com/hashicorp/go-uuid"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

type InMemoryConversionStor struct {
	// Can be used to simulate an error by setting it.
	ErrorToReturn error
	lastID        int
}

func NewInMemoryConversionStor() *InMemoryConversionStor {
	return &InMemoryConversionStor{lastID: 10000}
}

func (s *InMemoryConversionStor) AddFileToConvert(file *mcmodel.File) (*mcmodel.Conversion, error) {
	if s.ErrorToReturn != nil {
		return nil, s.ErrorToReturn
	}

	var (
		UUID string
		err  error
	)

	if UUID, err = uuid.GenerateUUID(); err != nil {
		return nil, err
	}

	now := time.Now()
	c := &mcmodel.Conversion{
		ID:        s.lastID,
		UUID:      UUID,
		ProjectID: file.ProjectID,
		OwnerID:   file.OwnerID,
		FileID:    file.ID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	s.lastID = s.lastID + 1

	return c, nil
}
