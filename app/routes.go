package app

import "net/http"

func (a *App) routes() {
	a.configRoutes()
	a.targetRoutes()
}

func (a *App) configRoutes() {
	// config
	a.router.HandleFunc("/config", a.handleConfig).Methods(http.MethodGet)
	// config/targets
	a.router.HandleFunc("/config/targets", a.handleConfigTargetsGet).Methods(http.MethodGet)
	a.router.HandleFunc("/config/targets/{id}", a.handleConfigTargetsGet).Methods(http.MethodGet)
	a.router.HandleFunc("/config/targets", a.handleConfigTargetsPost).Methods(http.MethodPost)
	a.router.HandleFunc("/config/targets/{id}", a.handleConfigTargetsDelete).Methods(http.MethodDelete)
	// config/subscriptions
	a.router.HandleFunc("/config/subscriptions", a.handleConfigSubscriptions).Methods(http.MethodGet)
	// config/outputs
	a.router.HandleFunc("/config/outputs", a.handleConfigOutputs).Methods(http.MethodGet)
	// config/inputs
	a.router.HandleFunc("/config/inputs", a.handleConfigInputs).Methods(http.MethodGet)
	// config/processors
	a.router.HandleFunc("/config/processors", a.handleConfigProcessors).Methods(http.MethodGet)
	// config/clustering
	a.router.HandleFunc("/config/clustering", a.handleConfigClustering).Methods(http.MethodGet)
	// config/api-server
	a.router.HandleFunc("/config/api-server", a.handleConfigAPIServer).Methods(http.MethodGet)
	// config/gnmi-server
	a.router.HandleFunc("/config/gnmi-server", a.handleConfigGNMIServer).Methods(http.MethodGet)
}

func (a *App) targetRoutes() {
	// targets
	a.router.HandleFunc("/targets", a.handleTargetsGet).Methods(http.MethodGet)
	a.router.HandleFunc("/targets/{id}", a.handleTargetsGet).Methods(http.MethodGet)
	a.router.HandleFunc("/targets/{id}", a.handleTargetsPost).Methods(http.MethodPost)
	a.router.HandleFunc("/targets/{id}", a.handleTargetsDelete).Methods(http.MethodDelete)
}
