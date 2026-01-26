package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
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
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"clustering config not found"}})
		return
	}
	clustering, ok := clusteringCfg.(*config.Clustering)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"clustering config is not a config.Clustering"}})
		return
	}
	if clustering == nil {
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
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	services, err := s.locker.GetServices(r.Context(), fmt.Sprintf("%s-gnmic-api", clustering.ClusterName), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
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
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
}

func (s *Server) handleClusterRebalance(w http.ResponseWriter, r *http.Request) {
	isLeader, err := s.clusterManager.IsLeader(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	if !isLeader {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{"not leader"}})
		return
	}
	err = s.clusterManager.RebalanceTargetsV2()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	s.logger.Info("rebalance targets completed")
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handleClusteringLeaderGet(w http.ResponseWriter, r *http.Request) {
	clusteringCfg, ok, err := s.configStore.Get("clustering", "clustering")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{"clustering config not found"}})
		return
	}
	clustering, ok := clusteringCfg.(*config.Clustering)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{"clustering config is not a config.Clustering"}})
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	// get leader
	leader, err := s.clusterManager.GetLeaderName(ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}

	services, err := s.locker.GetServices(ctx, fmt.Sprintf("%s-gnmic-api", clustering.ClusterName), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}

	instanceNodes, err := s.clusterManager.GetInstanceToTargetsMapping(ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}

	members := make([]clusterMember, 1)
	for _, s := range services {
		if strings.TrimSuffix(s.ID, "-api") != leader {
			continue
		}
		scheme := cluster_manager.GetAPIScheme(&cluster_manager.Member{Labels: s.Tags})
		// add the leader as a member then break from loop
		members[0].APIEndpoint = fmt.Sprintf("%s://%s", scheme, s.Address)
		members[0].Name = strings.TrimSuffix(s.ID, "-api")
		members[0].IsLeader = true
		members[0].NumberOfLockedTargets = len(instanceNodes[members[0].Name])
		members[0].LockedTargets = instanceNodes[members[0].Name]
		break
	}
	b, err := json.Marshal(members)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	w.Write(b)
}

func (s *Server) handleClusteringLeaderDelete(w http.ResponseWriter, r *http.Request) {
	leader, err := s.clusterManager.IsLeader(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	if !leader {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{"not leader"}})
		return
	}
	err = s.clusterManager.WithdrawLeader(r.Context(), 30*time.Second)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
}

func (s *Server) handleClusteringMembersGet(w http.ResponseWriter, r *http.Request) {
	// clusteringResponse
	clusteringCfg, ok, err := s.configStore.Get("clustering", "clustering")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{"clustering config not found"}})
		return
	}
	clustering, ok := clusteringCfg.(*config.Clustering)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{"clustering config is not a config.Clustering"}})
		return
	}
	// get leader
	leader, err := s.clusterManager.GetLeaderName(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}

	services, err := s.locker.GetServices(r.Context(), fmt.Sprintf("%s-gnmic-api", clustering.ClusterName), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}

	instanceNodes, err := s.clusterManager.GetInstanceToTargetsMapping(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	members := make([]clusterMember, len(services))
	for i, s := range services {
		scheme := cluster_manager.GetAPIScheme(&cluster_manager.Member{Labels: s.Tags})
		members[i].APIEndpoint = fmt.Sprintf("%s://%s", scheme, s.Address)
		members[i].Name = strings.TrimSuffix(s.ID, "-api")
		members[i].IsLeader = leader == members[i].Name
		members[i].NumberOfLockedTargets = len(instanceNodes[members[i].Name])
		members[i].LockedTargets = instanceNodes[members[i].Name]
	}
	b, err := json.Marshal(members)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	_, err = w.Write(b)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
}

func (s *Server) handleClusteringDrainInstance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{"member id is required"}})
		return
	}
	leader, err := s.clusterManager.IsLeader(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	if !leader {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{"not leader"}})
		return
	}
	err = s.clusterManager.DrainMember(r.Context(), id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	w.WriteHeader(http.StatusOK)
}

type moveRequest struct {
	Target            string `json:"target,omitempty"`
	DestinationMember string `json:"member,omitempty"`
}

func (s *Server) handleClusterMove(w http.ResponseWriter, r *http.Request) {
	// read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	defer r.Body.Close()
	var moveRequest moveRequest

	err = json.Unmarshal(body, &moveRequest)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	if moveRequest.Target == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{"target is required"}})
		return
	}
	if moveRequest.DestinationMember == "" {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{"member is required"}})
		return
	}
	// TODO: implement move target
	// err = s.clusterManager.MoveTarget(r.Context(), moveRequest.Target, moveRequest.DestinationMember)
	// if err != nil {
	// 	w.WriteHeader(http.StatusInternalServerError)
	// 	_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
	// 	return
	// }
	// w.WriteHeader(http.StatusOK)
}

// //
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	res, err := s.configStore.GetAll()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	configs := make(map[string]any)
	for k, v := range res {
		configs[k] = v
	}
	sanitizedRes := sanitizeConfig(configs)
	err = json.NewEncoder(w).Encode(sanitizedRes)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
}

func sanitizeConfig(res map[string]any) map[string]any {
	keys := []string{
		"api-server",
		"gnmi-server",
		"loader",
		"clustering",
		"global-flags",
		"tunnel-server",
	}

	for _, key := range keys {
		val, ok := res[key]
		if !ok {
			continue
		}
		switch v := val.(type) {
		case map[string]any:
			res[key] = v[key]
		default:
		}
	}

	return res
}

func (s *Server) handleHealthzGet(w http.ResponseWriter, r *http.Request) {
	res := map[string]string{"status": "healthy"}
	b, err := json.Marshal(res)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	_, err = w.Write(b)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
	}
}

func (s *Server) handleAdminShutdown(w http.ResponseWriter, r *http.Request) {
	// Not implemented yet
	w.WriteHeader(http.StatusNotImplemented)
	_ = json.NewEncoder(w).Encode(APIErrors{Errors: []string{"shutdown not implemented"}})
}
