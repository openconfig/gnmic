// © 2022 Nokia.
//
// This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
// No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
// This code is provided on an “as is” basis without any warranties of any kind.
//
// SPDX-License-Identifier: Apache-2.0

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
	a.healthRoutes(apiV1)
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
	r.HandleFunc("/config/targets/{id}/subscriptions", a.handleConfigTargetsSubscriptions).Methods(http.MethodPatch)
	// config/subscriptions
	r.HandleFunc("/config/subscriptions", a.handleConfigSubscriptions).Methods(http.MethodGet)
	// config/outputs
	r.HandleFunc("/config/outputs", a.handleConfigOutputs).Methods(http.MethodGet)
	// config/inputs
	r.HandleFunc("/config/inputs", a.handleConfigInputs).Methods(http.MethodGet)
	// config/processors
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

func (a *App) healthRoutes(r *mux.Router) {
	r.HandleFunc("/healthz", a.handleHealthzGet).Methods(http.MethodGet)
}
