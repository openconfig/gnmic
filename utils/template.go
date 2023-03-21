// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"path"
	"text/template"
)

func CreateTemplate(name, text string) (*template.Template, error) {
	return template.New(name).
		Option("missingkey=zero").
		Funcs(NewTemplateEngine().CreateFuncs()).
		Parse(text)
}

func CreateFileTemplate(filename string) (*template.Template, error) {
	name := path.Base(filename)

	tpl, err := template.New(name).
		Funcs(NewTemplateEngine().CreateFuncs()).
		ParseFiles(filename)

	template.Must(tpl, err)

	return tpl, err
}
