// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"google.golang.org/protobuf/encoding/prototext"
	"gopkg.in/yaml.v2"

	"github.com/openconfig/gnmi/proto/gnmi"

	"github.com/openconfig/gnmic/pkg/api"
	gfile "github.com/openconfig/gnmic/pkg/file"
	"github.com/openconfig/gnmic/pkg/gtemplate"
)

const (
	varFileSuffix = "_vars"
)

type UpdateItem struct {
	Path     string      `json:"path,omitempty" yaml:"path,omitempty"`
	Value    interface{} `json:"value,omitempty" yaml:"value,omitempty"`
	Encoding string      `json:"encoding,omitempty" yaml:"encoding,omitempty"`
}

type SetRequestFile struct {
	Updates          []*UpdateItem `json:"updates,omitempty" yaml:"updates,omitempty"`
	Replaces         []*UpdateItem `json:"replaces,omitempty" yaml:"replaces,omitempty"`
	UnionReplaces    []*UpdateItem `json:"union-replaces,omitempty" yaml:"union-replaces,omitempty"`
	Deletes          []string      `json:"deletes,omitempty" yaml:"deletes,omitempty"`
	CommitID         string        `yaml:"commit-id,omitempty" json:"commit-id,omitempty"`
	CommitAction     commitAction  `yaml:"commit-action,omitempty" json:"commit-action,omitempty"`
	RollbackDuration time.Duration `yaml:"rollback-duration,omitempty" json:"rollback-duration,omitempty"`
}

type commitAction string

const (
	commitActionRequest             commitAction = "request"
	commitActionCancel              commitAction = "cancel"
	commitActionConfirm             commitAction = "confirm"
	commitActionSetRollbackDuration commitAction = "set-rollback-duration"
)

func (c *Config) ReadSetRequestTemplate() error {
	if len(c.SetRequestFile) == 0 {
		return nil
	}
	c.setRequestTemplate = make([]*template.Template, len(c.SetRequestFile))
	for i, srf := range c.SetRequestFile {
		b, err := gfile.ReadFile(context.TODO(), srf)
		if err != nil {
			return err
		}
		if c.Debug {
			c.logger.Printf("set request file %d content: %s", i, string(b))
		}
		// read template
		c.setRequestTemplate[i], err = gtemplate.CreateTemplate(fmt.Sprintf("set-request-%d", i), string(b))
		if err != nil {
			return err
		}
	}
	return c.readTemplateVarsFile()
}

func (c *Config) readTemplateVarsFile() error {
	if c.SetRequestVars == "" {
		ext := filepath.Ext(c.SetRequestFile[0])
		c.SetRequestVars = fmt.Sprintf("%s%s%s", c.SetRequestFile[0][0:len(c.SetRequestFile[0])-len(ext)], varFileSuffix, ext)
		c.logger.Printf("trying to find variable file %q", c.SetRequestVars)
		_, err := os.Stat(c.SetRequestVars)
		if os.IsNotExist(err) {
			c.SetRequestVars = ""
			return nil
		} else if err != nil {
			return err
		}
	}
	b, err := readFile(c.SetRequestVars)
	if err != nil {
		return err
	}
	if c.setRequestVars == nil {
		c.setRequestVars = make(map[string]interface{})
	}
	err = yaml.Unmarshal(b, &c.setRequestVars)
	if err != nil {
		return err
	}
	tempInterface := convert(c.setRequestVars)
	switch t := tempInterface.(type) {
	case map[string]interface{}:
		c.setRequestVars = t
	default:
		return errors.New("unexpected variables file format")
	}
	if c.Debug {
		c.logger.Printf("request vars content: %v", c.setRequestVars)
	}
	return nil
}

func (c *Config) CreateSetRequestFromFile(targetName string) ([]*gnmi.SetRequest, error) {
	if len(c.setRequestTemplate) == 0 {
		return nil, errors.New("missing set request template")
	}
	reqs := make([]*gnmi.SetRequest, 0, len(c.setRequestTemplate))
	buf := new(bytes.Buffer)
	for _, srf := range c.setRequestTemplate {
		buf.Reset()
		err := srf.Execute(buf, templateInput{
			TargetName: targetName,
			Vars:       c.setRequestVars,
		})
		if err != nil {
			return nil, err
		}
		if c.Debug {
			c.logger.Printf("target %q template result:\n%s", targetName, buf.String())
		}
		//
		reqFile := new(SetRequestFile)
		err = yaml.Unmarshal(buf.Bytes(), reqFile)
		if err != nil {
			return nil, err
		}
		gnmiOpts := make([]api.GNMIOption, 0)
		buf.Reset()
		for _, upd := range reqFile.Updates {
			if upd.Path == "" {
				upd.Path = "/"
			}

			enc := upd.Encoding
			if enc == "" {
				enc = c.GlobalFlags.Encoding
			}
			buf.Reset()
			switch {
			case strings.HasPrefix(upd.Path, "cli:/"):
				val, ok := upd.Value.(string)
				if !ok {
					return nil, fmt.Errorf("value %v is not a string", upd.Value)
				}
				buf.WriteString(val)
			default:
				err = json.NewEncoder(buf).Encode(convert(upd.Value))
				if err != nil {
					return nil, err
				}
			}

			gnmiOpts = append(gnmiOpts,
				api.Update(
					api.Path(strings.TrimSpace(upd.Path)),
					api.Value(strings.TrimSpace(buf.String()), enc),
				),
			)
		}
		for _, upd := range reqFile.Replaces {
			if upd.Path == "" {
				upd.Path = "/"
			}
			enc := upd.Encoding
			if enc == "" {
				enc = c.GlobalFlags.Encoding
			}
			buf.Reset()
			switch {
			case upd.Path == "cli:/":
				val, ok := upd.Value.(string)
				if !ok {
					return nil, fmt.Errorf("value %v is not a string", upd.Value)
				}
				buf.WriteString(val)
			default:
				err = json.NewEncoder(buf).Encode(convert(upd.Value))
				if err != nil {
					return nil, err
				}
			}
			gnmiOpts = append(gnmiOpts, api.Replace(
				api.Path(strings.TrimSpace(upd.Path)),
				api.Value(strings.TrimSpace(buf.String()), enc),
			),
			)
		}
		for _, upd := range reqFile.UnionReplaces {
			if upd.Path == "" {
				upd.Path = "/"
			}
			enc := upd.Encoding
			if enc == "" {
				enc = c.GlobalFlags.Encoding
			}
			buf.Reset()
			switch {
			case upd.Path == "cli:/":
				val, ok := upd.Value.(string)
				if !ok {
					return nil, fmt.Errorf("value %v is not a string", upd.Value)
				}
				buf.WriteString(val)
			default:
				err = json.NewEncoder(buf).Encode(convert(upd.Value))
				if err != nil {
					return nil, err
				}
			}
			gnmiOpts = append(gnmiOpts, api.UnionReplace(
				api.Path(strings.TrimSpace(upd.Path)),
				api.Value(strings.TrimSpace(buf.String()), enc),
			),
			)
		}

		for _, s := range reqFile.Deletes {
			gnmiOpts = append(gnmiOpts, api.Delete(strings.TrimSpace(s)))
		}

		if reqFile.CommitID != "" {
			switch reqFile.CommitAction {
			case commitActionRequest:
				gnmiOpts = append(gnmiOpts,
					api.Extension_CommitRequest(
						c.LocalFlags.SetCommitId,
						c.LocalFlags.SetCommitRollbackDuration,
					))
			case commitActionCancel:
				gnmiOpts = append(gnmiOpts,
					api.Extension_CommitCancel(
						c.LocalFlags.SetCommitId,
					))
			case commitActionConfirm:
				gnmiOpts = append(gnmiOpts,
					api.Extension_CommitConfirm(
						c.LocalFlags.SetCommitId,
					))
			case commitActionSetRollbackDuration:
				gnmiOpts = append(gnmiOpts,
					api.Extension_CommitSetRollbackDuration(
						c.LocalFlags.SetCommitId,
						c.LocalFlags.SetCommitRollbackDuration,
					))
			default:
				return nil, fmt.Errorf("unknown commit action %s", reqFile.CommitAction)
			}
		}

		setReq, err := api.NewSetRequest(gnmiOpts...)
		if err != nil {
			return nil, err
		}
		reqs = append(reqs, setReq)
	}
	return reqs, nil
}

type templateInput struct {
	TargetName string
	Vars       map[string]interface{}
}

func (c *Config) CreateSetRequestFromProtoFile() ([]*gnmi.SetRequest, error) {
	reqs := make([]*gnmi.SetRequest, 0, len(c.SetRequestProtoFile))
	for _, r := range c.SetRequestProtoFile {
		b, err := os.ReadFile(r)
		if err != nil {
			return nil, err
		}
		req := new(gnmi.SetRequest)
		err = prototext.Unmarshal(b, req)
		if err != nil {
			return nil, err
		}
		reqs = append(reqs, req)
	}
	return reqs, nil
}
