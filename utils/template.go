package utils

import (
	"context"
	"text/template"

	"github.com/hairyhenderson/gomplate/v3"
	"github.com/hairyhenderson/gomplate/v3/data"
)

func CreateTemplate(name, text string) (*template.Template, error) {
	return template.New(name).
		Option("missingkey=zero").
		Funcs(gomplate.CreateFuncs(context.TODO(), new(data.Data))).
		Parse(text)
}
