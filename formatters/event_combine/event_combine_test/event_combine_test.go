// © 2023 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_sequence

import (
	"reflect"
	"testing"

	"github.com/openconfig/gnmic/formatters"
	_ "github.com/openconfig/gnmic/formatters/all"
)

func Test_combine_Apply(t *testing.T) {
	type fields struct {
		processorConfig map[string]any
		processorsSet   map[string]map[string]any
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
			name: "simple1",
			fields: fields{
				processorConfig: map[string]any{
					"debug": true,
					"processors": []any{
						map[string]any{
							"condition": ".tags.tag == \"t1\"",
							"name":      "proc1",
						},
						map[string]any{
							"name": "proc2",
						},
					},
				},
				processorsSet: map[string]map[string]any{
					"proc1": {
						"event-strings": map[string]any{
							"value-names": []string{"^number$"},
							"transforms": []map[string]any{
								{
									"replace": map[string]any{
										"apply-on": "name",
										"old":      "number",
										"new":      "new_number",
									},
								},
							},
							"debug": true,
						},
					},
					"proc2": {
						"event-strings": map[string]any{
							"tag-names": []string{"^tag$"},
							"transforms": []map[string]any{
								{
									"replace": map[string]any{
										"apply-on": "name",
										"old":      "tag",
										"new":      "new_tag",
									},
								},
							},
							"debug": true,
						},
					},
				},
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Tags:   map[string]string{"tag": "t1"},
						Values: map[string]interface{}{"number": "42"},
					},
					{
						Tags:   map[string]string{"t": "t1"},
						Values: map[string]interface{}{"n": "42"},
					},
				},
			},
			want: []*formatters.EventMsg{
				{
					Tags:   map[string]string{"new_tag": "t1"},
					Values: map[string]interface{}{"new_number": "42"},
				},
				{
					Tags:   map[string]string{"t": "t1"},
					Values: map[string]interface{}{"n": "42"},
				},
			},
		},
		{
			name: "simple2",
			fields: fields{
				processorConfig: map[string]any{
					"debug": true,
					"processors": []any{
						map[string]any{
							"condition": ".tags.tag == \"t2\"",
							"name":      "proc1",
						},
						map[string]any{
							"name": "proc2",
						},
					},
				},
				processorsSet: map[string]map[string]any{
					"proc1": {
						"event-strings": map[string]any{
							"value-names": []string{"^number$"},
							"transforms": []map[string]any{
								{
									"replace": map[string]any{
										"apply-on": "name",
										"old":      "number",
										"new":      "new_number",
									},
								},
							},
							"debug": true,
						},
					},
					"proc2": {
						"event-strings": map[string]any{
							"tag-names": []string{"^tag$"},
							"transforms": []map[string]any{
								{
									"replace": map[string]any{
										"apply-on": "name",
										"old":      "tag",
										"new":      "new_tag",
									},
								},
							},
							"debug": true,
						},
					},
				},
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Tags:   map[string]string{"tag": "t1"},
						Values: map[string]interface{}{"number": "42"},
					},
					{
						Tags:   map[string]string{"t": "t1"},
						Values: map[string]interface{}{"n": "42"},
					},
				},
			},
			want: []*formatters.EventMsg{
				{
					Tags:   map[string]string{"new_tag": "t1"},
					Values: map[string]interface{}{"number": "42"},
				},
				{
					Tags:   map[string]string{"t": "t1"},
					Values: map[string]interface{}{"n": "42"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := formatters.EventProcessors["event-combine"]
			p := in()
			err := p.Init(tt.fields.processorConfig, formatters.WithProcessors(tt.fields.processorsSet))
			if err != nil {
				t.Logf("%s failed to init the processor: %v", tt.name, err)
				t.Fail()
			}
			if got := p.Apply(tt.args.es...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("combine.Apply() = %v, want %v", got, tt.want)
			}
		})
	}
}
