// © 2025 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package event_time_epoch

import (
	"log"
	"os"
	"reflect"
	"testing"

	"github.com/openconfig/gnmic/pkg/formatters"
)

func Test_epoch_Apply(t *testing.T) {
	type fields map[string]any
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
			name: "simple",
			fields: map[string]any{
				"precision": "s",
				"value-names": []string{
					".*last-change",
				},
				"debug": true,
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "sub1",
						Timestamp: 42,
						Tags:      map[string]string{},
						Values: map[string]any{
							"interface/last-change": "2024-06-19T15:11:24.601Z",
						},
					},
				},
			},
			want: []*formatters.EventMsg{
				{
					Name:      "sub1",
					Timestamp: 42,
					Tags:      map[string]string{},
					Values: map[string]any{
						"interface/last-change": int64(1718809884),
					},
				},
			},
		},
		{
			name: "ms",
			fields: map[string]any{
				"precision": "ms",
				"value-names": []string{
					".*last-change",
				},
				"debug": true,
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "sub1",
						Timestamp: 42,
						Tags:      map[string]string{},
						Values: map[string]any{
							"interface/last-change": "2024-06-19T15:11:24.601Z",
						},
					},
				},
			},
			want: []*formatters.EventMsg{
				{
					Name:      "sub1",
					Timestamp: 42,
					Tags:      map[string]string{},
					Values: map[string]any{
						"interface/last-change": int64(1718809884601),
					},
				},
			},
		},
		{
			name: "us",
			fields: map[string]any{
				"precision": "us",
				"value-names": []string{
					".*last-change",
				},
				"debug": true,
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "sub1",
						Timestamp: 42,
						Tags:      map[string]string{},
						Values: map[string]any{
							"interface/last-change": "2024-06-19T15:11:24.601Z",
						},
					},
				},
			},
			want: []*formatters.EventMsg{
				{
					Name:      "sub1",
					Timestamp: 42,
					Tags:      map[string]string{},
					Values: map[string]any{
						"interface/last-change": int64(1718809884601000),
					},
				},
			},
		},
		{
			name: "ns",
			fields: map[string]any{
				"precision": "ns",
				"value-names": []string{
					".*last-change",
				},
				"debug": true,
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "sub1",
						Timestamp: 42,
						Tags:      map[string]string{},
						Values: map[string]any{
							"interface/last-change": "2024-06-19T15:11:24.601Z",
						},
					},
				},
			},
			want: []*formatters.EventMsg{
				{
					Name:      "sub1",
					Timestamp: 42,
					Tags:      map[string]string{},
					Values: map[string]any{
						"interface/last-change": int64(1718809884601000000),
					},
				},
			},
		},
		{
			name: "no_match",
			fields: map[string]any{
				"precision": "ns",
				"value-names": []string{
					".*no_match.*",
				},
				"debug": true,
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "sub1",
						Timestamp: 42,
						Tags:      map[string]string{},
						Values: map[string]any{
							"interface/last-change": "2024-06-19T15:11:24.601Z",
						},
					},
				},
			},
			want: []*formatters.EventMsg{
				{
					Name:      "sub1",
					Timestamp: 42,
					Tags:      map[string]string{},
					Values: map[string]any{
						"interface/last-change": "2024-06-19T15:11:24.601Z",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &epoch{}
			err := c.Init(tt.fields, formatters.WithLogger(log.New(os.Stderr, "[event-epoch-test]", log.Flags())))
			if err != nil {
				t.Errorf("failed to init processor in test %q: %v", tt.name, err)
				t.Fail()
			}
			if got := c.Apply(tt.args.es...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("epoch.Apply() = %v, want %v", got, tt.want)
			}
		})
	}
}
