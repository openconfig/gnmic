// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_data_convert

import (
	"log"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/gnmic/formatters"
)

func Test_dataConvert_Apply(t *testing.T) {
	type fields map[string]interface{}
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
			name: "nil_input",
			fields: map[string]interface{}{
				"value-names": []string{
					".*",
				},
				"debug": true,
			},
			args: args{},
			want: nil,
		},
		{
			name: "one_msg_bytes",
			fields: map[string]interface{}{
				"value-names": []string{
					"_total$",
				},
				"to":    "KB",
				"debug": true,
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "sub1",
						Timestamp: 42,
						Tags:      map[string]string{},
						Values: map[string]interface{}{
							"data_total": 1024,
						},
					},
				},
			},
			want: []*formatters.EventMsg{
				{
					Name:      "sub1",
					Timestamp: 42,
					Tags:      map[string]string{},
					Values: map[string]interface{}{
						"data_total": float64(1),
					},
				},
			},
		},
		{
			name: "one_msg_bytes_keep",
			fields: map[string]interface{}{
				"value-names": []string{
					"_total$",
				},
				"to":    "KB",
				"keep":  true,
				"debug": true,
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "sub1",
						Timestamp: 42,
						Tags:      map[string]string{},
						Values: map[string]interface{}{
							"data_total": 1024,
						},
					},
				},
			},
			want: []*formatters.EventMsg{
				{
					Name:      "sub1",
					Timestamp: 42,
					Tags:      map[string]string{},
					Values: map[string]interface{}{
						"data_total":    1024,
						"data_total_KB": float64(1),
					},
				},
			},
		},
		{
			name: "one_msg_bytes_from",
			fields: map[string]interface{}{
				"value-names": []string{
					"_total$",
				},
				"from":  "KB",
				"to":    "B",
				"debug": true,
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "sub1",
						Timestamp: 42,
						Tags:      map[string]string{},
						Values: map[string]interface{}{
							"data_total": 1,
						},
					},
				},
			},
			want: []*formatters.EventMsg{
				{
					Name:      "sub1",
					Timestamp: 42,
					Tags:      map[string]string{},
					Values: map[string]interface{}{
						"data_total": float64(1024),
					},
				},
			},
		},
		{
			name: "one_msg_multiple_values",
			fields: map[string]interface{}{
				"value-names": []string{
					"_total$",
				},
				"from":  "KB",
				"to":    "B",
				"debug": true,
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "sub1",
						Timestamp: 42,
						Tags:      map[string]string{},
						Values: map[string]interface{}{
							"data_total":  1,
							"bytes_total": 2,
						},
					},
				},
			},
			want: []*formatters.EventMsg{
				{
					Name:      "sub1",
					Timestamp: 42,
					Tags:      map[string]string{},
					Values: map[string]interface{}{
						"data_total":  float64(1024),
						"bytes_total": float64(2048),
					},
				},
			},
		},
		{
			name: "two_messages",
			fields: map[string]interface{}{
				"value-names": []string{
					"_total$",
				},
				"from":  "KB",
				"to":    "B",
				"debug": true,
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "sub1",
						Timestamp: 42,
						Tags:      map[string]string{},
						Values: map[string]interface{}{
							"data_total":  1,
							"bytes_total": 2,
						},
					},
					{
						Name:      "sub2",
						Timestamp: 42,
						Tags:      map[string]string{},
						Values: map[string]interface{}{
							"data_total":  1,
							"bytes_total": 2,
						},
					},
				},
			},
			want: []*formatters.EventMsg{
				{
					Name:      "sub1",
					Timestamp: 42,
					Tags:      map[string]string{},
					Values: map[string]interface{}{
						"data_total":  float64(1024),
						"bytes_total": float64(2048),
					},
				},
				{
					Name:      "sub2",
					Timestamp: 42,
					Tags:      map[string]string{},
					Values: map[string]interface{}{
						"data_total":  float64(1024),
						"bytes_total": float64(2048),
					},
				},
			},
		},
		{
			name: "string_value_with_unit",
			fields: map[string]interface{}{
				"value-names": []string{
					"_total$",
				},
				"to":    "B",
				"debug": true,
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "sub1",
						Timestamp: 42,
						Tags:      map[string]string{},
						Values: map[string]interface{}{
							"data_total":  "1 KB",
							"bytes_total": "2KB",
						},
					},
				},
			},
			want: []*formatters.EventMsg{
				{
					Name:      "sub1",
					Timestamp: 42,
					Tags:      map[string]string{},
					Values: map[string]interface{}{
						"data_total":  float64(1024),
						"bytes_total": float64(2048),
					},
				},
			},
		},
		{
			name: "one_msg_rename",
			fields: map[string]interface{}{
				"value-names": []string{
					"_total$",
				},
				"to":    "KB",
				"old":   `^(bytes)(\S+)`,
				"new":   "kilobytes${2}",
				"debug": true,
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "sub1",
						Timestamp: 42,
						Tags:      map[string]string{},
						Values: map[string]interface{}{
							"bytes_total": 1024,
						},
					},
				},
			},
			want: []*formatters.EventMsg{
				{
					Name:      "sub1",
					Timestamp: 42,
					Tags:      map[string]string{},
					Values: map[string]interface{}{
						"kilobytes_total": float64(1),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &dataConvert{}
			err := c.Init(tt.fields, formatters.WithLogger(log.New(os.Stderr, "[event-data-convert-test]", log.Flags())))
			if err != nil {
				t.Errorf("failed to init processor in test %q: %v", tt.name, err)
				t.Fail()
			}
			if got := c.Apply(tt.args.es...); !cmp.Equal(got, tt.want) {
				t.Errorf("got : %+v", got)
				t.Errorf("want: %+v", tt.want)
				t.Errorf("dataConvert.Apply() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
