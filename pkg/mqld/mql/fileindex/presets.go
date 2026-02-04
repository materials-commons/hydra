package fileindex

// ProjectPreset defines common patterns and extractions for a project type
type ProjectPreset struct {
	Type              string
	SearchablePatterns []string
	CommonExtractions []ExtractionPattern
	HelpExamples      []string
	Description       string
}

// ExtractionPattern defines a pattern for extracting structured data
type ExtractionPattern struct {
	Name        string
	Pattern     string
	ValueType   string // "string", "int", "float"
	Unit        string // Optional unit
	Description string
}

var (
	// MaterialsSciencePreset for materials science projects
	MaterialsSciencePreset = ProjectPreset{
		Type:        "materials",
		Description: "Materials Science and Engineering",
		SearchablePatterns: []string{
			"*/logs/*.log",
			"*/results/*.txt",
			"*/data/*.csv",
			"*/output/*.dat",
			"*_notes.txt",
			"*_metadata.txt",
		},
		CommonExtractions: []ExtractionPattern{
			{
				Name:        "Temperature",
				Pattern:     `Temp(?:erature)?:?\s*(\d+\.?\d*)\s*[CK°]`,
				ValueType:   "float",
				Unit:        "C",
				Description: "Temperature values in Celsius or Kelvin",
			},
			{
				Name:        "Pressure",
				Pattern:     `Pressure:?\s*(\d+\.?\d*)\s*(?:mbar|Pa|MPa|kPa)`,
				ValueType:   "float",
				Unit:        "mbar",
				Description: "Pressure values",
			},
			{
				Name:        "SampleID",
				Pattern:     `Sample[-_]?(\w+)`,
				ValueType:   "string",
				Unit:        "",
				Description: "Sample identifiers",
			},
			{
				Name:        "YieldStrength",
				Pattern:     `[Yy]ield [Ss]trength:?\s*(\d+\.?\d*)\s*MPa`,
				ValueType:   "float",
				Unit:        "MPa",
				Description: "Yield strength values",
			},
			{
				Name:        "TensileStrength",
				Pattern:     `[Tt]ensile [Ss]trength:?\s*(\d+\.?\d*)\s*MPa`,
				ValueType:   "float",
				Unit:        "MPa",
				Description: "Tensile strength values",
			},
			{
				Name:        "Composition",
				Pattern:     `Composition:?\s*([A-Za-z0-9\s\.\-]+)`,
				ValueType:   "string",
				Unit:        "",
				Description: "Material composition",
			},
			{
				Name:        "GrainSize",
				Pattern:     `[Gg]rain [Ss]ize:?\s*(\d+\.?\d*)\s*(?:μm|um|nm)`,
				ValueType:   "float",
				Unit:        "μm",
				Description: "Grain size measurements",
			},
		},
		HelpExamples: []string{
			`find-in-files "Temperature:" in logs`,
			`make-samples from found-data using pattern "Sample-(\\w+)"`,
			`extract-from-files pattern "Temp: (\\d+)C" as Temperature`,
			`query samples where {[has-file {[file-contains "heat treatment"]}]}`,
		},
	}

	// ChemistryPreset for chemistry projects
	ChemistryPreset = ProjectPreset{
		Type:        "chemistry",
		Description: "Chemistry and Chemical Engineering",
		SearchablePatterns: []string{
			"*/logs/*.log",
			"*/results/*.txt",
			"*/analysis/*.csv",
			"*_lab_notes.txt",
		},
		CommonExtractions: []ExtractionPattern{
			{
				Name:        "pH",
				Pattern:     `pH:?\s*(\d+\.?\d*)`,
				ValueType:   "float",
				Unit:        "",
				Description: "pH values",
			},
			{
				Name:        "Concentration",
				Pattern:     `Conc(?:entration)?:?\s*(\d+\.?\d*)\s*(?:M|mM|μM)`,
				ValueType:   "float",
				Unit:        "M",
				Description: "Concentration values",
			},
			{
				Name:        "Yield",
				Pattern:     `Yield:?\s*(\d+\.?\d*)\s*%`,
				ValueType:   "float",
				Unit:        "%",
				Description: "Reaction yield percentage",
			},
			{
				Name:        "ReactionTime",
				Pattern:     `[Rr]eaction [Tt]ime:?\s*(\d+\.?\d*)\s*(?:h|hr|hours?|min|minutes?)`,
				ValueType:   "float",
				Unit:        "h",
				Description: "Reaction time",
			},
			{
				Name:        "CompoundID",
				Pattern:     `Compound[-_]?(\w+)`,
				ValueType:   "string",
				Unit:        "",
				Description: "Compound identifiers",
			},
		},
		HelpExamples: []string{
			`find-in-files "pH:" in results`,
			`extract-from-files pattern "Yield: (\\d+\\.?\\d*)%" as Yield`,
			`make-samples from found-data using pattern "Compound-(\\w+)"`,
		},
	}

	// BiologyPreset for biology projects
	BiologyPreset = ProjectPreset{
		Type:        "biology",
		Description: "Biology and Life Sciences",
		SearchablePatterns: []string{
			"*/logs/*.log",
			"*/results/*.txt",
			"*/sequences/*.fasta",
			"*/analysis/*.csv",
			"*_notes.txt",
		},
		CommonExtractions: []ExtractionPattern{
			{
				Name:        "CellCount",
				Pattern:     `[Cc]ell [Cc]ount:?\s*(\d+\.?\d*(?:e[+-]?\d+)?)\s*(?:cells?)?`,
				ValueType:   "float",
				Unit:        "cells",
				Description: "Cell count values",
			},
			{
				Name:        "Viability",
				Pattern:     `[Vv]iability:?\s*(\d+\.?\d*)\s*%`,
				ValueType:   "float",
				Unit:        "%",
				Description: "Cell viability percentage",
			},
			{
				Name:        "GrowthRate",
				Pattern:     `[Gg]rowth [Rr]ate:?\s*(\d+\.?\d*)`,
				ValueType:   "float",
				Unit:        "1/h",
				Description: "Growth rate",
			},
			{
				Name:        "SampleID",
				Pattern:     `Sample[-_]?(\w+)`,
				ValueType:   "string",
				Unit:        "",
				Description: "Sample identifiers",
			},
		},
		HelpExamples: []string{
			`find-in-files "Cell Count:" in results`,
			`extract-from-files pattern "Viability: (\\d+\\.?\\d*)%" as Viability`,
			`make-samples from found-data using pattern "Sample-(\\w+)"`,
		},
	}

	// GeneralPreset for general/custom projects
	GeneralPreset = ProjectPreset{
		Type:        "general",
		Description: "General Purpose",
		SearchablePatterns: []string{
			"**/*.txt",
			"**/*.log",
			"**/*.csv",
			"**/*.dat",
		},
		CommonExtractions: []ExtractionPattern{
			{
				Name:        "SampleID",
				Pattern:     `Sample[-_]?(\w+)`,
				ValueType:   "string",
				Unit:        "",
				Description: "Sample identifiers",
			},
			{
				Name:        "NumericValue",
				Pattern:     `Value:?\s*(\d+\.?\d*)`,
				ValueType:   "float",
				Unit:        "",
				Description: "Generic numeric values",
			},
		},
		HelpExamples: []string{
			`find-in-files "your search text"`,
			`show searchable-files`,
			`make-samples from found-data using pattern "Sample-(\\w+)"`,
		},
	}
)

// GetPreset returns a preset by type name
func GetPreset(presetType string) *ProjectPreset {
	switch presetType {
	case "materials":
		return &MaterialsSciencePreset
	case "chemistry":
		return &ChemistryPreset
	case "biology":
		return &BiologyPreset
	case "general":
		return &GeneralPreset
	default:
		return &GeneralPreset
	}
}

// ListAvailablePresets returns a list of all available preset types
func ListAvailablePresets() []ProjectPreset {
	return []ProjectPreset{
		MaterialsSciencePreset,
		ChemistryPreset,
		BiologyPreset,
		GeneralPreset,
	}
}
