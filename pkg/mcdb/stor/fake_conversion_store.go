package stor

import "github.com/materials-commons/hydra/pkg/mcdb/mcmodel"

type FakeConversionStor struct{}

func NewFakeConversionStor() *FakeConversionStor {
	return &FakeConversionStor{}
}

func (s *FakeConversionStor) AddFileToConvert(file *mcmodel.File) (*mcmodel.Conversion, error) {
	return nil, nil
}
