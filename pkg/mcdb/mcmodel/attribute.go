package mcmodel

import (
	"encoding/json"
	"strconv"
	"strings"
)

const (
	ValueTypeUnset          = 0
	ValueTypeInt            = 1
	ValueTypeFloat          = 2
	ValueTypeString         = 3
	ValueTypeComplex        = 4
	ValueTypeArrayOfInt     = 5
	ValueTypeArrayOfFloat   = 6
	ValueTypeArrayOfString  = 7
	ValueTypeArrayOfComplex = 8
)

type Attribute struct {
	ID               int              `json:"id"`
	UUID             string           `json:"uuid"`
	Name             string           `json:"name"`
	AttributableID   int              `json:"attributable_id"`
	AttributableType string           `json:"attributable_type"`
	AttributeValues  []AttributeValue `json:"attribute_values"`
}

type AttributeValue struct {
	ID                  int                      `json:"id"`
	UUID                string                   `json:"uuid"`
	AttributeID         int                      `json:"attribute_id"`
	Unit                string                   `json:"unit"`
	Val                 string                   `json:"val"`
	ValueType           int                      `json:"value_type" gorm:"-"`
	ValueInt            int64                    `json:"value_int" gorm:"-"`
	ValueFloat          float64                  `json:"value_float" gorm:"-"`
	ValueString         string                   `json:"value_string" gorm:"-"`
	ValueComplex        map[string]interface{}   `gorm:"-"`
	ValueArrayOfInt     []int64                  `gorm:"-"`
	ValueArrayOfFloat   []float64                `gorm:"-"`
	ValueArrayOfString  []string                 `gorm:"-"`
	ValueArrayOfComplex []map[string]interface{} `gorm:"-"`
}

func (a *Attribute) LoadValues() error {
	if len(a.AttributeValues) == 0 {
		return nil
	}
	for i := range a.AttributeValues {
		if a.AttributeValues[i].ValueType != ValueTypeUnset {
			// Value already set so skip
			continue
		}

		var val map[string]interface{}
		if err := json.Unmarshal([]byte(a.AttributeValues[i].Val), &val); err != nil {
			return err
		}
		//fmt.Printf("%+v\n", val)
		switch val["value"].(type) {
		case int:
			a.AttributeValues[i].ValueType = ValueTypeInt
			a.AttributeValues[i].ValueInt = val["value"].(int64)
		case []int:
			a.AttributeValues[i].ValueType = ValueTypeArrayOfInt
			a.AttributeValues[i].ValueArrayOfInt = val["value"].([]int64)
		case float32, float64:
			a.AttributeValues[i].ValueType = ValueTypeFloat
			a.AttributeValues[i].ValueFloat = val["value"].(float64)
		case []float32, []float64:
			a.AttributeValues[i].ValueType = ValueTypeArrayOfFloat
			a.AttributeValues[i].ValueArrayOfFloat = val["value"].([]float64)
		case string:
			// Lots of numeric values are stored as strings, so we need to check and convert
			valAsStr := val["value"].(string)

			if strings.Contains(valAsStr, ".") {
				// Try and convert to float, if that fails, then keep as string
				valAsFloat, err := strconv.ParseFloat(valAsStr, 64)
				if err == nil {
					a.AttributeValues[i].ValueType = ValueTypeFloat
					a.AttributeValues[i].ValueFloat = valAsFloat
					return nil
				}
			}

			// Float failed so try and convert to int and if that is successful store as int, otherwise as string
			valAsInt, err := strconv.ParseInt(valAsStr, 10, 64)
			if err == nil {
				a.AttributeValues[i].ValueType = ValueTypeInt
				a.AttributeValues[i].ValueInt = valAsInt
				return nil
			}

			// Nope, its still a string
			a.AttributeValues[i].ValueType = ValueTypeString
			a.AttributeValues[i].ValueString = valAsStr
		case []string:
			// support later
		case map[interface{}]interface{}:
			// support later
		case []map[interface{}]interface{}:
			// support later
		default:
			//fmt.Printf("Unknown cast type for attribute %s\n", a.Name)
			// What to do here?
		}
	}
	return nil
}

func (a Attribute) GetValue() interface{} {
	return nil
}
