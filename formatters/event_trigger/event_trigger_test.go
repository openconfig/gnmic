package event_trigger

import (
	"log"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/utils"
)

type item struct {
	input  []*formatters.EventMsg
	output []*formatters.EventMsg
}

var actionsCfg = map[string]map[string]interface{}{
	"dummy1": {
		"name": "dummy1",
		"type": "http",
	},
	"dummy2": {
		"name": "dummy2",
		"type": "http",
		"url":  "http://remote-alerting-system:9090/",
	},
}
var testset = map[string]struct {
	processorType string
	processor     map[string]interface{}
	tests         []item
}{
	"init": {
		processorType: processorType,
		processor: map[string]interface{}{
			"debug": true,
			"actions": []string{
				"dummy1",
			},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input:  make([]*formatters.EventMsg, 0),
				output: make([]*formatters.EventMsg, 0),
			},
			{
				input: []*formatters.EventMsg{
					{
						Name: "sub1",
						Tags: map[string]string{
							"tag1": "1",
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Name: "sub1",
						Tags: map[string]string{
							"tag1": "1",
						},
					},
				},
			},
		},
	},
	"with_condition": {
		processorType: processorType,
		processor: map[string]interface{}{
			"condition": `.values["counter1"] > 90`,
			"debug":     true,
			"actions": []string{
				"dummy2",
			},
		},
		tests: []item{
			{
				input:  nil,
				output: nil,
			},
			{
				input:  make([]*formatters.EventMsg, 0),
				output: make([]*formatters.EventMsg, 0),
			},
			{
				input: []*formatters.EventMsg{
					{
						Name: "sub1",
						Tags: map[string]string{
							"tag1": "1",
						},
						Values: map[string]interface{}{
							"counter1": 91,
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Name: "sub1",
						Tags: map[string]string{
							"tag1": "1",
						},
						Values: map[string]interface{}{
							"counter1": 91,
						},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Name: "sub1",
						Tags: map[string]string{
							"tag1": "1",
						},
						Values: map[string]interface{}{
							"counter1": 89,
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Name: "sub1",
						Tags: map[string]string{
							"tag1": "1",
						},
						Values: map[string]interface{}{
							"counter1": 89,
						},
					},
				},
			},
			{
				input: []*formatters.EventMsg{
					{
						Name: "sub1",
						Tags: map[string]string{
							"tag1": "1",
						},
						Values: map[string]interface{}{
							"counter2": 91,
						},
					},
				},
				output: []*formatters.EventMsg{
					{
						Name: "sub1",
						Tags: map[string]string{
							"tag1": "1",
						},
						Values: map[string]interface{}{
							"counter2": 91,
						},
					},
				},
			},
		},
	},
}

var triggerOccWindowTestSet = map[string]struct {
	t   *Trigger
	now time.Time
	out bool
}{
	"defaults_0_occurrences": {
		t: &Trigger{
			logger:           log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags),
			Debug:            true,
			MinOccurrences:   1,
			MaxOccurrences:   1,
			Window:           time.Minute,
			occurrencesTimes: []time.Time{},
		},
		out: true,
		now: time.Now(),
	},
	"defaults_with_1_occurrence_in_window": {
		t: &Trigger{
			logger:         log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags),
			Debug:          true,
			MinOccurrences: 1,
			MaxOccurrences: 1,
			Window:         time.Minute,
			occurrencesTimes: []time.Time{
				time.Now().Add(-time.Second),
			},
			lastTrigger: time.Now().Add(-time.Second),
		},
		out: false,
		now: time.Now(),
	},
	"defaults_with_1_occurrence_out_of_window": {
		t: &Trigger{
			logger:         log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags),
			Debug:          true,
			MinOccurrences: 1,
			MaxOccurrences: 1,
			Window:         time.Minute,
			occurrencesTimes: []time.Time{
				time.Now().Add(-time.Hour),
			},
		},
		out: true,
		now: time.Now(),
	},
	"2max_1min_without_occurrences": {
		t: &Trigger{
			logger:           log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags),
			Debug:            true,
			MinOccurrences:   1,
			MaxOccurrences:   2,
			Window:           time.Minute,
			occurrencesTimes: []time.Time{},
		},
		out: true,
		now: time.Now(),
	},
	"2max_1min_with_1occurrence_in_window": {
		t: &Trigger{
			logger:         log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags),
			Debug:          true,
			MinOccurrences: 1,
			MaxOccurrences: 2,
			Window:         time.Minute,
			occurrencesTimes: []time.Time{
				time.Now().Add(-30 * time.Second),
			},
		},
		out: true,
		now: time.Now(),
	},
	"2max_1min_with_2occurrences_in_window": {
		t: &Trigger{
			logger:         log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags),
			Debug:          true,
			MinOccurrences: 1,
			MaxOccurrences: 2,
			Window:         time.Minute,
			occurrencesTimes: []time.Time{
				time.Now().Add(-10 * time.Second),
				time.Now().Add(-30 * time.Second),
			},
			lastTrigger: time.Now().Add(-10 * time.Second),
		},
		out: false,
		now: time.Now(),
	},
	"2max_2min_without_occurrences": {
		t: &Trigger{
			logger:           log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags),
			Debug:            true,
			MinOccurrences:   2,
			MaxOccurrences:   2,
			Window:           time.Minute,
			occurrencesTimes: []time.Time{},
		},
		out: false,
		now: time.Now(),
	},
	"2max_2min_with_1occurrence_in_window": {
		t: &Trigger{
			logger:         log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags),
			Debug:          true,
			MinOccurrences: 2,
			MaxOccurrences: 2,
			Window:         time.Minute,
			occurrencesTimes: []time.Time{
				time.Now().Add(-30 * time.Second),
			},
		},
		out: true,
		now: time.Now(),
	},
	"2max_2min_with_2occurrences_in_window": {
		t: &Trigger{
			logger:         log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags),
			Debug:          true,
			MinOccurrences: 2,
			MaxOccurrences: 2,
			Window:         time.Minute,
			occurrencesTimes: []time.Time{
				time.Now().Add(-10 * time.Second),
				time.Now().Add(-30 * time.Second),
			},
			lastTrigger: time.Now().Add(-10 * time.Second),
		},
		out: false,
		now: time.Now(),
	},
	"2max_2min_with_2occurrences_in_window_lastTrigger_out_of_window": {
		t: &Trigger{
			logger:         log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags),
			Debug:          true,
			MinOccurrences: 2,
			MaxOccurrences: 2,
			Window:         time.Minute,
			occurrencesTimes: []time.Time{
				time.Now().Add(-10 * time.Second),
				time.Now().Add(-30 * time.Second),
			},
			lastTrigger: time.Now().Add(-61 * time.Second),
		},
		out: true,
		now: time.Now(),
	},
}

func TestEventTrigger(t *testing.T) {
	for name, ts := range testset {
		if pi, ok := formatters.EventProcessors[ts.processorType]; ok {
			t.Log("found processor")
			p := pi()
			err := p.Init(ts.processor,
				formatters.WithLogger(log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)),
				formatters.WithActions(actionsCfg),
			)
			if err != nil {
				t.Errorf("failed to initialize processors: %v", err)
				return
			}
			t.Logf("processor: %+v", p)
			for i, item := range ts.tests {
				t.Run(name, func(t *testing.T) {
					t.Logf("running test item %d", i)
					outs := p.Apply(item.input...)
					if len(outs) != len(item.output) {
						t.Errorf("failed at %s, result has a different length than the expected result", name)
						t.Fail()
					}
					for j := range outs {
						if !cmp.Equal(outs[j], item.output[j]) {
							t.Errorf("failed at %s item %d, index %d, expected %+v, got: %+v", name, i, j, item.output[j], outs[j])
							t.Fail()
						}
					}
				})
			}
		} else {
			t.Errorf("event processor %s not found", ts.processorType)
		}
	}
}

func TestOccurrenceTrigger(t *testing.T) {
	for name, ts := range triggerOccWindowTestSet {
		t.Run(name, func(t *testing.T) {
			ok := ts.t.evalOccurrencesWithinWindow(ts.now)
			t.Logf("%q result: %v", name, ok)
			if ok != ts.out {
				t.Errorf("failed at %s , expected %+v, got: %+v", name, ts.out, ok)
			}
		})
	}
}
