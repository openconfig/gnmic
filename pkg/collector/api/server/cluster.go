package apiserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	cluster_manager "github.com/openconfig/gnmic/pkg/collector/managers/cluster"
	"github.com/openconfig/gnmic/pkg/config"
)

type clusteringResponse struct {
	ClusterName           string          `json:"name,omitempty"`
	NumberOfLockedTargets int             `json:"number-of-locked-targets"`
	Leader                string          `json:"leader,omitempty"`
	Members               []clusterMember `json:"members,omitempty"`
}

type clusterMember struct {
	Name                  string   `json:"name,omitempty"`
	APIEndpoint           string   `json:"api-endpoint,omitempty"`
	IsLeader              bool     `json:"is-leader,omitempty"`
	NumberOfLockedTargets int      `json:"number-of-locked-nodes"`
	LockedTargets         []string `json:"locked-targets,omitempty"`
}

func (s *Server) handleClusteringGet(w http.ResponseWriter, r *http.Request) {
	// clusteringResponse
	clusteringCfg, ok, err := s.configStore.Get("clustering", "clustering")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "clustering config not found", http.StatusNotFound)
		return
	}
	clustering, ok := clusteringCfg.(*config.Clustering)
	if !ok {
		http.Error(w, "clustering config is not a config.Clustering", http.StatusInternalServerError)
		return
	}
	cr := &clusteringResponse{
		ClusterName:           clustering.ClusterName,
		NumberOfLockedTargets: 0,
		Leader:                "",
		Members:               make([]clusterMember, 0),
	}
	cr.Leader, err = s.clusterManager.GetLeaderName(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	services, err := s.locker.GetServices(r.Context(), fmt.Sprintf("%s-gnmic-api", clustering.ClusterName), nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	instanceNodes, err := s.clusterManager.GetInstanceToTargetsMapping(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, v := range instanceNodes {
		cr.NumberOfLockedTargets += len(v)
	}

	cr.Members = make([]clusterMember, len(services))
	for i, srv := range services {
		scheme := cluster_manager.GetAPIScheme(&cluster_manager.Member{Labels: srv.Tags})
		cr.Members[i].APIEndpoint = fmt.Sprintf("%s://%s", scheme, srv.Address)
		cr.Members[i].Name = strings.TrimSuffix(srv.ID, "-api")
		cr.Members[i].IsLeader = cr.Leader == cr.Members[i].Name
		cr.Members[i].NumberOfLockedTargets = len(instanceNodes[cr.Members[i].Name])
		cr.Members[i].LockedTargets = instanceNodes[cr.Members[i].Name]
	}

	err = json.NewEncoder(w).Encode(cr)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *Server) handleClusterRebalance(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) handleClusteringLeaderGet(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) handleClusteringLeaderDelete(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) handleClusteringMembersGet(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) handleClusteringDrainInstance(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) handleHealthzGet(w http.ResponseWriter, r *http.Request) {
}

func (s *Server) handleAdminShutdown(w http.ResponseWriter, r *http.Request) {
}
