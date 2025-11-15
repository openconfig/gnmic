package apiserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/openconfig/gnmic/pkg/outputs"
)

// get all outputs
// curl command:
// curl http://localhost:8080/api/v1/outputs
func (s *Server) handleConfigOutputsGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		outputs, err := s.configStore.List("outputs")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
		err = json.NewEncoder(w).Encode(outputs)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
	} else {
		output, ok, err := s.configStore.Get("outputs", id)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{"output not found"}})
			return
		}
		err = json.NewEncoder(w).Encode(output)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
	}
}

// sample body:
//
//	{
//		"name": "output1",
//		"type": "file",
//		"filename": "output.txt"
//	}
//
// curl command:
// curl --request POST -H "Content-Type: application/json" \
// -d '{"name": "output1", "type": "file", "filename": "output.txt"}' \
// http://localhost:8080/api/v1/outputs
func (s *Server) handleConfigOutputsPost(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
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
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"invalid output config"}})
		return
	}
	outputName, ok := cfg["name"].(string)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"output name is required"}})
		return
	}
	if outputName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"output name is required"}})
		return
	}
	initializer := outputs.Outputs[cfg["type"].(string)]
	if initializer == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"unknown output type"}})
		return
	}
	impl := initializer()
	err = impl.Validate(cfg)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	evps, ok := cfg["event-processors"].([]string)
	if ok {
		for _, ep := range evps {
			_, ok, err := s.configStore.Get("processors", ep)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
				return
			}
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(APIErrors{Errors: []string{fmt.Sprintf("event processor %s not found", ep)}})
				return
			}
		}
	}
	_, err = s.configStore.Set("outputs", outputName, cfg)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleConfigOutputsPatch(w http.ResponseWriter, r *http.Request) {
	// TODO:
}

func (s *Server) handleConfigOutputsDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	err := s.outputsManager.StopOutput(id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	ok, _, err := s.configStore.Delete("outputs", id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"output not found"}})
		return
	}
	w.WriteHeader(http.StatusOK)
}
