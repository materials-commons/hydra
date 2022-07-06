package mqldb

import "github.com/materials-commons/gomcdb/mcmodel"

func createTestDB() *DB {
	db := &DB{
		ProjectID:                           1,
		ProcessAttributesByProcessID:        make(map[int]map[string]*mcmodel.Attribute),
		SampleAttributesBySampleIDAndStates: make(map[int]map[int]map[string]*mcmodel.Attribute),
	}

	// Fill out processes
	db.Processes = append(db.Processes, mcmodel.Activity{
		ID:   1,
		Name: "EBSD",
	})

	db.Processes = append(db.Processes, mcmodel.Activity{
		ID:   2,
		Name: "EBSD",
	})

	db.Processes = append(db.Processes, mcmodel.Activity{
		ID:   3,
		Name: "Texture",
	})

	db.Processes = append(db.Processes, mcmodel.Activity{
		ID:   4,
		Name: "Texture",
	})

	// First EBSD process attributes
	db.ProcessAttributesByProcessID[1] = make(map[string]*mcmodel.Attribute)
	db.ProcessAttributesByProcessID[1]["Beam Type"] = &mcmodel.Attribute{
		Name: "Beam Type",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType:   mcmodel.ValueTypeString,
				ValueString: "Wide",
			},
		},
	}
	db.ProcessAttributesByProcessID[1]["frames per second"] = &mcmodel.Attribute{
		Name: "frames per second",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType: mcmodel.ValueTypeInt,
				ValueInt:  5,
			},
		},
	}
	db.ProcessAttributesByProcessID[1]["note"] = &mcmodel.Attribute{
		Name: "note",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType:   mcmodel.ValueTypeString,
				ValueString: "ignore these results",
			},
		},
	}

	// Second EBSD process attributes
	db.ProcessAttributesByProcessID[2] = make(map[string]*mcmodel.Attribute)
	db.ProcessAttributesByProcessID[2]["Beam Type"] = &mcmodel.Attribute{
		Name: "Beam Type",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType:   mcmodel.ValueTypeString,
				ValueString: "Thin",
			},
		},
	}
	db.ProcessAttributesByProcessID[2]["frames per second"] = &mcmodel.Attribute{
		Name: "frames per second",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType: mcmodel.ValueTypeInt,
				ValueInt:  3,
			},
		},
	}

	// First Texture process attributes
	db.ProcessAttributesByProcessID[3] = make(map[string]*mcmodel.Attribute)
	db.ProcessAttributesByProcessID[3]["PF scale max"] = &mcmodel.Attribute{
		Name: "PF scale max",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType: mcmodel.ValueTypeInt,
				ValueInt:  2,
			},
		},
	}
	db.ProcessAttributesByProcessID[3]["note"] = &mcmodel.Attribute{
		Name: "note",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType:   mcmodel.ValueTypeString,
				ValueString: "ignore these results",
			},
		},
	}

	// Second Texture process attributes
	db.ProcessAttributesByProcessID[4] = make(map[string]*mcmodel.Attribute)
	db.ProcessAttributesByProcessID[4]["PF scale max"] = &mcmodel.Attribute{
		Name: "PF scale max",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType: mcmodel.ValueTypeInt,
				ValueInt:  3,
			},
		},
	}

	///////////////// Fill out samples

	db.Samples = append(db.Samples, mcmodel.Entity{
		Name: "S1",
		ID:   1,
		EntityStates: []mcmodel.EntityState{
			{ID: 1},
			{ID: 2},
		},
	})
	db.SampleAttributesBySampleIDAndStates[1] = make(map[int]map[string]*mcmodel.Attribute)

	// Allocate for entity states
	db.SampleAttributesBySampleIDAndStates[1][1] = make(map[string]*mcmodel.Attribute)
	db.SampleAttributesBySampleIDAndStates[1][1]["zn"] = &mcmodel.Attribute{
		Name: "zn",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType:  mcmodel.ValueTypeFloat,
				ValueFloat: 0.5,
			},
		},
	}
	db.SampleAttributesBySampleIDAndStates[1][1]["mg"] = &mcmodel.Attribute{
		Name: "mg",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType:  mcmodel.ValueTypeFloat,
				ValueFloat: 0.4,
			},
		},
	}

	db.SampleAttributesBySampleIDAndStates[1][2] = make(map[string]*mcmodel.Attribute)
	db.SampleAttributesBySampleIDAndStates[1][2]["zn"] = &mcmodel.Attribute{
		Name: "zn",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType:  mcmodel.ValueTypeFloat,
				ValueFloat: 0.5,
			},
		},
	}
	db.SampleAttributesBySampleIDAndStates[1][2]["mg"] = &mcmodel.Attribute{
		Name: "mg",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType:  mcmodel.ValueTypeFloat,
				ValueFloat: 0.5,
			},
		},
	}
	db.SampleAttributesBySampleIDAndStates[1][2]["hardness"] = &mcmodel.Attribute{
		Name: "hardness",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType: mcmodel.ValueTypeInt,
				ValueInt:  1,
			},
		},
	}

	db.Samples = append(db.Samples, mcmodel.Entity{
		Name: "S2",
		ID:   2,
		EntityStates: []mcmodel.EntityState{
			{ID: 3},
			{ID: 4},
		},
	})
	db.SampleAttributesBySampleIDAndStates[2] = make(map[int]map[string]*mcmodel.Attribute)

	// Allocate for entity states
	db.SampleAttributesBySampleIDAndStates[2][3] = make(map[string]*mcmodel.Attribute)
	db.SampleAttributesBySampleIDAndStates[2][3]["zn"] = &mcmodel.Attribute{
		Name: "zn",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType:  mcmodel.ValueTypeFloat,
				ValueFloat: 0.5,
			},
		},
	}
	db.SampleAttributesBySampleIDAndStates[2][3]["mg"] = &mcmodel.Attribute{
		Name: "mg",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType:  mcmodel.ValueTypeFloat,
				ValueFloat: 0.4,
			},
		},
	}
	db.SampleAttributesBySampleIDAndStates[2][3]["ductility"] = &mcmodel.Attribute{
		Name: "ductility",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType:  mcmodel.ValueTypeFloat,
				ValueFloat: 0.81,
			},
		},
	}
	db.SampleAttributesBySampleIDAndStates[2][3]["alloy"] = &mcmodel.Attribute{
		Name: "alloy",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType:   mcmodel.ValueTypeString,
				ValueString: "zn45",
			},
		},
	}

	db.SampleAttributesBySampleIDAndStates[2][4] = make(map[string]*mcmodel.Attribute)
	db.SampleAttributesBySampleIDAndStates[2][4]["zn"] = &mcmodel.Attribute{
		Name: "zn",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType:  mcmodel.ValueTypeFloat,
				ValueFloat: 0.6,
			},
		},
	}
	db.SampleAttributesBySampleIDAndStates[2][4]["mg"] = &mcmodel.Attribute{
		Name: "mg",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType:  mcmodel.ValueTypeFloat,
				ValueFloat: 0.3,
			},
		},
	}
	db.SampleAttributesBySampleIDAndStates[2][4]["bend"] = &mcmodel.Attribute{
		Name: "bend",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType:   mcmodel.ValueTypeString,
				ValueString: "Right",
			},
		},
	}

	db.Samples = append(db.Samples, mcmodel.Entity{
		Name: "S3",
		ID:   3,
		EntityStates: []mcmodel.EntityState{
			{ID: 5},
			{ID: 6},
		},
	})
	db.SampleAttributesBySampleIDAndStates[3] = make(map[int]map[string]*mcmodel.Attribute)

	// Allocate for entity states
	db.SampleAttributesBySampleIDAndStates[3][5] = make(map[string]*mcmodel.Attribute)
	db.SampleAttributesBySampleIDAndStates[3][5]["zn"] = &mcmodel.Attribute{
		Name: "zn",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType:  mcmodel.ValueTypeFloat,
				ValueFloat: 0.68,
			},
		},
	}
	db.SampleAttributesBySampleIDAndStates[3][5]["mg"] = &mcmodel.Attribute{
		Name: "mg",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType:  mcmodel.ValueTypeFloat,
				ValueFloat: 0.32,
			},
		},
	}

	db.SampleAttributesBySampleIDAndStates[3][6] = make(map[string]*mcmodel.Attribute)
	db.SampleAttributesBySampleIDAndStates[3][6]["zn"] = &mcmodel.Attribute{
		Name: "zn",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType:  mcmodel.ValueTypeFloat,
				ValueFloat: 0.45,
			},
		},
	}
	db.SampleAttributesBySampleIDAndStates[3][6]["mg"] = &mcmodel.Attribute{
		Name: "mg",
		AttributeValues: []mcmodel.AttributeValue{
			{
				ValueType:  mcmodel.ValueTypeFloat,
				ValueFloat: 0.45,
			},
		},
	}

	// Now set up the mapping of processes to samples, and samples to processes
	db.ProcessSamples = make(map[int][]*mcmodel.Entity)

	// EBSD
	db.ProcessSamples[1] = []*mcmodel.Entity{
		{ID: 1, Name: "S1"},
		{ID: 2, Name: "S2"},
	}

	// EBSD
	db.ProcessSamples[2] = []*mcmodel.Entity{
		{ID: 3, Name: "S3"},
	}

	// Texture
	db.ProcessSamples[3] = []*mcmodel.Entity{
		{ID: 1, Name: "S1"},
		{ID: 2, Name: "S2"},
	}

	// Texture
	db.ProcessSamples[4] = []*mcmodel.Entity{
		{ID: 3, Name: "S3"},
	}

	db.SampleProcesses = make(map[int][]*mcmodel.Activity)

	// S1
	db.SampleProcesses[1] = []*mcmodel.Activity{
		{ID: 1, Name: "EBSD"},
		{ID: 3, Name: "Texture"},
	}

	// S2
	db.SampleProcesses[2] = []*mcmodel.Activity{
		{ID: 1, Name: "EBSD"},
		{ID: 3, Name: "Texture"},
	}

	// S3
	db.SampleProcesses[3] = []*mcmodel.Activity{
		{ID: 2, Name: "EBSD"},
		{ID: 4, Name: "Texture"},
	}

	return db
}

func selectAllProcesses() Selection {
	return Selection{
		ProcessSelection: ProcessSelection{
			All: true,
		},
	}
}

func selectAllSamples() Selection {
	return Selection{
		SampleSelection: SampleSelection{
			All: true,
		},
	}
}
