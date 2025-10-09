package apiserver

import (
	"net/http"

	"github.com/gorilla/mux"
)

func (s *Server) routes() {
	apiV1 := s.router.PathPrefix("/api/v1").Subrouter()
	s.clusterRoutes(apiV1)
	s.configRoutes(apiV1)
	s.targetRoutes(apiV1)
	s.subscriptionRoutes(apiV1)
	s.healthRoutes(apiV1)
	s.adminRoutes(apiV1)
	s.outputsRoutes(apiV1)
	s.inputsRoutes(apiV1)
	s.processorsRoutes(apiV1)
	s.assignmentRoutes(apiV1)
}

func (s *Server) healthRoutes(r *mux.Router) {
	r.HandleFunc("/healthz", s.handleHealthzGet).Methods(http.MethodGet)
}

func (s *Server) adminRoutes(r *mux.Router) {
	r.HandleFunc("/admin/shutdown", s.handleAdminShutdown).Methods(http.MethodPost)
}

func (s *Server) clusterRoutes(r *mux.Router) {
	r.HandleFunc("/cluster", s.handleClusteringGet).Methods(http.MethodGet)
	r.HandleFunc("/cluster/rebalance", s.handleClusterRebalance).Methods(http.MethodPost)
	r.HandleFunc("/cluster/leader", s.handleClusteringLeaderGet).Methods(http.MethodGet)
	r.HandleFunc("/cluster/leader", s.handleClusteringLeaderDelete).Methods(http.MethodDelete)
	r.HandleFunc("/cluster/members", s.handleClusteringMembersGet).Methods(http.MethodGet)
	r.HandleFunc("/cluster/members/{id}/drain", s.handleClusteringDrainInstance).Methods(http.MethodPost)
}

func (s *Server) configRoutes(r *mux.Router) {
	r.HandleFunc("/config", s.handleConfig).Methods(http.MethodGet)
	r.HandleFunc("/config/targets", s.handleConfigTargetsGet).Methods(http.MethodGet)
	r.HandleFunc("/config/targets/{id}", s.handleConfigTargetsGet).Methods(http.MethodGet)
	r.HandleFunc("/config/targets", s.handleConfigTargetsPost).Methods(http.MethodPost)
	r.HandleFunc("/config/targets/{id}", s.handleConfigTargetsDelete).Methods(http.MethodDelete)
	r.HandleFunc("/config/targets/{id}/subscriptions", s.handleConfigTargetsSubscriptionsPatch).Methods(http.MethodPatch)
	r.HandleFunc("/config/targets/{id}/outputs", s.handleConfigTargetsOutputsPatch).Methods(http.MethodPatch)
	r.HandleFunc("/config/targets/{id}/state", s.handleTargetsStatePost).Methods(http.MethodPost)
	//
	r.HandleFunc("/config/subscriptions", s.handleConfigSubscriptionsGet).Methods(http.MethodGet)
	r.HandleFunc("/config/subscriptions", s.handleConfigSubscriptionsPost).Methods(http.MethodPost)
	r.HandleFunc("/config/subscriptions/{id}", s.handleConfigSubscriptionsGet).Methods(http.MethodGet)
	r.HandleFunc("/config/subscriptions/{id}", s.handleConfigSubscriptionsDelete).Methods(http.MethodDelete)
	//
	r.HandleFunc("/config/outputs", s.handleConfigOutputsGet).Methods(http.MethodGet)
	r.HandleFunc("/config/outputs", s.handleConfigOutputsPost).Methods(http.MethodPost)
	r.HandleFunc("/config/outputs/{id}", s.handleConfigOutputsGet).Methods(http.MethodGet)
	r.HandleFunc("/config/outputs/{id}/", s.handleConfigOutputsPatch).Methods(http.MethodPatch)
	r.HandleFunc("/config/outputs/{id}", s.handleConfigOutputsDelete).Methods(http.MethodDelete)
	//
	r.HandleFunc("/config/inputs", s.handleConfigInputsGet).Methods(http.MethodGet)
	r.HandleFunc("/config/inputs", s.handleConfigInputsPost).Methods(http.MethodPost)
	r.HandleFunc("/config/inputs/{id}", s.handleConfigInputsGet).Methods(http.MethodGet)
	r.HandleFunc("/config/inputs/{id}", s.handleConfigInputsDelete).Methods(http.MethodDelete)
	//
	r.HandleFunc("/config/processors", s.handleConfigProcessorsGet).Methods(http.MethodGet)
	r.HandleFunc("/config/processors", s.handleConfigProcessorsPost).Methods(http.MethodPost)
	r.HandleFunc("/config/processors/{id}", s.handleConfigProcessorsGet).Methods(http.MethodGet)
	r.HandleFunc("/config/processors/{id}", s.handleConfigProcessorsDelete).Methods(http.MethodDelete)
}

func (s *Server) targetRoutes(r *mux.Router) {
	r.HandleFunc("/targets", s.handleTargetsGet).Methods(http.MethodGet)
	r.HandleFunc("/targets/{id}", s.handleTargetsGet).Methods(http.MethodGet)
	r.HandleFunc("/targets/{id}/state/{state}", s.handleTargetsStatePost).Methods(http.MethodPost)
}

func (s *Server) subscriptionRoutes(r *mux.Router) {
	// r.HandleFunc("/subscriptions", c.handleSubscriptionsGet).Methods(http.MethodGet)
	// r.HandleFunc("/subscriptions", c.handleSubscriptionsPost).Methods(http.MethodPost)
	// r.HandleFunc("/subscriptions/{id}", c.handleSubscriptionsGet).Methods(http.MethodGet)
	// r.HandleFunc("/subscriptions/{id}", c.handleSubscriptionsDelete).Methods(http.MethodDelete)
}

func (s *Server) outputsRoutes(r *mux.Router) {
	// r.HandleFunc("/outputs", c.handleOutputsGet).Methods(http.MethodGet)
	// r.HandleFunc("/outputs", c.handleOutputsPost).Methods(http.MethodPost)
	// r.HandleFunc("/outputs/{id}", c.handleOutputsGet).Methods(http.MethodGet)
	// r.HandleFunc("/outputs/{id}", c.handleOutputsPatch).Methods(http.MethodPatch)
	// r.HandleFunc("/outputs/{id}", c.handleOutputsDelete).Methods(http.MethodDelete)
}

func (s *Server) inputsRoutes(r *mux.Router) {
	// r.HandleFunc("/inputs", c.handleInputsGet).Methods(http.MethodGet)
	// r.HandleFunc("/inputs", c.handleInputsPost).Methods(http.MethodPost)
	// r.HandleFunc("/inputs/{id}", c.handleInputsGet).Methods(http.MethodGet)
	// r.HandleFunc("/inputs/{id}", c.handleInputsDelete).Methods(http.MethodDelete)
}

func (s *Server) processorsRoutes(r *mux.Router) {
	// r.HandleFunc("/processors", c.handleProcessorsGet).Methods(http.MethodGet)
	// r.HandleFunc("/processors", c.handleProcessorsPost).Methods(http.MethodPost)
	// r.HandleFunc("/processors/{id}", c.handleProcessorsGet).Methods(http.MethodGet)
	// r.HandleFunc("/processors/{id}", c.handleProcessorsDelete).Methods(http.MethodDelete)
}

func (s *Server) assignmentRoutes(r *mux.Router) {
	r.HandleFunc("/assignments", s.handleAssignmentGet).Methods(http.MethodGet)
	r.HandleFunc("/assignments", s.handleAssignmentPost).Methods(http.MethodPost)
	r.HandleFunc("/assignments/{id}", s.handleAssignmentGet).Methods(http.MethodGet)
	r.HandleFunc("/assignments/{id}", s.handleAssignmentDelete).Methods(http.MethodDelete)
}
