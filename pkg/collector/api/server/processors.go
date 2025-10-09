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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(processors)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if cfg == nil {
		http.Error(w, "invalid processor config", http.StatusBadRequest)
		return
	}
	processorName, ok := cfg["name"].(string)
	if !ok {
		http.Error(w, "processor name is required", http.StatusBadRequest)
		return
	}
	if processorName == "" {
		http.Error(w, "processor name is required", http.StatusBadRequest)
		return
	}
	_, err = s.configStore.Set("processors", processorName, cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleConfigProcessorsDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	ok, _, err := s.configStore.Delete("processors", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "processor not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}
