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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = json.NewEncoder(w).Encode(targets)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}
	tc, ok, err := s.configStore.Get("targets", id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	err = json.NewEncoder(w).Encode(tc)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()
	m := map[string]any{}
	err = json.Unmarshal(body, &m)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	err = decoder.Decode(m)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if tc.Name == "" {
		http.Error(w, "target name is required", http.StatusBadRequest)
		return
	}
	if tc.Address == "" {
		http.Error(w, "target address is required", http.StatusBadRequest)
		return
	}
	ok, err := s.configStore.Set("targets", tc.Name, tc)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "target already exists", http.StatusBadRequest)
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
// -d '{"subscriptions": ["sub1", "sub2"]}' \
// http://localhost:8080/api/v1/config/targets/target1/subscriptions
func (s *Server) handleConfigTargetsSubscriptionsPatch(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer r.Body.Close()
	subs := []string{}
	if len(body) > 0 {
		err = json.Unmarshal(body, &subs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	_, err = s.configStore.LoadAndSet("targets", id,
		func(v any) (any, error) {
			tc, ok := v.(*types.TargetConfig)
			if !ok {
				return nil, fmt.Errorf("malformed target config")
			}
			tc.Subscriptions = subs
			return tc, nil
		})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
// -d '{"outputs": ["output1", "output2"]}' \
// http://localhost:8080/api/v1/config/targets/target1/outputs
func (s *Server) handleConfigTargetsOutputsPatch(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()
	outs := []string{}
	if len(body) > 0 {
		err = json.Unmarshal(body, &outs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	_, err = s.configStore.LoadAndSet("targets", id,
		func(v any) (any, error) {
			tc, ok := v.(*types.TargetConfig)
			if !ok {
				return nil, fmt.Errorf("malformed target config")
			}
			tc.Outputs = outs
			return tc, nil
		})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleConfigTargetsDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	ok, prev, err := s.configStore.Delete("targets", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	fmt.Printf("ok: %v, prev: %v\n", ok, prev)
	// if !ok {
	// 	http.Error(w, "target not found", http.StatusNotFound)
	// 	return
	// }
	w.WriteHeader(http.StatusOK)
}

type TargetResponse struct {
	Name          string                           `json:"name"`
	Config        *types.TargetConfig              `json:"config"`
	Subscriptions map[string]*SubscriptionResponse `json:"subscriptions"`
	State         string                           `json:"state"`
}

type SubscriptionResponse struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

func (s *Server) handleTargetsGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	response := make([]*TargetResponse, 0)
	if id == "" {
		s.targetsManager.ForEach(func(mt *targets_manager.ManagedTarget) {
			subs := make(map[string]*SubscriptionResponse)
			for _, sub := range mt.T.Subscriptions {
				if _, ok := mt.T.SubscribeClients[sub.Name]; ok {
					subs[sub.Name] = &SubscriptionResponse{
						Name:  sub.Name,
						State: "running",
					}
				} else {
					subs[sub.Name] = &SubscriptionResponse{
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}
	mt := s.targetsManager.Lookup(id)
	if mt == nil {
		http.Error(w, "target not found", http.StatusNotFound)
		return
	}
	subs := make(map[string]*SubscriptionResponse)
	for name := range mt.T.SubscribeClients {
		subs[name] = &SubscriptionResponse{
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// change target state to running/stopped by sending a POST request to the target id
func (s *Server) handleTargetsStatePost(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		http.Error(w, "target id is required", http.StatusBadRequest)
		return
	}
	state := vars["state"]
	if state == "" {
		http.Error(w, "target state is required", http.StatusBadRequest)
		return
	}
	mt := s.targetsManager.Lookup(id)
	if mt == nil {
		http.Error(w, "target not found", http.StatusNotFound)
		return
	}
	ok := s.targetsManager.SetIntendedState(id, state)
	if !ok {
		http.Error(w, "target state not changed", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}
