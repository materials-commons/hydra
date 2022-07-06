package object

import "fmt"

type BuiltinFunc func(args ...Object) Object

type ObjectType string

type Object interface {
	Type() ObjectType
	Inspect() string
}

const (
	NULL_OBJ         = "NULL"
	ERROR_OBJ        = "ERROR"
	INTEGER_OBJ      = "INTEGER"
	FLOAT_OBJ        = "FLOAT"
	BOOL_OBJ         = "BOOL"
	STRING_OBJ       = "STRING"
	BUILTIN_OBJ      = "BUILTIN"
	RETURN_VALUE_OBJ = "RETURN_VALUE"
)

type Integer struct {
	Value int64
}

func (o *Integer) Type() ObjectType {
	return INTEGER_OBJ
}

func (o *Integer) Inspect() string {
	return fmt.Sprintf("%d", o.Value)
}

//////////////////////////////

type Float struct {
	Value float64
}

func (o *Float) Type() ObjectType {
	return FLOAT_OBJ
}

func (o *Float) Inspect() string {
	return fmt.Sprintf("%f", o.Value)
}

/////////////////////////////

type Boolean struct {
	Value bool
}

func (o *Boolean) Type() ObjectType {
	return BOOL_OBJ
}

func (o *Boolean) Inspect() string {
	return fmt.Sprintf("%t", o.Value)
}

/////////////////////////////

type Null struct{}

func (o *Null) Type() ObjectType {
	return NULL_OBJ
}

func (o *Null) Inspect() string {
	return "null"
}

/////////////////////////////

type Error struct {
	Message string
}

func (o *Error) Type() ObjectType {
	return ERROR_OBJ
}

func (o *Error) Inspect() string {
	return "ERROR: " + o.Message
}

///////////////////////////////

type String struct {
	Value string
}

func (o *String) Type() ObjectType {
	return STRING_OBJ
}

func (o *String) Inspect() string {
	return o.Value
}

///////////////////////////////

type Builtin struct {
	Fn BuiltinFunc
}

func (o *Builtin) Type() ObjectType {
	return BUILTIN_OBJ
}

func (o *Builtin) Inspect() string {
	return "builtin func"
}

///////////////////////////////

type ReturnValue struct {
	Value Object
}

func (o *ReturnValue) Type() ObjectType {
	return RETURN_VALUE_OBJ
}

func (o *ReturnValue) Inspect() string {
	return o.Value.Inspect()
}
