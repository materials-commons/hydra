package mqldb

// MapToStatement takes a map, which represents the converted JSON payload for a statement
// and converts it to statement. It recursively calls itself to build out the full statement.
func MapToStatement(m map[string]interface{}) Statement {
	//fmt.Printf("MapToStatement = %+v\n", m)
	_, hasAnd := m["and"]
	_, hasOr := m["or"]
	_, hasFieldName := m["field_name"]
	switch {
	case hasAnd:
		andStatement := AndStatement{}
		left, hasLeft := m["left"]
		if hasLeft {
			andStatement.Left = MapToStatement(left.(map[string]interface{}))
		}

		right, hasRight := m["right"]
		if hasRight {
			andStatement.Right = MapToStatement(right.(map[string]interface{}))
		}

		return andStatement

	case hasOr:
		orStatement := OrStatement{}
		left, hasLeft := m["left"]
		if hasLeft {
			orStatement.Left = MapToStatement(left.(map[string]interface{}))
		}

		right, hasRight := m["right"]
		if hasRight {
			orStatement.Right = MapToStatement(right.(map[string]interface{}))
		}

		return orStatement

	case hasFieldName:
		fieldName, ok := m["field_name"].(string)
		if !ok {
			fieldName = ""
		}
		return MatchStatement{
			FieldType: int(m["field_type"].(float64)),
			FieldName: fieldName,
			Operation: m["operation"].(string),
			Value:     m["value"],
		}
	}

	return nil
}
