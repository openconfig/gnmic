package apiserver

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gorilla/mux"
)

func (s *Server) handleConfigInputsGet(w http.ResponseWriter, r *http.Request) {
	inputs, err := s.configStore.List("inputs")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(inputs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleConfigInputsPost(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, "invalid input config", http.StatusBadRequest)
		return
	}
	inputName, ok := cfg["name"].(string)
	if !ok {
		http.Error(w, "input name is required", http.StatusBadRequest)
		return
	}
	if inputName == "" {
		http.Error(w, "input name is required", http.StatusBadRequest)
		return
	}
	ok, err = s.configStore.Set("inputs", inputName, cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "input already exists", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleConfigInputsDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	ok, _, err := s.configStore.Delete("inputs", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "input not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}
