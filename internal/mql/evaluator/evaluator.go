package evaluator

import (
	"fmt"

	"github.com/materials-commons/hydra/internal/mql/ast"
	"github.com/materials-commons/hydra/internal/mql/object"
)

var (
	NULL  = &object.Null{}
	TRUE  = &object.Boolean{Value: true}
	FALSE = &object.Boolean{Value: false}
)

func Eval(node ast.Node) object.Object {
	switch node := node.(type) {
	case *ast.MQL:
		return evalMQL(node)
	case *ast.SelectStatement:
		return evalSelectStatement(node)
	case *ast.WhereStatement:
		return evalWhereStatement(node)
	case *ast.IntegerLiteral:
		return &object.Integer{Value: node.Value}
	case *ast.FloatLiteral:
		return &object.Float{Value: node.Value}
	case *ast.StringLiteral:
		return &object.String{Value: node.Value}
	case *ast.PrefixExpression:
		right := Eval(node.Right)
		if isError(right) {
			return right
		}
		return evalPrefixExpression(node.Operator, right)
	case *ast.InfixExpression:
		left := Eval(node.Left)
		if isError(left) {
			return left
		}

		right := Eval(node.Right)
		if isError(right) {
			return right
		}

		return evalInfixExpression(node.Operator, left, right)
	}

	return nil
}

func evalMQL(mql *ast.MQL) object.Object {
	var result object.Object

	for _, statement := range mql.Statements {
		result = Eval(statement)
		switch result := result.(type) {
		case *object.ReturnValue:
			return result.Value
		case *object.Error:
			return result
		}
	}

	return result
}

func evalSelectStatement(s *ast.SelectStatement) object.Object {
	return nil
}

func evalWhereStatement(s *ast.WhereStatement) object.Object {
	return nil
}

func evalPrefixExpression(operator string, right object.Object) object.Object {
	switch operator {
	default:
		return nil
	}
}

func evalInfixExpression(operator string, left, right object.Object) object.Object {
	switch {
	case left.Type() == object.INTEGER_OBJ && right.Type() == object.INTEGER_OBJ:
		return nil
	case left.Type() == object.FLOAT_OBJ && right.Type() == object.FLOAT_OBJ:
		return nil
	case operator == "=":
		return nil
	case operator == "<>":
		return nil
	case operator == "<":
		return nil
	case operator == "<=":
		return nil
	case operator == ">":
		return nil
	case operator == ">=":
		return nil
	case left.Type() != right.Type():
		return newError("type mismatch: %s %s %s", left.Type(), operator, right.Type())
	}
	return nil
}

func isError(o object.Object) bool {
	if o != nil {
		return o.Type() == object.ERROR_OBJ
	}

	return false
}

func newError(format string, a ...interface{}) *object.Error {
	return &object.Error{Message: fmt.Sprintf(format, a...)}
}
