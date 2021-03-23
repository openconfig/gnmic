package config

import (
	"text/template"
)

var tplFunc = template.FuncMap{
	"select": func(in interface{}, items ...string) interface{} {
		items = append([]string{"updates", "values"}, items...)
		acc := new(acc)
		acc.traverse(in, items...)
		return acc.l
	},
}

type acc struct {
	l []interface{}
}

func (a *acc) append(i interface{}) {
	if a.l == nil {
		a.l = make([]interface{}, 0)
	}
	a.l = append(a.l, i)
}
func (a *acc) traverse(input interface{}, items ...string) {
	if len(items) == 0 {
		a.append(input)
		return
	}
	switch input := input.(type) {
	case []interface{}:
		for _, i := range input {
			a.traverse(i, items...)
			return
		}
	case map[string]interface{}:
		if i, ok := input[items[0]]; ok {
			a.traverse(i, items[1:]...)
			return
		}
	default:
		a.append(input)
	}
}
