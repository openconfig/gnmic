package app

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/karimra/gnmic/collector"
	"github.com/karimra/gnmic/config"
)

func (a *App) routes() {
	a.router.HandleFunc("/health", a.handleHealth).Methods(http.MethodGet)
	// config
	a.router.HandleFunc("/config", a.handleConfig).Methods(http.MethodGet)
	// config/global
	a.router.HandleFunc("/config/global", a.handleConfigGlobal).Methods(http.MethodGet)
	// config/targets
	a.router.HandleFunc("/config/targets", a.handleConfigTargetsGet).Methods(http.MethodGet)
	a.router.HandleFunc("/config/targets", a.handleConfigTargetsPost).Methods(http.MethodPost)
	// a.router.HandleFunc("/config/targets", a.handleConfigTargetsPut).Methods(http.MethodPut)
	a.router.HandleFunc("/config/targets", a.handleConfigTargetsDelete).Methods(http.MethodDelete)
	// config/subscriptions
	a.router.HandleFunc("/config/subscriptions", a.handleConfigSubscriptions).Methods(http.MethodGet)
	// config/outputs
	a.router.HandleFunc("/config/outputs", a.handleConfigOutputs).Methods(http.MethodGet)
	// config/inputs
	a.router.HandleFunc("/config/inputs", a.handleConfigInputs).Methods(http.MethodGet)
	// config/processors
	a.router.HandleFunc("/config/processors", a.handleConfigProcessors).Methods(http.MethodGet)
	// config/locker
	a.router.HandleFunc("/config/locker", a.handleConfigLocker).Methods(http.MethodGet)
	// targets
	a.router.HandleFunc("/targets", a.handleTargets).Methods(http.MethodGet)
	a.router.HandleFunc("/targets", a.handleTargets).Methods(http.MethodPost)
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "%s", "ok")
}

func (a *App) handleConfigTargetsGet(w http.ResponseWriter, r *http.Request) {
	targets, err := a.Config.GetTargets()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
	}
	err = json.NewEncoder(w).Encode(targets)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
	}
}

func (a *App) handleConfigTargetsPost(w http.ResponseWriter, r *http.Request) {}

func (a *App) handleConfigTargetsDelete(w http.ResponseWriter, r *http.Request) {}

func (a *App) handleConfigSubscriptions(w http.ResponseWriter, r *http.Request) {
	subsc, err := a.Config.GetSubscriptions(nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
	}
	err = json.NewEncoder(w).Encode(subsc)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
	}
}

func (a *App) handleConfigOutputs(w http.ResponseWriter, r *http.Request) {
	outputs, err := a.Config.GetOutputs()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
	}
	err = json.NewEncoder(w).Encode(outputs)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
	}
}

func (a *App) handleConfigLocker(w http.ResponseWriter, r *http.Request) {
	locker, err := a.Config.GetLocker()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
	}
	err = json.NewEncoder(w).Encode(locker)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
	}
}

func (a *App) handleConfigInputs(w http.ResponseWriter, r *http.Request) {
	inputs, err := a.Config.GetInputs()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
	}
	err = json.NewEncoder(w).Encode(inputs)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
	}
}

func (a *App) handleConfigProcessors(w http.ResponseWriter, r *http.Request) {
	evps, err := a.Config.GetEventProcessors()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
		return
	}
	err = json.NewEncoder(w).Encode(evps)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
		return
	}
}

func (a *App) handleConfig(w http.ResponseWriter, r *http.Request) {
	targets, err := a.Config.GetTargets()
	if err != nil {
		a.Logger.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
		return
	}
	//
	subscriptions, err := a.Config.GetSubscriptions(nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
	}
	//
	outputs, err := a.Config.GetOutputs()
	if err != nil {
		a.Logger.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
		return
	}
	//
	inputs, err := a.Config.GetInputs()
	if err != nil {
		a.Logger.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
		return
	}
	//
	evps, err := a.Config.GetEventProcessors()
	if err != nil {
		a.Logger.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
		return
	}
	//
	locker, err := a.Config.GetLocker()
	if err != nil {
		a.Logger.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
		return
	}
	cfgd := &cfg{
		a.Config.Globals,
		a.Config.LocalFlags,
		targets,
		subscriptions,
		outputs,
		inputs,
		evps,
		locker,
	}
	//
	json.NewEncoder(w).Encode(cfgd)
	if err != nil {
		a.Logger.Print(err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
		return
	}
}

func (a *App) handleConfigGlobal(w http.ResponseWriter, r *http.Request) {
	err := json.NewEncoder(w).Encode(a.Config.Globals)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
	}
}

func (a *App) handleTargets(w http.ResponseWriter, r *http.Request) {
	err := json.NewEncoder(w).Encode(a.collector.Targets)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
	}
}

type cfg struct {
	*config.GlobalFlags
	*config.LocalFlags
	Targets       map[string]*collector.TargetConfig
	Subscriptions map[string]*collector.SubscriptionConfig
	Outputs       map[string]map[string]interface{}
	Inputs        map[string]map[string]interface{}
	Processors    map[string]map[string]interface{}
	Locker        map[string]interface{}
}
