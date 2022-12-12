// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_starlark

import (
	"log"
	"os"
	"reflect"
	"testing"

	"github.com/openconfig/gnmic/formatters"
)

func Test_starlarkProc_Apply(t *testing.T) {
	type fields struct {
		cfg map[string]interface{}
	}
	type args struct {
		es []*formatters.EventMsg
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   []*formatters.EventMsg
	}{
		{
			name: "print",
			fields: fields{
				cfg: map[string]interface{}{
					"debug": true,
					"source": `
def apply(*events):
  for e in events:
    print(e)
  return events
					`,
				},
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "ev1",
						Timestamp: 42,
						Tags: map[string]string{
							"tag1": "v1",
							"tag2": "v2",
						},
						Values: map[string]interface{}{
							"val1": 42,
							"val2": "foo",
						},
					},
					{
						Name:      "ev2",
						Timestamp: 42,
						Tags: map[string]string{
							"tag1": "v1",
							"tag2": "v2",
						},
						Values: map[string]interface{}{
							"val1": 42,
							"val2": "foo",
						},
					},
				},
			},
			want: []*formatters.EventMsg{
				{
					Name:      "ev1",
					Timestamp: 42,
					Tags: map[string]string{
						"tag1": "v1",
						"tag2": "v2",
					},
					Values: map[string]interface{}{
						"val1": 42,
						"val2": "foo",
					},
				},
				{
					Name:      "ev2",
					Timestamp: 42,
					Tags: map[string]string{
						"tag1": "v1",
						"tag2": "v2",
					},
					Values: map[string]interface{}{
						"val1": 42,
						"val2": "foo",
					},
				},
			},
		},
		{
			name: "add_tag",
			fields: fields{
				cfg: map[string]interface{}{
					"debug": true,
					"source": `
def apply(*events):
  for e in events:
    e.tags["new_tag"] = "new_tag"
  return events
					`,
				},
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "ev1",
						Timestamp: 42,
						Tags: map[string]string{
							"tag1": "v1",
							"tag2": "v2",
						},
						Values: map[string]interface{}{
							"val1": 42,
							"val2": "foo",
						},
					},
					{
						Name:      "ev2",
						Timestamp: 42,
						Tags: map[string]string{
							"tag1": "v1",
							"tag2": "v2",
						},
						Values: map[string]interface{}{
							"val1": 42,
							"val2": "foo",
						},
					},
				},
			},
			want: []*formatters.EventMsg{
				{
					Name:      "ev1",
					Timestamp: 42,
					Tags: map[string]string{
						"tag1":    "v1",
						"tag2":    "v2",
						"new_tag": "new_tag",
					},
					Values: map[string]interface{}{
						"val1": 42,
						"val2": "foo",
					},
				},
				{
					Name:      "ev2",
					Timestamp: 42,
					Tags: map[string]string{
						"tag1":    "v1",
						"tag2":    "v2",
						"new_tag": "new_tag",
					},
					Values: map[string]interface{}{
						"val1": 42,
						"val2": "foo",
					},
				},
			},
		},
		{
			name: "delete_tag",
			fields: fields{
				cfg: map[string]interface{}{
					"debug": true,
					"source": `
def apply(*events):
  for e in events:
    e.tags.pop("tag1")
  return events
					`,
				},
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "ev1",
						Timestamp: 42,
						Tags: map[string]string{
							"tag1": "v1",
							"tag2": "v2",
						},
						Values: map[string]interface{}{
							"val1": 42,
							"val2": "foo",
						},
					},
					{
						Name:      "ev2",
						Timestamp: 42,
						Tags: map[string]string{
							"tag1": "v1",
							"tag2": "v2",
						},
						Values: map[string]interface{}{
							"val1": 42,
							"val2": "foo",
						},
					},
				},
			},
			want: []*formatters.EventMsg{
				{
					Name:      "ev1",
					Timestamp: 42,
					Tags: map[string]string{
						"tag2": "v2",
					},
					Values: map[string]interface{}{
						"val1": 42,
						"val2": "foo",
					},
				},
				{
					Name:      "ev2",
					Timestamp: 42,
					Tags: map[string]string{
						"tag2": "v2",
					},
					Values: map[string]interface{}{
						"val1": 42,
						"val2": "foo",
					},
				},
			},
		},
		{
			name: "add_value",
			fields: fields{
				cfg: map[string]interface{}{
					"debug": true,
					"source": `
def apply(*events):
  for e in events:
    e.values["new_val"] = "val"
  return events
					`,
				},
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "ev1",
						Timestamp: 42,
						Tags: map[string]string{
							"tag1": "v1",
							"tag2": "v2",
						},
						Values: map[string]interface{}{
							"val1": 42,
							"val2": "foo",
						},
					},
					{
						Name:      "ev2",
						Timestamp: 42,
						Tags: map[string]string{
							"tag1": "v1",
							"tag2": "v2",
						},
						Values: map[string]interface{}{
							"val1": 42,
							"val2": "foo",
						},
					},
				},
			},
			want: []*formatters.EventMsg{
				{
					Name:      "ev1",
					Timestamp: 42,
					Tags: map[string]string{
						"tag1": "v1",
						"tag2": "v2",
					},
					Values: map[string]interface{}{
						"val1":    42,
						"val2":    "foo",
						"new_val": "val",
					},
				},
				{
					Name:      "ev2",
					Timestamp: 42,
					Tags: map[string]string{
						"tag1": "v1",
						"tag2": "v2",
					},
					Values: map[string]interface{}{
						"val1":    42,
						"val2":    "foo",
						"new_val": "val",
					},
				},
			},
		},
		{
			name: "delete_val",
			fields: fields{
				cfg: map[string]interface{}{
					"debug": true,
					"source": `
def apply(*events):
  for e in events:
    e.values.pop("val1")
  return events
					`,
				},
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "ev1",
						Timestamp: 42,
						Tags: map[string]string{
							"tag1": "v1",
							"tag2": "v2",
						},
						Values: map[string]interface{}{
							"val1": 42,
							"val2": "foo",
						},
					},
					{
						Name:      "ev2",
						Timestamp: 42,
						Tags: map[string]string{
							"tag1": "v1",
							"tag2": "v2",
						},
						Values: map[string]interface{}{
							"val2": "foo",
						},
					},
				},
			},
			want: []*formatters.EventMsg{
				{
					Name:      "ev1",
					Timestamp: 42,
					Tags: map[string]string{
						"tag1": "v1",
						"tag2": "v2",
					},
					Values: map[string]interface{}{
						"val2": "foo",
					},
				},
				{
					Name:      "ev2",
					Timestamp: 42,
					Tags: map[string]string{
						"tag1": "v1",
						"tag2": "v2",
					},
					Values: map[string]interface{}{
						"val2": "foo",
					},
				},
			},
		},
		{
			name: "insert_event",
			fields: fields{
				cfg: map[string]interface{}{
					"debug": true,
					"source": `
def apply(*events):
  evs = []
  for e in events:
    evs.append(e)
  ne = Event("new_event")
  ne.tags["tag1"] = "tag1"
  evs.append(ne)
  return evs
					`,
				},
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "ev1",
						Timestamp: 42,
						Tags: map[string]string{
							"tag1": "v1",
							"tag2": "v2",
						},
						Values: map[string]interface{}{
							"val1": 42,
							"val2": "foo",
						},
					},
					{
						Name:      "ev2",
						Timestamp: 42,
						Tags: map[string]string{
							"tag1": "v1",
							"tag2": "v2",
						},
						Values: map[string]interface{}{
							"val1": 42,
							"val2": "foo",
						},
					},
				},
			},
			want: []*formatters.EventMsg{
				{
					Name:      "ev1",
					Timestamp: 42,
					Tags: map[string]string{
						"tag1": "v1",
						"tag2": "v2",
					},
					Values: map[string]interface{}{
						"val1": 42,
						"val2": "foo",
					},
				},
				{
					Name:      "ev2",
					Timestamp: 42,
					Tags: map[string]string{
						"tag1": "v1",
						"tag2": "v2",
					},
					Values: map[string]interface{}{
						"val1": 42,
						"val2": "foo",
					},
				},
				{
					Name: "new_event",
					Tags: map[string]string{
						"tag1": "tag1",
					},
					Values: map[string]interface{}{},
				},
			},
		},
		{
			name: "use_cache",
			fields: fields{
				cfg: map[string]interface{}{
					"debug": true,
					"source": `
cache = {}
def apply(*events):
  evs = []
  for e in events:
	target_if = e.tags["target"] + "_" + e.tags["interface_name"]
	if e.values.get("description"):
	  cache[target_if] = e.values["description"]
	  continue
	e.tags["description"] = cache[target_if]
	evs.append(e)
  return evs
					`,
				},
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "ev1",
						Timestamp: 42,
						Tags: map[string]string{
							"target":         "router1",
							"interface_name": "if1",
						},
						Values: map[string]interface{}{
							"description": "foo",
						},
					},
					{
						Name:      "ev2",
						Timestamp: 42,
						Tags: map[string]string{
							"target":         "router1",
							"interface_name": "if1",
						},
						Values: map[string]interface{}{
							"val1": 42,
						},
					},
				},
			},
			want: []*formatters.EventMsg{
				{
					Name:      "ev2",
					Timestamp: 42,
					Tags: map[string]string{
						"target":         "router1",
						"interface_name": "if1",
						"description":    "foo",
					},
					Values: map[string]interface{}{
						"val1": 42,
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &starlarkProc{}
			err := p.Init(tt.fields.cfg, formatters.WithLogger(log.New(os.Stderr, "test", log.Default().Flags())))
			if err != nil {
				t.Errorf("%q failed to init processor: %v", tt.name, err)
				t.Fail()
			}
			got := p.Apply(tt.args.es...)
			t.Logf("got : %v", got)
			t.Logf("want: %v", tt.want)
			// compare lengths first
			if len(got) != len(tt.want) {
				t.Logf("expected and gotten outputs are not of the same length")
				t.Logf("expected: %+v", tt.want)
				t.Logf("     got: %+v", got)
				t.Fail()
			}
			//
			for j := range got {
				t.Logf("%q index %d, output=%+v", tt.name, j, got[j])
				if !reflect.DeepEqual(got[j].Values, tt.want[j].Values) {
					t.Logf("failed at %s index %d, values are different", tt.name, j)
					t.Logf("expected: %+v", tt.want[j])
					t.Logf("     got: %+v", got[j])
					t.Fail()
				}
				if !reflect.DeepEqual(got[j].Tags, tt.want[j].Tags) {
					t.Logf("failed at %s index %d, tags are different", tt.name, j)
					t.Logf("expected: %+v", tt.want[j])
					t.Logf("     got: %+v", got[j])
					t.Fail()
				}
				if !reflect.DeepEqual(got[j].Name, tt.want[j].Name) {
					t.Logf("failed at %s index %d, names are different", tt.name, j)
					t.Logf("expected: %+v", tt.want[j])
					t.Logf("     got: %+v", got[j])
					t.Fail()
				}
				if !reflect.DeepEqual(got[j].Timestamp, tt.want[j].Timestamp) {
					t.Logf("failed at %s index %d, timestamps are different", tt.name, j)
					t.Logf("expected: %+v", tt.want[j])
					t.Logf("     got: %+v", got[j])
					t.Fail()
				}
			}
		})
	}
}
