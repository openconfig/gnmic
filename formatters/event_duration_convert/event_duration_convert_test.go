package event_data_convert

import (
	"log"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/karimra/gnmic/formatters"
)

const (
	oneMins = int64(60)
	oneHs   = int64(60 * 60)
	oneDs   = int64(24 * 60 * 60)
	oneWs   = int64(7 * 24 * 60 * 60)
)

func Test_durationConvert_Apply(t *testing.T) {
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
			name: "week",
			fields: map[string]interface{}{
				"value-names": []string{
					".*uptime",
				},
				"debug": true,
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "sub1",
						Timestamp: 42,
						Tags:      map[string]string{},
						Values: map[string]interface{}{
							"connection_uptime": "1w",
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
						"connection_uptime": oneWs,
					},
				},
			},
		},
		{
			name: "week_day",
			fields: map[string]interface{}{
				"value-names": []string{
					".*uptime",
				},
				"debug": true,
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "sub1",
						Timestamp: 42,
						Tags:      map[string]string{},
						Values: map[string]interface{}{
							"connection_uptime": "1w2d",
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
						"connection_uptime": oneWs + 2*oneDs,
					},
				},
			},
		},
		{
			name: "week_day_hour",
			fields: map[string]interface{}{
				"value-names": []string{
					".*uptime",
				},
				"debug": true,
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "sub1",
						Timestamp: 42,
						Tags:      map[string]string{},
						Values: map[string]interface{}{
							"connection_uptime": "1w2d3h",
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
						"connection_uptime": oneWs + 2*oneDs + 3*oneHs,
					},
				},
			},
		},
		{
			name: "week_day_hour_minute",
			fields: map[string]interface{}{
				"value-names": []string{
					".*uptime",
				},
				"debug": true,
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "sub1",
						Timestamp: 42,
						Tags:      map[string]string{},
						Values: map[string]interface{}{
							"connection_uptime": "1w2d3h4m",
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
						"connection_uptime": oneWs + 2*oneDs + 3*oneHs + 4*oneMins,
					},
				},
			},
		},
		{
			name: "week_day_hour_minute_second",
			fields: map[string]interface{}{
				"value-names": []string{
					".*uptime",
				},
				"debug": true,
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "sub1",
						Timestamp: 42,
						Tags:      map[string]string{},
						Values: map[string]interface{}{
							"connection_uptime": "1w2d3h4m5s",
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
						"connection_uptime": oneWs + 2*oneDs + 3*oneHs + 4*oneMins + 5,
					},
				},
			},
		},
		{
			name: "week_second",
			fields: map[string]interface{}{
				"value-names": []string{
					".*uptime",
				},
				"debug": true,
			},
			args: args{
				es: []*formatters.EventMsg{
					{
						Name:      "sub1",
						Timestamp: 42,
						Tags:      map[string]string{},
						Values: map[string]interface{}{
							"connection_uptime": "1w5s",
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
						"connection_uptime": oneWs + 5,
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &durationConvert{}
			err := c.Init(tt.fields, formatters.WithLogger(log.New(os.Stderr, "[event-duration-convert-test]", log.Flags())))
			if err != nil {
				t.Errorf("failed to init processor in test %q: %v", tt.name, err)
				t.Fail()
			}
			if got := c.Apply(tt.args.es...); !cmp.Equal(got, tt.want) {
				t.Errorf("durationConvert.Apply() = %v, want %v", got, tt.want)
			}
		})
	}
}
