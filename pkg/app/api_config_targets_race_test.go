// This code is a Contribution to the gNMIc project ("Work") made under the Google Software Grant and Corporate Contributor License Agreement ("CLA") and governed by the Apache License 2.0.
// This code is provided on an "as is" basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

package app

// Tests for the remaining unprotected accesses to a.Config.Targets in the API
// handlers, which race with concurrent loader writes (AddTargetConfig / DeleteTarget).
//
// All three tests fail under "go test -race" before the fix is applied:
//
//   go test -race ./pkg/app/... \
//     -run 'TestHandleConfigRacesWithLoaderWrite|TestHandleTargetsPostRacesWithLoaderWrite|TestHandleClusteringDrainRacesWithLoaderWrite'
//
// After the fix (adding configLock.RLock/RUnlock to the three handlers) every test passes.

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sync"
	"testing"

	"github.com/gorilla/mux"
	"github.com/openconfig/gnmic/pkg/api/types"
)

func TestHandleConfigRacesWithLoaderWrite(t *testing.T) {
	a := New()
	defer a.Cfn()

	for i := range 20 {
		name := fmt.Sprintf("target-%d", i)
		a.Config.Targets[name] = &types.TargetConfig{Name: name}
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
				a.AddTargetConfig(&types.TargetConfig{Name: fmt.Sprintf("loader-%d", i)})
				runtime.Gosched()
			}
		}
	}()

	r := httptest.NewRequest(http.MethodGet, "/api/v1/config", nil)
	for range 500 {
		w := httptest.NewRecorder()
		a.handleConfig(w, r)
	}

	close(stop)
	wg.Wait()
}

func TestHandleTargetsPostRacesWithLoaderWrite(t *testing.T) {
	a := New()
	defer a.Cfn()

	for i := range 20 {
		name := fmt.Sprintf("target-%d", i)
		a.Config.Targets[name] = &types.TargetConfig{Name: name}
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
				a.AddTargetConfig(&types.TargetConfig{Name: fmt.Sprintf("loader-%d", i)})
				runtime.Gosched()
			}
		}
	}()

	for i := range 500 {
		id := fmt.Sprintf("target-%d", i%20)
		r := mux.SetURLVars(
			httptest.NewRequest(http.MethodPost, "/api/v1/targets/"+id, nil),
			map[string]string{"id": id},
		)
		w := httptest.NewRecorder()
		a.handleTargetsPost(w, r)
	}

	close(stop)
	wg.Wait()
}
