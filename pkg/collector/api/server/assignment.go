package apiserver

import (
	"encoding/json"
	"io"
	"net/http"
	"sort"

	"github.com/gorilla/mux"
)

type assignmentConfig struct {
	Assignments []*assignement `json:"assignments"`
}

type assignement struct {
	Target string `json:"target,omitempty"`
	Member string `json:"member,omitempty"`
	// Epoch  int64  `json:"epoch,omitempty"`
}

// create an assignment by sending a POST request to the assignments endpoint
// sample body:
//
//	{
//		"assignments": [{"target": "target1", "member": "member1", "epoch": 1}, {"target": "target2", "member": "member2", "epoch": 2}]	// list of target names
//	}
//
// sample curl command:
// curl --request POST -H "Content-Type: application/json" \
// -d '{"assignments": [{"target": "target1", "member": "member1", "epoch": 1}, {"target": "target2", "member": "member2", "epoch": 2}]}' \
// http://localhost:8080/api/v1/assignments
func (s *Server) handleAssignmentPost(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()
	cfg := new(assignmentConfig)
	err = json.Unmarshal(body, &cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if cfg == nil {
		http.Error(w, "invalid assignment config", http.StatusBadRequest)
		return
	}
	if len(cfg.Assignments) == 0 {
		http.Error(w, "assignment list is required", http.StatusBadRequest)
		return
	}

	for _, assignment := range cfg.Assignments {
		if assignment.Target == "" {
			http.Error(w, "target name is required", http.StatusBadRequest)
			return
		}
		_, err := s.configStore.Set("assignments", assignment.Target, assignment)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

// delete an assignment by sending a DELETE request to the assignments endpoint
// sample curl command:
// curl --request DELETE http://localhost:8080/api/v1/assignments/target1
func (s *Server) handleAssignmentDelete(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	ok, _, err := s.configStore.Delete("assignments", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "assignment not found", http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)
}

type assignmentResponse struct {
	Member  string   `json:"member"`
	Targets []string `json:"targets"`
}

func (s *Server) handleAssignmentGet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	if id == "" {
		assignments, err := s.configStore.List("assignments")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		ar := &assignmentResponse{
			Targets: make([]string, 0, len(assignments)),
		}
		for k, v := range assignments {
			if ar.Member == "" {
				vm, ok := v.(*assignement)
				if ok {
					ar.Member = vm.Member
				}
			}
			ar.Targets = append(ar.Targets, k)
		}
		sort.Strings(ar.Targets)
		err = json.NewEncoder(w).Encode(ar)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}
	assignment, ok, err := s.configStore.Get("assignments", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "assignment not found", http.StatusNotFound)
		return
	}
	err = json.NewEncoder(w).Encode(assignment)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
