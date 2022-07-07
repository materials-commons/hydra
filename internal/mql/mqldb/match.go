package mqldb

import (
	"strconv"

	"github.com/materials-commons/hydra/pkg/mcdb/mcmodel"
)

func tryEvalAttributeIntMatch(val1 int64, match MatchStatement) bool {
	val2, ok := matchValToInt(match)
	if !ok {
		return false
	}
	return evalIntMatch(val1, val2, match.Operation)
}

func matchValToInt(match MatchStatement) (int64, bool) {
	switch match.Value.(type) {
	case int64:
		return match.Value.(int64), true
	case int:
		return int64(match.Value.(int)), true
	case float64:
		return int64(match.Value.(float64)), true
	case float32:
		return int64(match.Value.(float32)), true
	case string:
		val, err := strconv.ParseFloat(match.Value.(string), 64)
		if err != nil {
			return -1, false
		}

		return int64(val), true
	default:
		return -1, false
	}
}

func tryEvalAttributeFloatMatch(val1 float64, match MatchStatement) bool {
	val2, ok := matchValToFloat(match)
	if !ok {
		return false
	}

	return evalFloatMatch(val1, val2, match.Operation)
}

func matchValToFloat(match MatchStatement) (float64, bool) {
	switch match.Value.(type) {
	case int64:
		return float64(match.Value.(int64)), true
	case int:
		return float64(match.Value.(int)), true
	case float64:
		return match.Value.(float64), true
	case float32:
		return float64(match.Value.(float32)), true
	case string:
		val, err := strconv.ParseFloat(match.Value.(string), 64)
		if err != nil {
			return -1, false
		}

		return val, true
	default:
		return -1, false
	}
}

func tryEvalAttributeStringMatch(val1 string, match MatchStatement) bool {
	val2, ok := match.Value.(string)
	if !ok {
		return false
	}
	return evalStringMatch(val1, val2, match.Operation)
}

func evalProcessFieldMatch(process *mcmodel.Activity, match MatchStatement) bool {
	if process == nil {
		return false
	}
	if match.FieldName == "name" {
		name, ok := match.Value.(string)
		if !ok {
			return false
		}
		return evalStringMatch(name, process.Name, match.Operation)
	}

	if match.FieldName == "id" {
		id, ok := match.Value.(int)
		if !ok {
			return false
		}
		return evalIntMatch(int64(id), int64(process.ID), match.Operation)
	}

	return false
}

func evalSampleFieldMatch(sampleState *SampleState, match MatchStatement) bool {
	if sampleState == nil {
		return false
	}
	if match.FieldName == "name" {
		name, ok := match.Value.(string)
		if !ok {
			return false
		}
		return evalStringMatch(name, sampleState.sample.Name, match.Operation)
	}

	if match.FieldName == "id" {
		id, ok := match.Value.(int)
		if !ok {
			return false
		}
		return evalIntMatch(int64(id), int64(sampleState.sample.ID), match.Operation)
	}

	return false
}

func evalStringMatch(val1, val2, operation string) bool {
	switch operation {
	case "=":
		return val1 == val2
	case "<>":
		return val1 != val2
	default:
		return false
	}
}

func evalIntMatch(val1, val2 int64, operation string) bool {
	switch operation {
	case "=":
		return val1 == val2
	case "<>":
		return val1 != val2
	case ">":
		return val1 > val2
	case ">=":
		return val1 >= val2
	case "<":
		return val1 < val2
	case "<=":
		return val1 <= val2
	default:
		return false
	}
}

func evalFloatMatch(val1, val2 float64, operation string) bool {
	switch operation {
	case "=":
		return val1 == val2
	case "<>":
		return val1 != val2
	case ">":
		return val1 > val2
	case ">=":
		return val1 >= val2
	case "<":
		return val1 < val2
	case "<=":
		return val1 <= val2
	default:
		return false
	}
}
