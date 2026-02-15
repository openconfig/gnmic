package apiserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/gorilla/mux"
)

type ProcessorConfigResponse struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Config any    `json:"config"`
}

type ProcessorConfigRequest struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Config any    `json:"config"`
}

func (s *Server) handleConfigProcessorsGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id != "" {
		processor, ok, err := s.store.Config.Get("processors", id)
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
		processorConfig := ProcessorConfigResponse{
			Name: id,
		}
		for k, v := range processor.(map[string]any) {
			switch v.(type) {
			case map[string]any:
				processorConfig.Type = k
				processorConfig.Config = v
			default:
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(APIErrors{Errors: []string{fmt.Sprintf("unknown processor type: %T", v)}})
				return
			}
		}
		err = json.NewEncoder(w).Encode(processorConfig)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
		return
	}
	processors, err := s.store.Config.List("processors")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	processorConfigs := make([]ProcessorConfigResponse, 0, len(processors))
	for name, processor := range processors {
		switch processor := processor.(type) {
		case map[string]any:
			processorConfig := ProcessorConfigResponse{
				Name: name,
			}
			for k, v := range processor {
				switch v.(type) {
				case map[string]any:
					processorConfig.Type = k
					processorConfig.Config = v
				default:
					w.WriteHeader(http.StatusInternalServerError)
					json.NewEncoder(w).Encode(APIErrors{Errors: []string{fmt.Sprintf("unknown processor type: %T", v)}})
					return
				}
				break
			}
			processorConfigs = append(processorConfigs, processorConfig)
		default:
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{fmt.Sprintf("unknown processor type: %T", processor)}})
			return
		}
	}
	err = json.NewEncoder(w).Encode(processorConfigs)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
}

func (s *Server) handleConfigProcessorsPost(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	defer r.Body.Close()
	cfg := new(ProcessorConfigRequest)
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
	if cfg.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"processor name is required"}})
		return
	}
	if cfg.Type == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"processor type is required"}})
		return
	}
	storeCfg := map[string]any{
		cfg.Type: cfg.Config,
	}
	_, err = s.store.Config.Set("processors", cfg.Name, storeCfg)
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
	if s.outputsManager.ProcessorInUse(id) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"processor is in use by outputs"}})
		return
	}
	if s.inputsManager.ProcessorInUse(id) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"processor is in use by inputs"}})
		return
	}
	_, _, err := s.store.Config.Delete("processors", id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	w.WriteHeader(http.StatusOK)
}
