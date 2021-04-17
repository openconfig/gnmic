package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/karimra/gnmic/collector"
	"github.com/openconfig/gnmi/proto/gnmi"
	"gopkg.in/yaml.v2"
)

type update struct {
	Path     string      `json:"path,omitempty" yaml:"path,omitempty"`
	Value    interface{} `json:"value,omitempty" yaml:"value,omitempty"`
	Encoding string      `json:"encoding,omitempty" yaml:"encoding,omitempty"`
}

type setRequestFile struct {
	Updates  []*update `json:"updates,omitempty" yaml:"updates,omitempty"`
	Replaces []*update `json:"replaces,omitempty" yaml:"replaces,omitempty"`
	Deletes  []string  `json:"deletes,omitempty" yaml:"deletes,omitempty"`
}

func (c *Config) CreateSetRequestFromFile() (*gnmi.SetRequest, error) {
	b, err := readFile(c.SetRequestFile)
	if err != nil {
		fmt.Printf("err readFile: %v\n", err)
		return nil, err
	}
	if c.Debug {
		c.logger.Printf("set request file content: %s", string(b))
	}

	reqFile := new(setRequestFile)
	err = yaml.Unmarshal(b, reqFile)
	if err != nil {
		return nil, err
	}
	sReq := &gnmi.SetRequest{
		Delete:  make([]*gnmi.Path, 0, len(reqFile.Deletes)),
		Replace: make([]*gnmi.Update, 0, len(reqFile.Replaces)),
		Update:  make([]*gnmi.Update, 0, len(reqFile.Updates)),
	}
	buf := new(bytes.Buffer)
	for _, upd := range reqFile.Updates {
		gnmiPath, err := collector.ParsePath(strings.TrimSpace(upd.Path))
		if err != nil {
			return nil, err
		}

		enc := upd.Encoding
		if enc == "" {
			enc = c.GlobalFlags.Encoding
		}
		value := new(gnmi.TypedValue)
		buf.Reset()
		err = json.NewEncoder(buf).Encode(convert(upd.Value))
		if err != nil {
			return nil, err
		}
		err = setValue(value, strings.ToLower(enc), buf.String())
		if err != nil {
			return nil, err
		}
		sReq.Update = append(sReq.Update, &gnmi.Update{
			Path: gnmiPath,
			Val:  value,
		})
	}
	for _, upd := range reqFile.Replaces {
		gnmiPath, err := collector.ParsePath(strings.TrimSpace(upd.Path))
		if err != nil {
			return nil, err
		}

		enc := upd.Encoding
		if enc == "" {
			enc = c.GlobalFlags.Encoding
		}
		value := new(gnmi.TypedValue)
		buf.Reset()
		err = json.NewEncoder(buf).Encode(convert(upd.Value))
		if err != nil {
			return nil, err
		}
		err = setValue(value, strings.ToLower(enc), buf.String())
		if err != nil {
			return nil, err
		}
		sReq.Replace = append(sReq.Replace, &gnmi.Update{
			Path: gnmiPath,
			Val:  value,
		})
	}
	for _, s := range reqFile.Deletes {
		gnmiPath, err := collector.ParsePath(strings.TrimSpace(s))
		if err != nil {
			return nil, err
		}
		sReq.Delete = append(sReq.Delete, gnmiPath)
	}
	return sReq, nil
}
