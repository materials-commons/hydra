package mqldb

import (
	"github.com/apex/log"
	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
	"gorm.io/gorm"
)

type DB struct {
	ProjectID int
	db        *gorm.DB

	// Process and process data lookups
	Processes                    []mcmodel.Activity
	AllProcessAttributes         []*mcmodel.Attribute
	ProcessAttributesByProcessID map[int]map[string]*mcmodel.Attribute
	ProcessSamples               map[int][]*mcmodel.Entity

	// Sample and sample data lookups
	Samples             []mcmodel.Entity
	SampleProcesses     map[int][]*mcmodel.Activity
	AllSampleAttributes []*mcmodel.Attribute

	// A sample can have multiple states, thus this maps a sample id to a list of states, then each
	// state is a hash map of the attribute name to the attribute structure. For example, given a
	// sample with ID 1, that has 2 states (with IDs of 4, and 5), where each state has two attributes,
	// named "AttrA" and "AttrB", then you would end up with SampleAttributesBySampleIDAndStates
	// looking as follows:
	// [1] <-- Sample id key into hash map (map[int]...)
	//    [4] <-- First entity state id in entry for the first state (map[int]map[int]...)
	//       ["AttrA"] = &mcmodel.Attribute  <- AttrA is the key into map[string]
	//       ["AttrB"] = &mcmodel.Attribute
	//    [5]
	//       ["AttrA"] = &mcmodel.Attribute
	//       ["AttrB"] = &mcmodel.Attribute
	// ie map[1]map[4]map["AttrA"] = &mcmodel.Attribute{Name: "AttrA", Value...}
	//    map[1]map[4]map["AttrB"] = &mcmodel.Attribute{Name: "AttrB", Value...}
	//    map[1]map[5]map["AttrA"] = &mcmodel.Attribute{Name: "AttrA", Value (different than AttrA above)}
	//    map[1]map[5]map["AttrB"] = &mcmodel.Attribute{Name: "AttrB", Value (different than AttrB above)}

	SampleAttributesBySampleIDAndStates map[int]map[int]map[string]*mcmodel.Attribute
}

// NewDB creates a new in memory instance of the samples, processes, attributes and their relationships DB that
// is used by the evaluator for query processing.
func NewDB(projectID int, db *gorm.DB) *DB {
	return &DB{
		ProjectID:                           projectID,
		db:                                  db,
		ProcessAttributesByProcessID:        make(map[int]map[string]*mcmodel.Attribute),
		ProcessSamples:                      make(map[int][]*mcmodel.Entity),
		SampleAttributesBySampleIDAndStates: make(map[int]map[int]map[string]*mcmodel.Attribute),
		SampleProcesses:                     make(map[int][]*mcmodel.Activity),
	}
}

// Activity2Entity represents the join table for mapping the relationships between processes and samples.
type Activity2Entity struct {
	ID         int
	ActivityID int
	EntityID   int
}

func (Activity2Entity) TableName() string {
	return "activity2entity"
}

// Load loads the samples, processes and attributes for the given project into memory.
func (db *DB) Load() error {
	// Make sure project exists
	var project mcmodel.Project
	if err := db.db.First(&project, db.ProjectID).Error; err != nil {
		return err
	}

	if err := db.loadProcessesAndAttributes(); err != nil {
		return err
	}

	if err := db.loadSamplesAndAttributes(); err != nil {
		return err
	}

	if err := db.loadProcessSampleMappings(); err != nil {
		return err
	}

	db.wireupAttributesToProcessesAndSamples()

	return nil
}

func (db *DB) loadProcessesAndAttributes() error {
	if err := db.db.Where("project_id = ?", db.ProjectID).Find(&db.Processes).Error; err != nil {
		return err
	}

	err := db.db.Preload("AttributeValues").Where("attributable_type = ?", "App\\Models\\Activity").
		Where("attributable_id in (select id from activities where project_id = ?)", db.ProjectID).
		Find(&db.AllProcessAttributes).Error
	if err != nil {
		return err
	}

	for _, process := range db.Processes {
		db.ProcessAttributesByProcessID[process.ID] = make(map[string]*mcmodel.Attribute)
	}

	for i, attr := range db.AllProcessAttributes {
		db.ProcessAttributesByProcessID[attr.AttributableID][attr.Name] = db.AllProcessAttributes[i]
		if err := attr.LoadValues(); err != nil {
			log.Errorf("Failed converting attribute %d/%s values: %s", attr.ID, attr.Name, err)
		}
	}

	return nil
}

func (db *DB) loadSamplesAndAttributes() error {
	err := db.db.Preload("EntityStates").Where("project_id = ?", db.ProjectID).Find(&db.Samples).Error
	if err != nil {
		return err
	}

	err = db.db.Preload("AttributeValues").Where("attributable_type = ?", "App\\Models\\EntityState").
		Where(`attributable_id in 
                       (select distinct id from entity_states where entity_id in 
                               (select id from entities where project_id = ?))`, db.ProjectID).
		Find(&db.AllSampleAttributes).Error
	if err != nil {
		return err
	}

	// Create a map of entity state ids to sample state ids because the attributes are
	// all going to have an entity state id associated with them and we need to figure
	// out which sample that state is associated with. Also create hash entries for the
	// samples in SampleAttributeBySampleIDAndStates
	entityStateIDToSampleID := make(map[int]int)
	for _, sample := range db.Samples {
		db.SampleAttributesBySampleIDAndStates[sample.ID] = make(map[int]map[string]*mcmodel.Attribute)
		for _, entityState := range sample.EntityStates {
			entityStateIDToSampleID[entityState.ID] = sample.ID
			db.SampleAttributesBySampleIDAndStates[sample.ID][entityState.ID] = make(map[string]*mcmodel.Attribute)
		}
	}

	// Load the SampleAttributesBySampleIDAndStates map of values
	for i, attr := range db.AllSampleAttributes {
		// Here AttributableType == "App\Models\EntityState" and AttributableID == EntityState.ID
		sampleID := entityStateIDToSampleID[attr.AttributableID]
		db.SampleAttributesBySampleIDAndStates[sampleID][attr.AttributableID][attr.Name] = db.AllSampleAttributes[i]
		if err := attr.LoadValues(); err != nil {
			log.Errorf("Failed converting attribute %d/%s values: %s", attr.ID, attr.Name, err)
		}
	}

	return nil
}

func (db *DB) loadProcessSampleMappings() error {
	// Now setup mapping of samples -> to their associated processes, and processes -> to their associated samples
	var activity2entity []Activity2Entity
	err := db.db.Where("entity_id in (select id from entities where project_id = ?)", db.ProjectID).
		Find(&activity2entity).Error
	if err != nil {
		return err
	}

	// For fast lookup map the samples and processes by their id. This will be used in activity2entity work below
	// to create db entry map of samples to their list of processes, and processes to their list of samples.
	sampleMap := make(map[int]*mcmodel.Entity)
	for i := range db.Samples {
		sampleMap[db.Samples[i].ID] = &db.Samples[i]
	}

	processMap := make(map[int]*mcmodel.Activity)
	for i := range db.Processes {
		processMap[db.Processes[i].ID] = &db.Processes[i]
	}

	for _, a2e := range activity2entity {
		sample := sampleMap[a2e.EntityID]
		process := processMap[a2e.ActivityID]
		if sample != nil {
			if process != nil {
				db.SampleProcesses[sample.ID] = append(db.SampleProcesses[sample.ID], process)
			}
		}

		if process != nil {
			if sample != nil {
				db.ProcessSamples[process.ID] = append(db.ProcessSamples[process.ID], sample)
			}
		}
	}

	return nil
}

func (db *DB) wireupAttributesToProcessesAndSamples() {
	// Add the process attributes to each process
	for i := range db.Processes {
		for attrName := range db.ProcessAttributesByProcessID[db.Processes[i].ID] {
			attr := db.ProcessAttributesByProcessID[db.Processes[i].ID][attrName]
			db.Processes[i].Attributes = append(db.Processes[i].Attributes, *attr)
		}
	}

	// Add the sample state and sample state attributes to each sample by iterating through the
	// SampleAttributesBySampleIDAndStates map that is a multi-level hash map of
	// samples -> sample states -> attrNames ->Attribute
	for i := range db.Samples {
		sampleID := db.Samples[i].ID
		for sampleStateID := range db.SampleAttributesBySampleIDAndStates[sampleID] {
			// We have the Sample State ID, so create a sample state that will be used to
			// add attributes specific to that state, and then append that state to the
			// list of sample states for the sample.
			entityState := mcmodel.EntityState{
				ID:       sampleStateID,
				EntityID: sampleID,
			}

			// Add attributes for that sample state to the state we just created
			for attrName := range db.SampleAttributesBySampleIDAndStates[sampleID][sampleStateID] {
				attr := db.SampleAttributesBySampleIDAndStates[sampleID][sampleStateID][attrName]
				entityState.Attributes = append(entityState.Attributes, *attr)
			}

			// Append that sample state to the list of sample states for this sample
			db.Samples[i].EntityStates = append(db.Samples[i].EntityStates, entityState)
		}
	}
}
