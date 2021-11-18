package app

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/karimra/gnmic/types"
	"github.com/karimra/gnmic/utils"
	"github.com/prometheus/client_golang/prometheus"
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
		a.reg.MustRegister(prometheus.NewGoCollector())
		a.reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))
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
	if _, ok := a.Config.Targets[id]; !ok {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{fmt.Sprintf("target %q not found", id)}})
		return
	}
	go a.TargetSubscribeStream(a.ctx, id)
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
