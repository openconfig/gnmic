package apiserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/gorilla/mux"
	"github.com/openconfig/gnmic/pkg/inputs"
)

func (s *Server) handleConfigInputsGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		inputs, err := s.configStore.List("inputs")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
		err = json.NewEncoder(w).Encode(inputs)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
	} else {
		input, ok, err := s.configStore.Get("inputs", id)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{"input not found"}})
			return
		}
		err = json.NewEncoder(w).Encode(input)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
	}
}

func (s *Server) handleConfigInputsPost(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	defer r.Body.Close()
	cfg := map[string]any{}
	fmt.Println("body", string(body))
	err = json.Unmarshal(body, &cfg)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Println("err", err)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	if cfg == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"invalid input config"}})
		return
	}
	inputType, ok := cfg["type"].(string)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"input type is required"}})
		return
	}
	if !slices.Contains(inputs.InputTypes, inputType) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{fmt.Sprintf("unknown input type: %q", inputType)}})
		return
	}
	inputName, ok := cfg["name"].(string)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"input name is required"}})
		return
	}
	if inputName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"input name is required"}})
		return
	}
	initializer := inputs.Inputs[inputType]
	if initializer == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{fmt.Sprintf("unknown input type: %q", inputType)}})
		return
	}
	impl := initializer()
	err = impl.Validate(cfg)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	// validate event processors exist
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
	// validate outputs exist
	outs, ok := cfg["outputs"].([]string)
	if ok {
		for _, out := range outs {
			_, ok, err := s.configStore.Get("outputs", out)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
				return
			}
			if !ok {
				w.WriteHeader(http.StatusNotFound)
				json.NewEncoder(w).Encode(APIErrors{Errors: []string{fmt.Sprintf("output %s not found", out)}})
				return
			}
		}
	}
	_, err = s.configStore.Set("inputs", inputName, cfg)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleConfigInputsDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	ok, _, err := s.configStore.Delete("inputs", id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"input not found"}})
		return
	}
	w.WriteHeader(http.StatusOK)
}
