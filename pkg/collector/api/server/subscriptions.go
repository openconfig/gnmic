package apiserver

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mitchellh/mapstructure"
	"github.com/openconfig/gnmic/pkg/api/types"
)

func (s *Server) handleConfigSubscriptionsGet(w http.ResponseWriter, r *http.Request) {
	subscriptions, err := s.configStore.List("subscriptions")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(subscriptions)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
	sub := new(types.SubscriptionConfig)
	// handles time.Duration
	decoder, err := mapstructure.NewDecoder(
		&mapstructure.DecoderConfig{
			DecodeHook: mapstructure.StringToTimeDurationHookFunc(),
			Result:     sub,
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
	ok, err := s.configStore.Set("subscriptions", sub.Name, sub)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "subscription already exists", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleConfigSubscriptionsDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	ok, _, err := s.configStore.Delete("subscriptions", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "subscription not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}
