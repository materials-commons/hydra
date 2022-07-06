package mcmodel

type Entity struct {
	ID           int           `json:"id"`
	Name         string        `json:"name"`
	Files        []File        `json:"files" gorm:"many2many:entity2file"`
	EntityStates []EntityState `json:"entity_states"`
}

type EntityState struct {
	ID         int         `json:"id"`
	EntityID   int         `json:"entity_id"`
	Attributes []Attribute `json:"attributes" gorm:"-"`
}
