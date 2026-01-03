package apiserver

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/mitchellh/mapstructure"
	"github.com/openconfig/gnmic/pkg/api/types"
)

// Apply request is a request to apply the configuration to the collector.
// Any object that is not provided in the request is deleted.
type ConfigApplyRequest struct {
	Targets       map[string]*types.TargetConfig       `json:"targets"`
	Subscriptions map[string]*types.SubscriptionConfig `json:"subscriptions"`
	Outputs       map[string]map[string]any            `json:"outputs"`
	Inputs        map[string]map[string]any            `json:"inputs"`
	Processors    map[string]map[string]any            `json:"processors"`
}

func validateApplyRequest(req *ConfigApplyRequest) error {
	if len(req.Targets) == 0 && len(req.Subscriptions) == 0 && len(req.Outputs) == 0 && len(req.Inputs) == 0 && len(req.Processors) == 0 {
		return nil // valid reset request
	}
	if len(req.Targets) == 0 && len(req.Inputs) == 0 {
		return errors.New("at least one of targets or inputs is required")
	}
	if len(req.Targets) > 0 && len(req.Subscriptions) == 0 {
		return errors.New("if targets are provided, at least one subscription is required")
	}
	if len(req.Inputs) > 0 && len(req.Outputs) == 0 {
		return errors.New("if inputs are provided, at least one output is required")
	}
	// TODO: validate each config
	return nil
}

func (s *Server) handleConfigApply(w http.ResponseWriter, r *http.Request) {
	s.applyLock.Lock()
	defer s.applyLock.Unlock()

	var reader io.Reader = r.Body
	// if content is gzip, decompress it
	if r.Header.Get("Content-Encoding") == "gzip" {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{"gzip error: " + err.Error()}})
			return
		}
		defer gz.Close()
		reader = gz
	}
	req, err := decodeRequest(reader)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"decode error: " + err.Error()}})
		return
	}

	err = validateApplyRequest(req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"validate error: " + err.Error()}})
		return
	}
	// delete subscriptions
	existingSubscriptions, err := s.configStore.Keys("subscriptions")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"get subscriptions error: " + err.Error()}})
		return
	}
	for _, name := range existingSubscriptions {
		if _, ok := req.Subscriptions[name]; !ok {
			_, _, err := s.configStore.Delete("subscriptions", name)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
				return
			}
		}
	}
	// delete targets
	existingTargets, err := s.configStore.Keys("targets")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"get targets error: " + err.Error()}})
		return
	}
	for _, name := range existingTargets {
		if _, ok := req.Targets[name]; !ok {
			_, _, err := s.configStore.Delete("targets", name)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(APIErrors{Errors: []string{"delete target error: " + err.Error()}})
				return
			}
		}
	}
	// delete inputs
	existingInputs, err := s.configStore.Keys("inputs")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"get inputs error: " + err.Error()}})
		return
	}
	for _, name := range existingInputs {
		if _, ok := req.Inputs[name]; !ok {
			_, _, err := s.configStore.Delete("inputs", name)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(APIErrors{Errors: []string{"delete input error: " + err.Error()}})
				return
			}
		}
	}
	// delete outputs
	existingOutputs, err := s.configStore.Keys("outputs")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"get outputs error: " + err.Error()}})
		return
	}
	for _, name := range existingOutputs {
		if _, ok := req.Outputs[name]; !ok {
			_, _, err := s.configStore.Delete("outputs", name)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(APIErrors{Errors: []string{"delete output error: " + err.Error()}})
				return
			}
		}
	}
	// delete processors
	existingProcessors, err := s.configStore.Keys("processors")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"get processors error: " + err.Error()}})
		return
	}
	for _, name := range existingProcessors {
		if _, ok := req.Processors[name]; !ok {
			_, _, err := s.configStore.Delete("processors", name)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				json.NewEncoder(w).Encode(APIErrors{Errors: []string{"delete processor error: " + err.Error()}})
				return
			}
		}
	}
	//
	// apply subscriptions
	for name, cfg := range req.Subscriptions {
		_, err = s.configStore.Set("subscriptions", name, cfg)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{"set subscription error: " + err.Error()}})
			return
		}
	}
	// apply processors
	for name, cfg := range req.Processors {
		_, err = s.configStore.Set("processors", name, cfg)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{"set processor error: " + err.Error()}})
			return
		}
	}
	// apply outputs
	for name, cfg := range req.Outputs {
		_, err = s.configStore.Set("outputs", name, cfg)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{"set output error: " + err.Error()}})
			return
		}
	}
	// apply targets
	for name, cfg := range req.Targets {
		_, err = s.configStore.Set("targets", name, cfg)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{"set target error: " + err.Error()}})
			return
		}
	}
	// apply inputs
	for name, cfg := range req.Inputs {
		_, err = s.configStore.Set("inputs", name, cfg)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{"set input error: " + err.Error()}})
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func decodeRequest(reader io.Reader) (*ConfigApplyRequest, error) {
	dec := json.NewDecoder(reader)
	reqMap := make(map[string]any)
	err := dec.Decode(&reqMap)
	if err != nil {
		return nil, err
	}
	req, err := decodeRequestMap(reqMap)
	if err != nil {
		return nil, err
	}
	return req, nil
}

func decodeRequestMap(reqMap map[string]any) (*ConfigApplyRequest, error) {
	req := new(ConfigApplyRequest)
	mdec, err := mapstructure.NewDecoder(
		&mapstructure.DecoderConfig{
			DecodeHook: mapstructure.StringToTimeDurationHookFunc(),
			Result:     req,
		},
	)
	if err != nil {
		return nil, err
	}
	err = mdec.Decode(reqMap)
	if err != nil {
		return nil, err
	}
	return req, nil
}
