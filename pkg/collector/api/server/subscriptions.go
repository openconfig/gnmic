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

func (s *Server) handleConfigSubscriptionsGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		// Get all subscriptions
		subscriptions, err := s.store.Config.List("subscriptions")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
		err = json.NewEncoder(w).Encode(subscriptions)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
		return
	}

	// Get single subscription by ID
	sub, ok, err := s.store.Config.Get("subscriptions", id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"subscription not found"}})
		return
	}
	err = json.NewEncoder(w).Encode(sub)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
}

// sample body:
//
//	{
//		"name": "subscription1",
//		"prefix": "interfaces",
//		"set-target": true,
//		"paths": ["interfaces/interface/state"],
//		"mode": "STREAM",
//		"stream-mode": "TARGET_DEFINED",
//		"encoding": "JSON",
//		"sample-interval": 1000
//	}
//
// sample curl command:
// curl --request POST -H "Content-Type: application/json" \
// -d '{"name": "subscription1", "prefix": "interfaces", "set-target": true, "paths": ["interfaces/interface/state"], "mode": "STREAM", "stream-mode": "TARGET_DEFINED", "encoding": "JSON", "sample-interval": 1000}' \
// http://localhost:8080/api/v1/subscriptions
func (s *Server) handleConfigSubscriptionsPost(w http.ResponseWriter, r *http.Request) {
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
	sub := new(types.SubscriptionConfig)
	// handles time.Duration
	decoder, err := mapstructure.NewDecoder(
		&mapstructure.DecoderConfig{
			DecodeHook: mapstructure.StringToTimeDurationHookFunc(),
			Result:     sub,
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
	_, err = s.store.Config.Set("subscriptions", sub.Name, sub)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleConfigSubscriptionsDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	ok, _, err := s.store.Config.Delete("subscriptions", id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"subscription not found"}})
		return
	}
	w.WriteHeader(http.StatusOK)
}

// SubscriptionResponse represents a subscription with its targets and states
type SubscriptionResponse struct {
	Name    string                      `json:"name"`
	Config  *types.SubscriptionConfig   `json:"config"`
	Targets map[string]*TargetStateInfo `json:"targets"`
}

// TargetStateInfo represents target information for a subscription
type TargetStateInfo struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

// handleSubscriptionsGet returns runtime subscription information with target states
func (s *Server) handleSubscriptionsGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	if id == "" {
		subscriptionsMap := make(map[string]*SubscriptionResponse)
		// build current subscriptions map
		_, err := s.store.Config.List("subscriptions", func(name string, sub any) bool {
			switch sub := sub.(type) {
			case *types.SubscriptionConfig:
				subscriptionsMap[sub.Name] = &SubscriptionResponse{
					Name:    sub.Name,
					Config:  sub,
					Targets: make(map[string]*TargetStateInfo),
				}
			}
			return false
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
		// Collect all subscriptions from targets
		s.targetsManager.ForEach(func(mt *targets_manager.ManagedTarget) {
			subStates := mt.T.SubscribeClientStates()
			for name, active := range subStates {
				if subscriptionsMap[name] == nil {
					subscriptionsMap[name] = &SubscriptionResponse{
						Name:    name,
						Targets: make(map[string]*TargetStateInfo),
					}
				}
				state := "stopped"
				if active {
					state = "running"
				}
				subscriptionsMap[name].Targets[mt.Name] = &TargetStateInfo{
					Name:  mt.Name,
					State: state,
				}
			}
		})
		// Return all subscriptions
		response := make([]*SubscriptionResponse, 0)
		for _, sub := range subscriptionsMap {
			response = append(response, sub)
		}
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
		return
	}
	sub, ok, err := s.store.Config.Get("subscriptions", id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"subscription not found"}})
		return
	}
	switch sub := sub.(type) {
	case *types.SubscriptionConfig:
		response := &SubscriptionResponse{
			Name:    sub.Name,
			Config:  sub,
			Targets: make(map[string]*TargetStateInfo),
		}
		s.targetsManager.ForEach(func(mt *targets_manager.ManagedTarget) {
			subStates := mt.T.SubscribeClientStates()
			active, exists := subStates[id]
			if !exists {
				return
			}
			state := "stopped"
			if active {
				state = "running"
			}
			response.Targets[mt.Name] = &TargetStateInfo{
				Name:  mt.Name,
				State: state,
			}
		})
		err = json.NewEncoder(w).Encode(response)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
		}
	default:
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{fmt.Sprintf("unknown subscription type: %T", sub)}})
		return
	}
}
