package stor

import "github.com/materials-commons/hydra/pkg/mcdb/mcmodel"

type InMemoryConversionStor struct{}

func NewInMemoryConversionStor() *InMemoryConversionStor {
	return &InMemoryConversionStor{}
}

func (s *InMemoryConversionStor) AddFileToConvert(file *mcmodel.File) (*mcmodel.Conversion, error) {
	return nil, nil
}
