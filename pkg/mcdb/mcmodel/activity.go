package mcmodel

type Activity struct {
	ID         int         `json:"id"`
	Name       string      `json:"name"`
	Attributes []Attribute `json:"attributes" gorm:"-"`
}
