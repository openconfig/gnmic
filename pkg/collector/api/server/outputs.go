package apiserver

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gorilla/mux"
)

// get all outputs
// curl command:
// curl http://localhost:8080/api/v1/outputs
func (s *Server) handleConfigOutputsGet(w http.ResponseWriter, r *http.Request) {
	outputs, err := s.configStore.List("outputs")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(outputs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
		http.Error(w, "invalid output config", http.StatusBadRequest)
		return
	}
	outputName, ok := cfg["name"].(string)
	if !ok {
		http.Error(w, "output name is required", http.StatusBadRequest)
		return
	}
	if outputName == "" {
		http.Error(w, "output name is required", http.StatusBadRequest)
		return
	}
	ok, err = s.configStore.Set("outputs", outputName, cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "output already exists", http.StatusBadRequest)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleConfigOutputsPatch(w http.ResponseWriter, r *http.Request) {
	// vars := mux.Vars(r)
	// id := vars["id"]
	// body, err := io.ReadAll(r.Body)
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// 	return
	// }
	// defer r.Body.Close()
	// cfg := map[string]any{}
	// err = json.Unmarshal(body, &cfg)
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusBadRequest)
	// 	return
	// }
	// if cfg == nil {
	// 	http.Error(w, "invalid output config", http.StatusBadRequest)
	// 	return
	// }
	// _, err = c.configStore.Set("outputs", id, cfg)
	// if err != nil {
	// 	if errors.Is(err, store.ErrKeyNotFound) {
	// 		http.Error(w, "output not found", http.StatusNotFound)
	// 		return
	// 	}
	// 	http.Error(w, err.Error(), http.StatusInternalServerError)
	// 	return
	// }
	// w.WriteHeader(http.StatusOK)
}

func (s *Server) handleConfigOutputsDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	ok, _, err := s.configStore.Delete("outputs", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "output not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}
