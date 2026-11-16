// © 2026 Supranett
//
// SPDX-License-Identifier: Apache-2.0

package formatters

import (
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/openconfig/gnmi/proto/gnmi"
	"google.golang.org/protobuf/proto"
)

type isolationProcessor struct {
	BaseProcessor
	calls int
}

func (p *isolationProcessor) Init(any, ...Option) error { return nil }

func (p *isolationProcessor) Apply(events ...*EventMsg) []*EventMsg {
	p.calls++
	return events
}

func isolationPath(name string) *gnmi.Path {
	return &gnmi.Path{Elem: []*gnmi.PathElem{{Name: name}}}
}

func isolationNotification(payload string, position int, atomic bool) *gnmi.Notification {
	updates := []*gnmi.Update{
		{Path: isolationPath("valid-before"), Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_IntVal{IntVal: 10}}},
		{Path: isolationPath("valid-after"), Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_IntVal{IntVal: 20}}},
	}
	updates = slices.Insert(updates, position, &gnmi.Update{
		Path: isolationPath("invalid"), Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_JsonIetfVal{JsonIetfVal: []byte(payload)}},
	})
	return &gnmi.Notification{
		Timestamp: 123, Atomic: atomic, Prefix: &gnmi.Path{Target: "STATE_DB"},
		Update: updates, Delete: []*gnmi.Path{isolationPath("removed-interface")},
	}
}

func TestResponseToEventsIsolatesInvalidUpdates(t *testing.T) {
	for _, payload := range []string{`{`, `{"temperature":1e400}`} {
		for position := range 3 {
			t.Run(payload+string(rune('0'+position)), func(t *testing.T) {
				processor := &isolationProcessor{}
				events, err := ResponseToEventMsgs("state", &gnmi.SubscribeResponse{
					Response: &gnmi.SubscribeResponse_Update{Update: isolationNotification(payload, position, false)},
				}, map[string]string{"source": "device-a"}, processor)
				if err == nil || !strings.Contains(err.Error(), "/invalid") {
					t.Fatalf("decode diagnostic = %v, want rejected path", err)
				}
				if len(events) != 3 || processor.calls != 1 {
					t.Fatalf("events=%+v processor calls=%d, want 2 updates and 1 delete", events, processor.calls)
				}
				if events[0].Values["/valid-before"] != int64(10) || events[1].Values["/valid-after"] != int64(20) {
					t.Fatalf("healthy values changed: %v, %v", events[0].Values, events[1].Values)
				}
				for _, event := range events {
					if event.Timestamp != 123 || event.Tags["source"] != "device-a" || event.Tags["target"] != "STATE_DB" {
						t.Errorf("metadata lost: %+v", event)
					}
				}
				if !slices.Equal(events[2].Deletes, []string{"/removed-interface"}) {
					t.Fatalf("deletes = %v", events[2].Deletes)
				}
			})
		}
	}
}

func TestResponseToEventsJoinsDecodeErrors(t *testing.T) {
	notification := isolationNotification(`{`, 0, false)
	notification.Update = append(notification.Update, &gnmi.Update{
		Path: isolationPath("overflow"), Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_JsonVal{JsonVal: []byte(`{"value":1e400}`)}},
	})
	events, err := ResponseToEventMsgs("state", &gnmi.SubscribeResponse{
		Response: &gnmi.SubscribeResponse_Update{Update: notification},
	}, map[string]string{"source": "device-a"})
	var syntaxError *json.SyntaxError
	var typeError *json.UnmarshalTypeError
	if !errors.As(err, &syntaxError) || !errors.As(err, &typeError) {
		t.Fatalf("decode errors = %v, want both syntax and numeric errors", err)
	}
	for _, context := range []string{"state", "device-a", "STATE_DB", "/invalid", "/overflow"} {
		if !strings.Contains(err.Error(), context) {
			t.Errorf("diagnostic %q lacks %q", err, context)
		}
	}
	if len(events) != 3 {
		t.Fatalf("events = %v, want the two healthy values and delete", events)
	}
}

func TestResponseToEventsPreservesAtomicNotifications(t *testing.T) {
	for _, payload := range []string{`{`, `{"temperature":40}`} {
		processor := &isolationProcessor{}
		events, err := ResponseToEventMsgs("state", &gnmi.SubscribeResponse{
			Response: &gnmi.SubscribeResponse_Update{Update: isolationNotification(payload, 1, true)},
		}, nil, processor)
		if payload == "{" {
			if err == nil || len(events) != 0 || processor.calls != 0 {
				t.Fatalf("invalid atomic notification escaped: events=%+v error=%v calls=%d", events, err, processor.calls)
			}
		} else if err != nil || len(events) != 4 || processor.calls != 1 {
			t.Fatalf("valid atomic notification lost: events=%+v error=%v calls=%d", events, err, processor.calls)
		}
	}
}

func TestGetResponseToEventsIsolatesNotifications(t *testing.T) {
	processor := &isolationProcessor{}
	events, err := GetResponseToEventMsgs(&gnmi.GetResponse{Notification: []*gnmi.Notification{
		isolationNotification(`{`, 0, true),
		isolationNotification(`{`, 2, false),
		{Update: []*gnmi.Update{{Path: isolationPath("other"), Val: &gnmi.TypedValue{Value: &gnmi.TypedValue_IntVal{IntVal: 30}}}}},
	}}, nil, processor)
	if err == nil || len(events) != 4 || processor.calls != 1 {
		t.Fatalf("partial Get lost: events=%+v error=%v calls=%d", events, err, processor.calls)
	}
}

func TestHealthyEmptyResponsesStillRunProcessors(t *testing.T) {
	for _, atomic := range []bool{false, true} {
		processor := &isolationProcessor{}
		events, err := ResponseToEventMsgs("state", &gnmi.SubscribeResponse{
			Response: &gnmi.SubscribeResponse_Update{Update: &gnmi.Notification{Atomic: atomic}},
		}, nil, processor)
		if err != nil || len(events) != 0 || processor.calls != 1 {
			t.Fatalf("atomic=%t events=%v error=%v calls=%d, want one processor call", atomic, events, err, processor.calls)
		}
	}
	processor := &isolationProcessor{}
	events, err := GetResponseToEventMsgs(&gnmi.GetResponse{}, nil, processor)
	if err != nil || len(events) != 0 || processor.calls != 1 {
		t.Fatalf("empty Get: events=%v error=%v calls=%d, want one processor call", events, err, processor.calls)
	}
}

func TestEventMarshalPreservesPartialResults(t *testing.T) {
	options := &MarshalOptions{Format: "event"}
	for _, message := range []proto.Message{
		&gnmi.SubscribeResponse{Response: &gnmi.SubscribeResponse_Update{Update: isolationNotification(`{`, 1, false)}},
		&gnmi.GetResponse{Notification: []*gnmi.Notification{isolationNotification(`{`, 1, false)}},
	} {
		data, err := options.Marshal(message, nil)
		if err == nil {
			t.Fatal("partial serialization lost decode diagnostic")
		}
		var events []*EventMsg
		if err := json.Unmarshal(data, &events); err != nil || len(events) != 3 {
			t.Fatalf("serialized events=%+v error=%v", events, err)
		}
	}
}

func FuzzNotificationUpdateIsolation(f *testing.F) {
	for _, seed := range []string{`{`, `{"temperature":1e400}`, `"NA"`, `null`, `[]`, `{"value":42}`} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, payload string) {
		if len(payload) > 4096 {
			t.Skip()
		}
		for _, atomic := range []bool{false, true} {
			events, err := ResponseToEventMsgs("state", &gnmi.SubscribeResponse{
				Response: &gnmi.SubscribeResponse_Update{Update: isolationNotification(payload, 1, atomic)},
			}, nil)
			want := 4
			if err != nil {
				want = 3
				if atomic {
					want = 0
				}
			}
			if len(events) != want {
				t.Fatalf("atomic=%t events=%d error=%v, want %d", atomic, len(events), err, want)
			}
		}
	})
}
