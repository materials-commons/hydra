package mqldb

type Selection struct {
	ProcessSelection ProcessSelection
	SampleSelection  SampleSelection
}

type ProcessSelection struct {
	All        bool
	Name       bool
	ID         bool
	Attributes []string
}

type SampleSelection struct {
	All        bool
	Name       bool
	ID         bool
	Attributes []string
}
