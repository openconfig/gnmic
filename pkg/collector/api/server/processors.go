package apiserver

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gorilla/mux"
)

func (s *Server) handleConfigProcessorsGet(w http.ResponseWriter, r *http.Request) {
	processors, err := s.configStore.List("processors")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	err = json.NewEncoder(w).Encode(processors)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
}

func (s *Server) handleConfigProcessorsPost(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()
	cfg := map[string]any{}
	err = json.Unmarshal(body, &cfg)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	if cfg == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"invalid processor config"}})
		return
	}
	processorName, ok := cfg["name"].(string)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"processor name is required"}})
		return
	}
	if processorName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"processor name is required"}})
		return
	}
	_, err = s.configStore.Set("processors", processorName, cfg)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleConfigProcessorsDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	ok, _, err := s.configStore.Delete("processors", id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"processor not found"}})
		return
	}
	w.WriteHeader(http.StatusOK)
}
