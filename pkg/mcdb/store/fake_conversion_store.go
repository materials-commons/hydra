package store

import "github.com/materials-commons/gomcdb/mcmodel"

type FakeConversionStore struct{}

func NewFakeConversionStore() *FakeConversionStore {
	return &FakeConversionStore{}
}

func (s *FakeConversionStore) AddFileToConvert(file *mcmodel.File) (*mcmodel.Conversion, error) {
	return nil, nil
}
