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
		subscriptions, err := s.configStore.List("subscriptions")
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
	sub, ok, err := s.configStore.Get("subscriptions", id)
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
	_, err = s.configStore.Set("subscriptions", sub.Name, sub)
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
	ok, _, err := s.configStore.Delete("subscriptions", id)
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

// RuntimeSubscriptionResponse represents a subscription with its targets and states
type RuntimeSubscriptionResponse struct {
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

	// Collect all subscriptions from targets
	subscriptionsMap := make(map[string]*RuntimeSubscriptionResponse)

	s.targetsManager.ForEach(func(mt *targets_manager.ManagedTarget) {
		fmt.Println("target", mt.Name, "subscriptions", mt.T.Subscriptions)
		for _, sub := range mt.T.Subscriptions {
			fmt.Println("subscription", sub.Name, "target", mt.Name)
			if subscriptionsMap[sub.Name] == nil {
				subscriptionsMap[sub.Name] = &RuntimeSubscriptionResponse{
					Name:    sub.Name,
					Config:  sub,
					Targets: make(map[string]*TargetStateInfo),
				}
			}

			// Determine state for this subscription on this target
			state := "stopped"
			if _, ok := mt.T.SubscribeClients[sub.Name]; ok {
				state = "running"
			}

			subscriptionsMap[sub.Name].Targets[mt.Name] = &TargetStateInfo{
				Name:  mt.Name,
				State: state,
			}
		}
	})

	if id == "" {
		// Return all subscriptions
		response := make([]*RuntimeSubscriptionResponse, 0, len(subscriptionsMap))
		for _, sub := range subscriptionsMap {
			response = append(response, sub)
		}

		err := json.NewEncoder(w).Encode(response)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
		return
	}

	// Return single subscription
	sub, ok := subscriptionsMap[id]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"subscription not found"}})
		return
	}

	err := json.NewEncoder(w).Encode([]*RuntimeSubscriptionResponse{sub})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
}
