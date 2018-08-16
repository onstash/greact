package testutil

import (
	"github.com/gernest/vected/vdom/value"
)

type CallbackHandle func([]value.Value)

func (CallbackHandle) Release() {}

type Object struct {
	name      string
	namespace string
	props     map[string]*Object
	value     interface{}
	typ       value.Type
}

func (o *Object) Bool() bool {
	return o.value.(bool)
}

func (o *Object) Float() float64 {
	return o.value.(float64)
}

func (o *Object) Int() int {
	return o.value.(int)
}
func (o *Object) String() string {
	return o.value.(string)
}

func (o *Object) Type() value.Type {
	return o.typ
}

func (o *Object) Set(k string, v interface{}) {
	if o.props == nil {
		o.props = make(map[string]*Object)
	}
	switch e := v.(type) {
	case bool:
		o.props[k] = &Object{typ: value.TypeBoolean, value: e}
	case string:
		o.props[k] = &Object{typ: value.TypeString, value: e}
	case float64:
		o.props[k] = &Object{typ: value.TypeNumber, value: e}
	case nil:
		o.props[k] = &Object{typ: value.TypeNull, value: e}
	case value.Value:
		o.props[k] = &Object{typ: value.TypeObject, value: e}
	}
}

func (o *Object) Get(k string) value.Value {
	if m, ok := o.props[k]; ok {
		return m
	}
	return &Object{typ: value.TypeUndefined}
}

func (o *Object) Call(k string, args ...interface{}) value.Value {
	return &Object{typ: value.TypeUndefined}
}

func (o *Object) Index(n int) value.Value {
	if v, ok := o.value.([]interface{}); ok {
		return &Object{value: v[n]}
	}
	return &Object{typ: value.TypeUndefined}
}

func (o *Object) Invoke(args ...interface{}) value.Value {
	return &Object{typ: value.TypeUndefined}
}
