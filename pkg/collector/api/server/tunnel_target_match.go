package apiserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/mitchellh/mapstructure"
	"github.com/openconfig/gnmic/pkg/config"
)

const (
	tunnelTargetMatchesPath = "tunnel-target-matches"
)

func (s *Server) handleConfigTunnelTargetMatchesGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		targets, err := s.configStore.List(tunnelTargetMatchesPath)
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
	tc, ok, err := s.configStore.Get(tunnelTargetMatchesPath, id)
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

func (s *Server) handleConfigTunnelTargetMatchesPost(w http.ResponseWriter, r *http.Request) {
	dec := json.NewDecoder(r.Body)
	defer r.Body.Close()
	var m map[string]any
	if err := dec.Decode(&m); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	tc := new(config.TunnelTargetMatch)
	if err := mapstructure.Decode(m, tc); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	_, err := s.configStore.Set(tunnelTargetMatchesPath, tc.ID, tc)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleConfigTunnelTargetMatchesDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	ok, _, err := s.configStore.Delete(tunnelTargetMatchesPath, id)
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
	w.WriteHeader(http.StatusOK)
}
