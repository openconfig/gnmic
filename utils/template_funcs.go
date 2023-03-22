package utils

import (
	"context"
	"text/template"

	"github.com/hairyhenderson/gomplate/v3"
	"github.com/hairyhenderson/gomplate/v3/data"
)

type templateEngine interface {
	CreateFuncs() template.FuncMap
}

func NewTemplateEngine() templateEngine {
	return &gmplt{}
}

type gmplt struct{}

func (*gmplt) CreateFuncs() template.FuncMap {
	return gomplate.CreateFuncs(context.TODO(), new(data.Data))
}
