package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func (c *CLI) routes() {
	c.router.HandleFunc("/health", c.handleHealth).Methods(http.MethodGet)
	c.router.HandleFunc("/config", c.handleConfig).Methods(http.MethodGet)
	c.router.HandleFunc("/config/global", c.handleConfigGlobal).Methods(http.MethodGet)
	c.router.HandleFunc("/config/targets", c.handleConfigTargets).Methods(http.MethodGet)
	c.router.HandleFunc("/config/subscriptions", c.handleConfigSubscriptions).Methods(http.MethodGet)
	c.router.HandleFunc("/config/outputs", c.handleConfigOutputs).Methods(http.MethodGet)
	c.router.HandleFunc("/config/inputs", c.handleConfigInputs).Methods(http.MethodGet)
	c.router.HandleFunc("/config/processors", c.handleConfigProcessors).Methods(http.MethodGet)
	c.router.HandleFunc("/config/locker", c.handleConfigLocker).Methods(http.MethodGet)
	c.router.HandleFunc("/targets", c.handleTargets).Methods(http.MethodGet)
}

func (c *CLI) handleHealth(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "%s", "ok")
}

func (c *CLI) handleConfigTargets(w http.ResponseWriter, r *http.Request) {
	targets, err := c.config.GetTargets()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
	err = json.NewEncoder(w).Encode(targets)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
}

func (c *CLI) handleConfigSubscriptions(w http.ResponseWriter, r *http.Request) {
	subsc, err := c.config.GetSubscriptions(nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
	err = json.NewEncoder(w).Encode(subsc)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
}

func (c *CLI) handleConfigOutputs(w http.ResponseWriter, r *http.Request) {
	outputs, err := c.config.GetOutputs()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
	err = json.NewEncoder(w).Encode(outputs)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
}

func (c *CLI) handleConfigLocker(w http.ResponseWriter, r *http.Request) {
	locker, err := c.config.GetLocker()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
	err = json.NewEncoder(w).Encode(locker)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
}

func (c *CLI) handleConfigInputs(w http.ResponseWriter, r *http.Request) {
	inputs, err := c.config.GetInputs()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
	err = json.NewEncoder(w).Encode(inputs)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
}

func (c *CLI) handleConfigProcessors(w http.ResponseWriter, r *http.Request) {
	evps, err := c.config.GetEventProcessors()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
	err = json.NewEncoder(w).Encode(evps)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
}

func (c *CLI) handleConfig(w http.ResponseWriter, r *http.Request) {
	configMap := make(map[string]interface{})
	b, err := json.Marshal(c.config.Globals)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
	err = json.Unmarshal(b, &configMap)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
	//
	b, err = json.Marshal(c.config.LocalFlags)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
	err = json.Unmarshal(b, &configMap)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
	//
	targets, err := c.config.GetTargets()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
	configMap["targets"] = targets
	//
	outputs, err := c.config.GetOutputs()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
	configMap["outputs"] = outputs
	//
	evp, err := c.config.GetEventProcessors()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
	configMap["processors"] = evp
	//
	locker, err := c.config.GetLocker()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
	configMap["locker"] = locker
	err = json.NewEncoder(w).Encode(configMap)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
}

func (c *CLI) handleConfigGlobal(w http.ResponseWriter, r *http.Request) {
	err := json.NewEncoder(w).Encode(c.config.Globals)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
}

func (c *CLI) handleTargets(w http.ResponseWriter, r *http.Request) {
	err := json.NewEncoder(w).Encode(c.collector.Targets)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err)
	}
}
