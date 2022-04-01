package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func (a *App) newAPIServer() (*http.Server, error) {
	a.routes()
	tlscfg, err := utils.NewTLSConfig(
		a.Config.APIServer.CaFile,
		a.Config.APIServer.CertFile,
		a.Config.APIServer.KeyFile,
		a.Config.APIServer.SkipVerify,
		true)
	if err != nil {
		return nil, err
	}
	if a.Config.APIServer.EnableMetrics {
		a.router.Handle("/metrics", promhttp.HandlerFor(a.reg, promhttp.HandlerOpts{}))
		a.reg.MustRegister(collectors.NewGoCollector())
		a.reg.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	}
	s := &http.Server{
		Addr:         a.Config.APIServer.Address,
		Handler:      a.router,
		ReadTimeout:  a.Config.APIServer.Timeout / 2,
		WriteTimeout: a.Config.APIServer.Timeout / 2,
	}

	if tlscfg != nil {
		s.TLSConfig = tlscfg
	}

	return s, nil
}

type APIErrors struct {
	Errors []string `json:"errors,omitempty"`
}

func (a *App) handleConfigTargetsGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	var err error
	a.configLock.RLock()
	defer a.configLock.RUnlock()
	if id == "" {
		err = json.NewEncoder(w).Encode(a.Config.Targets)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		}
		return
	}
	if t, ok := a.Config.Targets[id]; ok {
		err = json.NewEncoder(w).Encode(t)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		}
		return
	}
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(APIErrors{Errors: []string{fmt.Sprintf("target %q not found", id)}})
}

func (a *App) handleConfigTargetsPost(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	defer r.Body.Close()
	tc := new(types.TargetConfig)
	err = json.Unmarshal(body, tc)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	// if _, ok := a.Config.Targets[tc.Name]; ok {
	// 	w.WriteHeader(http.StatusBadRequest)
	// 	json.NewEncoder(w).Encode(APIErrors{Errors: []string{"target config already exists"}})
	// 	return
	// }
	// a.Config.Targets[tc.Name] = tc
	a.AddTargetConfig(tc)
}

func (a *App) handleConfigTargetsDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	err := a.DeleteTarget(a.ctx, id)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
}

func (a *App) handleConfigSubscriptions(w http.ResponseWriter, r *http.Request) {
	a.handlerCommonGet(w, r, a.Config.Subscriptions)
}

func (a *App) handleConfigOutputs(w http.ResponseWriter, r *http.Request) {
	a.handlerCommonGet(w, r, a.Config.Outputs)
}

func (a *App) handleConfigClustering(w http.ResponseWriter, r *http.Request) {
	a.handlerCommonGet(w, r, a.Config.Clustering)
}

func (a *App) handleConfigAPIServer(w http.ResponseWriter, r *http.Request) {
	a.handlerCommonGet(w, r, a.Config.APIServer)
}

func (a *App) handleConfigGNMIServer(w http.ResponseWriter, r *http.Request) {
	a.handlerCommonGet(w, r, a.Config.GnmiServer)
}

func (a *App) handleConfigInputs(w http.ResponseWriter, r *http.Request) {
	a.handlerCommonGet(w, r, a.Config.Inputs)
}

func (a *App) handleConfigProcessors(w http.ResponseWriter, r *http.Request) {
	a.handlerCommonGet(w, r, a.Config.Processors)
}

func (a *App) handleConfig(w http.ResponseWriter, r *http.Request) {
	a.handlerCommonGet(w, r, a.Config)
}

func (a *App) handleTargetsGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		a.handlerCommonGet(w, r, a.Targets)
		return
	}
	if t, ok := a.Targets[id]; ok {
		a.handlerCommonGet(w, r, t)
		return
	}
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(APIErrors{Errors: []string{"no targets found"}})
}

func (a *App) handleTargetsPost(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	tc, ok := a.Config.Targets[id]
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{fmt.Sprintf("target %q not found", id)}})
		return
	}
	go a.TargetSubscribeStream(a.ctx, tc)
}

func (a *App) handleTargetsDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if _, ok := a.Targets[id]; !ok {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{fmt.Sprintf("target %q not found", id)}})
		return
	}
	err := a.DeleteTarget(a.ctx, id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
}

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

func (a *App) handleClusteringGet(w http.ResponseWriter, r *http.Request) {
	if a.Config.Clustering == nil {
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	resp := new(clusteringResponse)
	resp.ClusterName = a.Config.ClusterName

	leaderKey := fmt.Sprintf("gnmic/%s/leader", a.Config.ClusterName)
	leader, err := a.locker.List(ctx, leaderKey)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	resp.Leader = leader[leaderKey]
	lockedNodesPrefix := fmt.Sprintf("gnmic/%s/targets", a.Config.ClusterName)

	lockedNodes, err := a.locker.List(a.ctx, lockedNodesPrefix)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	resp.NumberOfLockedTargets = len(lockedNodes)
	services, err := a.locker.GetServices(ctx, fmt.Sprintf("%s-gnmic-api", a.Config.ClusterName), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}

	instanceNodes := make(map[string][]string)
	for k, v := range lockedNodes {
		name := strings.TrimPrefix(k, fmt.Sprintf("gnmic/%s/targets/", a.Config.ClusterName))
		if _, ok := instanceNodes[v]; !ok {
			instanceNodes[v] = make([]string, 0)
		}
		instanceNodes[v] = append(instanceNodes[v], name)
	}
	resp.Members = make([]clusterMember, len(services))
	for i, s := range services {
		resp.Members[i].APIEndpoint = s.Address
		resp.Members[i].Name = strings.TrimSuffix(s.ID, "-api")
		resp.Members[i].IsLeader = resp.Leader == resp.Members[i].Name
		resp.Members[i].NumberOfLockedTargets = len(instanceNodes[resp.Members[i].Name])
		resp.Members[i].LockedTargets = instanceNodes[resp.Members[i].Name]
	}
	b, err := json.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	w.Write(b)
}

func (a *App) handleClusteringMembersGet(w http.ResponseWriter, r *http.Request) {
	if a.Config.Clustering == nil {
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	// get leader
	leaderKey := fmt.Sprintf("gnmic/%s/leader", a.Config.ClusterName)
	leader, err := a.locker.List(ctx, leaderKey)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	// get locked targets to instance mapping
	lockedNodesPrefix := fmt.Sprintf("gnmic/%s/targets", a.Config.ClusterName)
	lockedNodes, err := a.locker.List(a.ctx, lockedNodesPrefix)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}

	services, err := a.locker.GetServices(ctx, fmt.Sprintf("%s-gnmic-api", a.Config.ClusterName), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}

	instanceNodes := make(map[string][]string)
	for k, v := range lockedNodes {
		name := strings.TrimPrefix(k, fmt.Sprintf("gnmic/%s/targets/", a.Config.ClusterName))
		if _, ok := instanceNodes[v]; !ok {
			instanceNodes[v] = make([]string, 0)
		}
		instanceNodes[v] = append(instanceNodes[v], name)
	}
	members := make([]clusterMember, len(services))
	for i, s := range services {
		scheme := "http://"
		for _, t := range s.Tags {
			if strings.HasPrefix(t, "protocol=") {
				scheme = fmt.Sprintf("%s://", strings.TrimPrefix(t, "protocol="))
			}
		}
		members[i].APIEndpoint = fmt.Sprintf("%s%s", scheme, s.Address)
		members[i].Name = strings.TrimSuffix(s.ID, "-api")
		members[i].IsLeader = leader[leaderKey] == members[i].Name
		members[i].NumberOfLockedTargets = len(instanceNodes[members[i].Name])
		members[i].LockedTargets = instanceNodes[members[i].Name]
	}
	b, err := json.Marshal(members)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	w.Write(b)
}

func (a *App) handleClusteringLeaderGet(w http.ResponseWriter, r *http.Request) {
	if a.Config.Clustering == nil {
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	// get leader
	leaderKey := fmt.Sprintf("gnmic/%s/leader", a.Config.ClusterName)
	leader, err := a.locker.List(ctx, leaderKey)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	// get locked targets to instance mapping
	lockedNodesPrefix := fmt.Sprintf("gnmic/%s/targets", a.Config.ClusterName)
	lockedNodes, err := a.locker.List(a.ctx, lockedNodesPrefix)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}

	services, err := a.locker.GetServices(ctx, fmt.Sprintf("%s-gnmic-api", a.Config.ClusterName), nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}

	instanceNodes := make(map[string][]string)
	for k, v := range lockedNodes {
		name := strings.TrimPrefix(k, fmt.Sprintf("gnmic/%s/targets/", a.Config.ClusterName))
		if _, ok := instanceNodes[v]; !ok {
			instanceNodes[v] = make([]string, 0)
		}
		instanceNodes[v] = append(instanceNodes[v], name)
	}
	members := make([]clusterMember, 1)
	for _, s := range services {
		if strings.TrimSuffix(s.ID, "-api") != leader[leaderKey] {
			continue
		}
		scheme := "http://"
		for _, t := range s.Tags {
			if strings.HasPrefix(t, "protocol=") {
				scheme = fmt.Sprintf("%s://", strings.TrimPrefix(t, "protocol="))
			}
		}
		// add the leader as a member then break from loop
		members[0].APIEndpoint = fmt.Sprintf("%s%s", scheme, s.Address)
		members[0].Name = strings.TrimSuffix(s.ID, "-api")
		members[0].IsLeader = true
		members[0].NumberOfLockedTargets = len(instanceNodes[members[0].Name])
		members[0].LockedTargets = instanceNodes[members[0].Name]
		break
	}
	b, err := json.Marshal(members)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	w.Write(b)
}

func headersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

func (a *App) loggingMiddleware(next http.Handler) http.Handler {
	next = handlers.LoggingHandler(a.Logger.Writer(), next)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func (a *App) handlerCommonGet(w http.ResponseWriter, r *http.Request, i interface{}) {
	a.configLock.RLock()
	defer a.configLock.RUnlock()
	b, err := json.Marshal(i)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	w.Write(b)
}
