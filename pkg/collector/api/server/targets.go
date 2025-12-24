package apiserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mitchellh/mapstructure"
	"github.com/openconfig/gnmic/pkg/api/types"
	targets_manager "github.com/openconfig/gnmic/pkg/collector/managers/targets"
)

func (s *Server) handleConfigTargetsGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		targets, err := s.configStore.List("targets")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
		err = json.NewEncoder(w).Encode(targets)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
		return
	}
	tc, ok, err := s.configStore.Get("targets", id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{fmt.Sprintf("target %s not found", id)}})
		return
	}
	err = json.NewEncoder(w).Encode(tc)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
}

// sample body:
//
//	{
//		"name": "target1",
//		"address": "127.0.0.1:57400",
//		"username": "admin",
//		"password": "admin"
//	}
//
// sample curl command:
// curl --request POST -H "Content-Type: application/json" \
// -d '{"name": "target1", "address": "127.0.0.1:57400", "username": "admin", "password": "admin", "insecure": true}' \
// http://localhost:8080/api/v1/config/targets
func (s *Server) handleConfigTargetsPost(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	defer r.Body.Close()
	m := map[string]any{}
	err = json.Unmarshal(body, &m)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	tc := new(types.TargetConfig)
	// handles time.Duration
	decoder, err := mapstructure.NewDecoder(
		&mapstructure.DecoderConfig{
			DecodeHook: mapstructure.StringToTimeDurationHookFunc(),
			Result:     tc,
		})
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	err = decoder.Decode(m)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	if tc.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"target name is required"}})
		return
	}
	if tc.Address == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"target address is required"}})
		return
	}
	// validate subscriptions
	for _, sub := range tc.Subscriptions {
		_, ok, err := s.configStore.Get("subscriptions", sub)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{fmt.Sprintf("subscription %s not found", sub)}})
			return
		}
	}
	// validate outputs
	for _, out := range tc.Outputs {
		_, ok, err := s.configStore.Get("outputs", out)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{fmt.Sprintf("output %s not found", out)}})
			return
		}
	}

	_, err = s.configStore.Set("targets", tc.Name, tc)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}

	w.WriteHeader(http.StatusOK)
}

// update target subscriptions by sending a PATCH request to the target id
// sample body:
//
//	{
//		"subscriptions": ["sub1", "sub2"]
//	}
//
// sample curl command:
// curl --request PATCH -H "Content-Type: application/json" \
// -d '["sub1", "sub2"]' \
// http://localhost:8080/api/v1/config/targets/target1/subscriptions
func (s *Server) handleConfigTargetsSubscriptionsPatch(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}

	defer r.Body.Close()
	subs := []string{}
	if len(body) > 0 {
		err = json.Unmarshal(body, &subs)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
	}
	// ensure subscriptions exist
	for _, sub := range subs {
		_, ok, err := s.configStore.Get("subscriptions", sub)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{fmt.Sprintf("subscription %s not found", sub)}})
			return
		}
	}
	_, err = s.configStore.SetFn("targets", id,
		func(v any) (any, error) {
			tc, ok := v.(*types.TargetConfig)
			if !ok {
				return nil, fmt.Errorf("malformed target config")
			}
			tc.Subscriptions = subs
			return tc, nil
		})
	if err != nil {
		// TODO: handle key not found error
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	w.WriteHeader(http.StatusOK)
}

// update target outputs by sending a PATCH request to the target id
// sample body:
//
//	{
//		"outputs": ["output1", "output2"]
//	}
//
// sample curl command:
// curl --request PATCH -H "Content-Type: application/json" \
// -d '["output1", "output2"]' \
// http://localhost:8080/api/v1/config/targets/target1/outputs
func (s *Server) handleConfigTargetsOutputsPatch(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	defer r.Body.Close()
	outs := []string{}
	if len(body) > 0 {
		err = json.Unmarshal(body, &outs)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
	}
	// ensure outputs exist
	for _, out := range outs {
		_, ok, err := s.configStore.Get("outputs", out)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{fmt.Sprintf("output %s not found", out)}})
			return
		}
	}
	_, err = s.configStore.SetFn("targets", id,
		func(v any) (any, error) {
			tc, ok := v.(*types.TargetConfig)
			if !ok {
				return nil, fmt.Errorf("malformed target config")
			}
			tc.Outputs = outs
			return tc, nil
		})
	if err != nil {
		// TODO: handle key not found error
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleConfigTargetsDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	_, _, err := s.configStore.Delete("targets", id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	w.WriteHeader(http.StatusOK)
}

type TargetResponse struct {
	Name          string                                `json:"name"`
	Config        *types.TargetConfig                   `json:"config"`
	Subscriptions map[string]*SubscriptionStateResponse `json:"subscriptions"`
	State         string                                `json:"state"`
}

type SubscriptionStateResponse struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

func (s *Server) handleTargetsGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	response := make([]*TargetResponse, 0)
	if id == "" {
		s.targetsManager.ForEach(func(mt *targets_manager.ManagedTarget) {
			subs := make(map[string]*SubscriptionStateResponse)
			for _, sub := range mt.T.Subscriptions {
				if _, ok := mt.T.SubscribeClients[sub.Name]; ok {
					subs[sub.Name] = &SubscriptionStateResponse{
						Name:  sub.Name,
						State: "running",
					}
				} else {
					subs[sub.Name] = &SubscriptionStateResponse{
						Name:  sub.Name,
						State: "stopped",
					}
				}
			}
			response = append(response, &TargetResponse{
				Name:          mt.Name,
				Config:        mt.T.Config,
				Subscriptions: subs,
				State:         mt.State.Load().(string),
			})
		})
		err := json.NewEncoder(w).Encode(response)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
		return
	}
	mt := s.targetsManager.Lookup(id)
	if mt == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"target not found"}})
		return
	}
	subs := make(map[string]*SubscriptionStateResponse)
	for name := range mt.T.SubscribeClients {
		subs[name] = &SubscriptionStateResponse{
			Name:  name,
			State: "running",
		}
	}
	response = append(response, &TargetResponse{
		Name:          mt.Name,
		Config:        mt.T.Config,
		Subscriptions: subs,
		State:         mt.State.Load().(string),
	})
	err := json.NewEncoder(w).Encode(response)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
}

// change target state to running/stopped by sending a POST request to the target id
func (s *Server) handleTargetsStatePost(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"target id is required"}})
		return
	}
	state := vars["state"]
	if state == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"target state is required"}})
		return
	}
	mt := s.targetsManager.Lookup(id)
	if mt == nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"target not found"}})
		return
	}
	ok := s.targetsManager.SetIntendedState(id, state)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"target state not changed"}})
		return
	}
	w.WriteHeader(http.StatusOK)
}
