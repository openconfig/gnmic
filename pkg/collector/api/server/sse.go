package apiserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	collstore "github.com/openconfig/gnmic/pkg/collector/store"
	"github.com/zestor-dev/zestor/store"
)

// validSSEKinds are the store kinds that can be streamed via SSE.
var validSSEKinds = map[string]struct{}{
	collstore.KindTargets:             {},
	collstore.KindOutputs:             {},
	collstore.KindInputs:              {},
	collstore.KindSubscriptions:       {},
	collstore.KindProcessors:          {},
	collstore.KindAssignments:         {},
	collstore.KindTunnelTargetMatches: {},
}

// sseEvent is the JSON payload sent for each SSE event.
type sseEvent struct {
	Timestamp time.Time `json:"timestamp"`  // when the event was emitted
	Store     string    `json:"store"`      // "config" or "state"
	Kind      string    `json:"kind"`       // targets, outputs, inputs, subscriptions
	Name      string    `json:"name"`       // entry name / key
	EventType string    `json:"event-type"` // create, update, delete
	Object    any       `json:"object"`     // the entry value
}

// handleSSE streams store changes for a given kind as Server-Sent Events.
//
// GET /api/v1/sse/{kind}?store=config|state|all
//
// Query parameter "store" selects which store(s) to watch:
//   - "config" — config store only
//   - "state"  — state store only
//   - "all"    — both (default)
//
// The client receives an event stream where each event is a JSON-encoded
// sseEvent. An initial snapshot of existing entries is sent first (as
// "create" events), followed by live updates.
func (s *Server) handleSSE(w http.ResponseWriter, r *http.Request) {
	kind := mux.Vars(r)["kind"]
	if _, ok := validSSEKinds[kind]; !ok {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{
			Errors: []string{fmt.Sprintf("invalid kind %q; expected one of: targets, outputs, inputs, subscriptions", kind)},
		})
		return
	}

	storeFilter := r.URL.Query().Get("store")
	if storeFilter == "" {
		storeFilter = "all"
	}
	if storeFilter != "config" && storeFilter != "state" && storeFilter != "all" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{
			Errors: []string{fmt.Sprintf("invalid store %q; expected one of: config, state, all", storeFilter)},
		})
		return
	}

	// ensure the ResponseWriter supports flushing.
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// set up watches.
	var configCh <-chan *store.Event[any]
	var configCancel func()
	var stateCh <-chan *store.Event[any]
	var stateCancel func()

	if storeFilter == "config" || storeFilter == "all" {
		var err error
		configCh, configCancel, err = s.store.Config.Watch(kind, store.WithInitialReplay[any]())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
		defer configCancel()
	}
	if storeFilter == "state" || storeFilter == "all" {
		var err error
		stateCh, stateCancel, err = s.store.State.Watch(kind, store.WithInitialReplay[any]())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
		defer stateCancel()
	}

	// set SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
	flusher.Flush()

	ctx := r.Context()
	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-keepalive.C:
			// SSE comment line as keepalive to detect broken connections.
			fmt.Fprintf(w, ": keepalive\n\n")
			flusher.Flush()
		case ev, ok := <-configCh:
			if !ok {
				configCh = nil
				continue
			}
			s.sendSSEEvent(w, flusher, "config", ev)
		case ev, ok := <-stateCh:
			if !ok {
				stateCh = nil
				continue
			}
			s.sendSSEEvent(w, flusher, "state", ev)
		}
	}
}

func (s *Server) sendSSEEvent(w http.ResponseWriter, flusher http.Flusher, storeName string, ev *store.Event[any]) {
	data, err := json.Marshal(sseEvent{
		Timestamp: time.Now(),
		Store:     storeName,
		Kind:      ev.Kind,
		Name:      ev.Name,
		EventType: string(ev.EventType),
		Object:    ev.Object,
	})
	if err != nil {
		s.logger.Error("failed to marshal SSE event", "error", err)
		return
	}
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.EventType, data)
	flusher.Flush()
}
