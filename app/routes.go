package app

import (
	"net/http"

	"github.com/gorilla/mux"
)

func (a *App) routes() {
	apiV1 := a.router.PathPrefix("/api/v1").Subrouter()
	a.clusterRoutes(apiV1)
	a.configRoutes(apiV1)
	a.targetRoutes(apiV1)

}

func (a *App) clusterRoutes(r *mux.Router) {
	r.HandleFunc("/cluster", a.handleClusteringGet).Methods(http.MethodGet)
	r.HandleFunc("/cluster/members", a.handleClusteringMembersGet).Methods(http.MethodGet)
	r.HandleFunc("/cluster/leader", a.handleClusteringLeaderGet).Methods(http.MethodGet)
}

func (a *App) configRoutes(r *mux.Router) {
	// config
	r.HandleFunc("/config", a.handleConfig).Methods(http.MethodGet)
	// config/targets
	r.HandleFunc("/config/targets", a.handleConfigTargetsGet).Methods(http.MethodGet)
	r.HandleFunc("/config/targets/{id}", a.handleConfigTargetsGet).Methods(http.MethodGet)
	r.HandleFunc("/config/targets", a.handleConfigTargetsPost).Methods(http.MethodPost)
	r.HandleFunc("/config/targets/{id}", a.handleConfigTargetsDelete).Methods(http.MethodDelete)
	// config/subscriptions
	r.HandleFunc("/config/subscriptions", a.handleConfigSubscriptions).Methods(http.MethodGet)
	// config/outputs
	r.HandleFunc("/config/outputs", a.handleConfigOutputs).Methods(http.MethodGet)
	// config/inputs
	r.HandleFunc("/config/inputs", a.handleConfigInputs).Methods(http.MethodGet)
	// config/processsors
	r.HandleFunc("/config/processors", a.handleConfigProcessors).Methods(http.MethodGet)
	// config/clustering
	r.HandleFunc("/config/clustering", a.handleConfigClustering).Methods(http.MethodGet)
	// config/api-server
	r.HandleFunc("/config/api-server", a.handleConfigAPIServer).Methods(http.MethodGet)
	// config/gnmi-server
	r.HandleFunc("/config/gnmi-server", a.handleConfigGNMIServer).Methods(http.MethodGet)
}

func (a *App) targetRoutes(r *mux.Router) {
	// targets
	r.HandleFunc("/targets", a.handleTargetsGet).Methods(http.MethodGet)
	r.HandleFunc("/targets/{id}", a.handleTargetsGet).Methods(http.MethodGet)
	r.HandleFunc("/targets/{id}", a.handleTargetsPost).Methods(http.MethodPost)
	r.HandleFunc("/targets/{id}", a.handleTargetsDelete).Methods(http.MethodDelete)
}
