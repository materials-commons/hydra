package store

import "github.com/materials-commons/hydra/pkg/mcdb/mcmodel"

type FakeConversionStore struct{}

func NewFakeConversionStore() *FakeConversionStore {
	return &FakeConversionStore{}
}

func (s *FakeConversionStore) AddFileToConvert(file *mcmodel.File) (*mcmodel.Conversion, error) {
	return nil, nil
}
