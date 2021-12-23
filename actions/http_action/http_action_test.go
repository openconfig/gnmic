package http_action

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/karimra/gnmic/actions"
	"github.com/karimra/gnmic/formatters"
	"github.com/karimra/gnmic/utils"
)

type item struct {
	input  *formatters.EventMsg
	output interface{}
}

var testset = map[string]struct {
	actionType string
	action     map[string]interface{}
	tests      []item
}{
	"default_values": {
		actionType: actionType,
		action: map[string]interface{}{
			"type":  "http",
			"name":  "act1",
			"URL":   "http://localhost:8080",
			"debug": true,
		},
		tests: []item{
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Tags: map[string]string{
						"tag1": "1",
					},
				},
				output: map[string]interface{}{
					"Input": map[string]interface{}{
						"name": "sub1",
						"tags": map[string]interface{}{
							"tag1": "1",
						},
					},
				},
			},
		},
	},
	"with_simple_template": {
		actionType: actionType,
		action: map[string]interface{}{
			"type":  "http",
			"name":  "act1",
			"url":   "http://localhost:8080",
			"body":  `{{ name .Input }}`,
			"debug": true,
		},
		tests: []item{
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Tags: map[string]string{
						"tag1": "1",
					},
				},
				output: "sub1",
			},
			{
				input: &formatters.EventMsg{
					Name: "sub2",
				},
				output: "sub2",
			},
		},
	},
	"remove_all_tags": {
		actionType: actionType,
		action: map[string]interface{}{
			"type":  "http",
			"name":  "act1",
			"url":   "http://localhost:8080",
			"body":  `{{ withTags .Input }}`,
			"debug": true,
		},
		tests: []item{
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Tags: map[string]string{
						"tag1": "1",
					},
				},
				output: map[string]interface{}{
					"name": "sub1",
				},
			},
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Tags: map[string]string{
						"tag1": "1",
						"tag2": "2",
					},
				},
				output: map[string]interface{}{
					"name": "sub1",
				},
			},
		},
	},
	"remove_some_tags": {
		actionType: actionType,
		action: map[string]interface{}{
			"type":  "http",
			"name":  "act1",
			"url":   "http://localhost:8080",
			"body":  `{{ withoutTags .Input "tag1" }}`,
			"debug": true,
		},
		tests: []item{
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Tags: map[string]string{
						"tag1": "1",
					},
				},
				output: map[string]interface{}{
					"name": "sub1",
				},
			},
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Tags: map[string]string{
						"tag1": "1",
						"tag2": "2",
					},
				},
				output: map[string]interface{}{
					"name": "sub1",
					"tags": map[string]interface{}{
						"tag2": "2",
					},
				},
			},
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Tags: map[string]string{
						"tag2": "2",
					},
				},
				output: map[string]interface{}{
					"name": "sub1",
					"tags": map[string]interface{}{
						"tag2": "2",
					},
				},
			},
		},
	},
	"select_some_tags": {
		actionType: actionType,
		action: map[string]interface{}{
			"type":  "http",
			"name":  "act1",
			"url":   "http://localhost:8080",
			"body":  `{{ withTags .Input "tag1" }}`,
			"debug": true,
		},
		tests: []item{
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Tags: map[string]string{
						"tag1": "1",
						"tag2": "2",
						"tag3": "3",
					},
				},
				output: map[string]interface{}{
					"name": "sub1",
					"tags": map[string]interface{}{
						"tag1": "1",
					},
				},
			},
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Tags: map[string]string{
						"tag2": "2",
					},
				},
				output: map[string]interface{}{
					"name": "sub1",
				},
			},
		},
	},
	"remove_all_values": {
		actionType: actionType,
		action: map[string]interface{}{
			"type":  "http",
			"name":  "act1",
			"url":   "http://localhost:8080",
			"body":  `{{ withValues .Input }}`,
			"debug": true,
		},
		tests: []item{
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Tags: map[string]string{
						"tag1": "1",
					},
				},
				output: map[string]interface{}{
					"name": "sub1",
					"tags": map[string]interface{}{
						"tag1": "1",
					},
				},
			},
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Tags: map[string]string{
						"tag1": "1",
					},
					Values: map[string]interface{}{
						"val1": "1",
					},
				},
				output: map[string]interface{}{
					"name": "sub1",
					"tags": map[string]interface{}{
						"tag1": "1",
					},
				},
			},
		},
	},
	"remove_some_values": {
		actionType: actionType,
		action: map[string]interface{}{
			"type":  "http",
			"name":  "act1",
			"url":   "http://localhost:8080",
			"body":  `{{ withoutValues .Input "val1"}}`,
			"debug": true,
		},
		tests: []item{
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Tags: map[string]string{
						"tag1": "1",
					},
				},
				output: map[string]interface{}{
					"name": "sub1",
					"tags": map[string]interface{}{
						"tag1": "1",
					},
				},
			},
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Tags: map[string]string{
						"tag1": "1",
					},
					Values: map[string]interface{}{
						"val1": "1",
					},
				},
				output: map[string]interface{}{
					"name": "sub1",
					"tags": map[string]interface{}{
						"tag1": "1",
					},
				},
			},
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Tags: map[string]string{
						"tag1": "1",
					},
					Values: map[string]interface{}{
						"val1": "1",
						"val2": "2",
					},
				},
				output: map[string]interface{}{
					"name": "sub1",
					"tags": map[string]interface{}{
						"tag1": "1",
					},
					"values": map[string]interface{}{
						"val2": "2",
					},
				},
			},
		},
	},
	"select_some_values": {
		actionType: actionType,
		action: map[string]interface{}{
			"type":  "http",
			"name":  "act1",
			"url":   "http://localhost:8080",
			"body":  `{{ withValues .Input "val1" }}`,
			"debug": true,
		},
		tests: []item{
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Tags: map[string]string{
						"tag1": "1",
					},
					Values: map[string]interface{}{
						"val1": "1",
					},
				},
				output: map[string]interface{}{
					"name": "sub1",
					"tags": map[string]interface{}{
						"tag1": "1",
					},
					"values": map[string]interface{}{
						"val1": "1",
					},
				},
			},
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Tags: map[string]string{
						"tag1": "1",
					},
				},
				output: map[string]interface{}{
					"name": "sub1",
					"tags": map[string]interface{}{
						"tag1": "1",
					},
				},
			},
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Tags: map[string]string{
						"tag1": "1",
					},
					Values: map[string]interface{}{
						"val2": "2",
					},
				},
				output: map[string]interface{}{
					"name": "sub1",
					"tags": map[string]interface{}{
						"tag1": "1",
					},
				},
			},
		},
	},
	"select_tags_and_values": {
		actionType: actionType,
		action: map[string]interface{}{
			"type":  "http",
			"name":  "act1",
			"url":   "http://localhost:8080",
			"body":  `{{ withTags (withValues .Input "val1") "tag1" }}`,
			"debug": true,
		},
		tests: []item{
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Tags: map[string]string{
						"tag1": "1",
						"tag2": "2",
					},
					Values: map[string]interface{}{
						"val1": "1",
					},
				},
				output: map[string]interface{}{
					"name": "sub1",
					"tags": map[string]interface{}{
						"tag1": "1",
					},
					"values": map[string]interface{}{
						"val1": "1",
					},
				},
			},
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Tags: map[string]string{
						"tag2": "2",
					},
				},
				output: map[string]interface{}{
					"name": "sub1",
				},
			},
			{
				input: &formatters.EventMsg{
					Name: "sub1",
					Tags: map[string]string{
						"tag2": "2",
					},
					Values: map[string]interface{}{
						"val1": "1",
						"val2": "2",
					},
				},
				output: map[string]interface{}{
					"name": "sub1",
					"values": map[string]interface{}{
						"val1": "1",
					},
				},
			},
		},
	},
}

func TestHTTPAction(t *testing.T) {
	for name, ts := range testset {
		if ai, ok := actions.Actions[ts.actionType]; ok {
			t.Log("found action")
			a := ai()
			err := a.Init(ts.action, actions.WithLogger(log.New(os.Stderr, loggingPrefix, utils.DefaultLoggingFlags)))
			if err != nil {
				t.Errorf("failed to initialize action: %v", err)
				return
			}
			t.Logf("action: %+v", a)
			mux := http.NewServeMux()
			mux.Handle("/", echo())
			ah, ok := a.(*httpAction)
			if !ok {
				t.Errorf("failed to assert action type: %T", a)
				t.Fail()
				return
			}
			// start http server
			urlAddr, err := url.Parse(ah.URL)
			if err != nil {
				t.Errorf("failed to parse URL: %v", err)
				t.Fail()
				return
			}
			s := &http.Server{
				Addr:    urlAddr.Host,
				Handler: mux,
			}
			go func() {
				if err := s.ListenAndServe(); err != nil {
					if !errors.Is(err, http.ErrServerClosed) {
						t.Logf("failed to start http server: %v", err)
					}
				}
			}()
			// wait for server
			time.Sleep(time.Second)
			//
			for i, item := range ts.tests {
				t.Run(name, func(t *testing.T) {
					t.Logf("running test item %d", i)
					res, err := a.Run(&actions.Context{Input: item.input})
					if err != nil {
						t.Errorf("failed at %s item %d, %v", name, i, err)
						t.Fail()
						return
					}
					t.Logf("Run result: %+v", string(res.([]byte)))
					var result interface{}
					err = json.Unmarshal(res.([]byte), &result)
					if err != nil {
						t.Errorf("failed at %s item %d, %v", name, i, err)
						t.Fail()
						return
					}
					if !reflect.DeepEqual(result, item.output) {
						t.Errorf("failed at %s item %d, expected %+v(%T), got: %+v(%T)", name, i, item.output, item.output, result, result)
					}
				})
			}
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			s.Shutdown(ctx)
			cancel()
		} else {
			t.Errorf("action %s not found", ts.actionType)
		}
	}
}

func echo() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := ioutil.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintf(w, "%v", err)
			return
		}
		fmt.Fprint(w, string(b))
	})
}
