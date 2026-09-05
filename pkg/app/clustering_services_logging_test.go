// © 2026 Supranett.
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/openconfig/gnmic/pkg/lockers"
)

func TestUpdateServicesLogsMembershipChanges(t *testing.T) {
	var logs bytes.Buffer
	a := &App{
		configLock:  new(sync.RWMutex),
		apiServices: make(map[string]*lockers.Service),
		Logger:      slog.New(slog.NewJSONHandler(&logs, nil)),
	}
	first := &lockers.Service{ID: "first", Address: "192.0.2.1:7890", Tags: []string{"a"}}
	refreshed := &lockers.Service{ID: "first", Address: "192.0.2.2:7890", Tags: []string{"b"}}
	second := &lockers.Service{ID: "second", Address: "192.0.2.3:7890"}
	type record struct {
		Message string `json:"msg"`
		ID      string `json:"id"`
	}
	for _, tc := range []struct {
		name     string
		services []*lockers.Service
		want     []record
	}{
		{name: "empty"},
		{name: "initial member", services: []*lockers.Service{first}, want: []record{{"adding service", "first"}}},
		{name: "unchanged", services: []*lockers.Service{first}},
		{name: "updated endpoint and tags", services: []*lockers.Service{refreshed}},
		{name: "new member", services: []*lockers.Service{refreshed, second}, want: []record{{"adding service", "second"}}},
		{name: "removed member", services: []*lockers.Service{second}, want: []record{{"deleting service", "first"}}},
		{name: "clear", want: []record{{Message: "deleting all services"}}},
		{name: "still empty"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			logs.Reset()
			a.updateServices(tc.services)
			wantServices := make(map[string]*lockers.Service, len(tc.services))
			for _, service := range tc.services {
				wantServices[service.ID] = service
			}
			if diff := cmp.Diff(wantServices, a.apiServices); diff != "" {
				t.Errorf("service registry mismatch (-want +got):\n%s", diff)
			}
			var got []record
			decoder := json.NewDecoder(&logs)
			for {
				var entry record
				if err := decoder.Decode(&entry); err == io.EOF {
					break
				} else if err != nil {
					t.Fatal(err)
				}
				got = append(got, entry)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("INFO records mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
