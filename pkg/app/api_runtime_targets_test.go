// © 2026 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/openconfig/gnmic/pkg/api/target"
	"github.com/openconfig/gnmic/pkg/api/types"
)

func TestRuntimeTargetsAPIConcurrentMutation(t *testing.T) {
	for _, test := range []struct {
		name   string
		method string
		id     string
	}{
		{name: "list", method: http.MethodGet},
		{name: "get", method: http.MethodGet, id: "changing"},
		{name: "delete missing", method: http.MethodDelete, id: "missing"},
	} {
		t.Run(test.name, func(t *testing.T) {
			a := New()
			defer a.Cfn()
			fixed := target.NewTarget(&types.TargetConfig{Name: "fixed"})
			changing := target.NewTarget(&types.TargetConfig{Name: "changing"})
			a.Targets["fixed"] = fixed
			stop, done, started := make(chan struct{}), make(chan struct{}), make(chan struct{})
			go func() {
				defer close(done)
				close(started)
				for {
					select {
					case <-stop:
						return
					default:
					}
					a.operLock.Lock()
					a.Targets["changing"] = changing
					a.operLock.Unlock()
					runtime.Gosched()
					a.operLock.Lock()
					delete(a.Targets, "changing")
					a.operLock.Unlock()
				}
			}()
			defer func() { close(stop); <-done }()
			<-started
			for range 1000 {
				request := httptest.NewRequest(test.method, "/api/v1/targets/"+test.id, nil)
				request = mux.SetURLVars(request, map[string]string{"id": test.id})
				response := httptest.NewRecorder()
				if test.method == http.MethodDelete {
					a.handleTargetsDelete(response, request)
				} else {
					a.handleTargetsGet(response, request)
				}
				if response.Code != http.StatusOK && response.Code != http.StatusNotFound {
					t.Fatalf("unexpected status %d: %s", response.Code, response.Body.String())
				}
			}
		})
	}
}

func TestRuntimeTargetsAPIResponseReleasesOperationalLock(t *testing.T) {
	a := New()
	defer a.Cfn()
	password := "test-only-password"
	a.Targets["device"] = target.NewTarget(&types.TargetConfig{Name: "device", Password: &password})
	for _, id := range []string{"", "device", "missing"} {
		request := mux.SetURLVars(httptest.NewRequest(http.MethodGet, "/api/v1/targets", nil), map[string]string{"id": id})
		response := &operationalLockResponseWriter{ResponseRecorder: httptest.NewRecorder(), app: a, test: t}
		a.handleTargetsGet(response, request)
		if strings.Contains(response.Body.String(), password) {
			t.Fatal("runtime target response exposed the password")
		}
		wantStatus := http.StatusOK
		if id == "missing" {
			wantStatus = http.StatusNotFound
		}
		if response.Code != wantStatus {
			t.Fatalf("GET %q status = %d, want %d", id, response.Code, wantStatus)
		}
	}
}

type operationalLockResponseWriter struct {
	*httptest.ResponseRecorder
	app  *App
	test *testing.T
}

func (w *operationalLockResponseWriter) Write(body []byte) (int, error) {
	if w.app.operLock.TryLock() {
		w.app.operLock.Unlock()
	} else {
		w.test.Error("HTTP response writing blocks runtime target mutations")
	}
	return w.ResponseRecorder.Write(body)
}
