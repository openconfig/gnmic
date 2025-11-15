package apiserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/gorilla/mux"
)

type assignmentConfig struct {
	Assignments   []*assignement `json:"assignments"`
	Unassignments []string       `json:"unassignments,omitempty"`
}

func (a *assignmentConfig) validate() error {
	if len(a.Assignments) == 0 && len(a.Unassignments) == 0 {
		return fmt.Errorf("assignments or unassignments is required")
	}
	if len(a.Assignments) > 0 {
		for _, assignment := range a.Assignments {
			if assignment.Target == "" {
				return fmt.Errorf("target is required")
			}
			if assignment.Member == "" {
				return fmt.Errorf("member is required")
			}
		}
	}
	if len(a.Unassignments) > 0 {
		for _, unassignment := range a.Unassignments {
			if unassignment == "" {
				return fmt.Errorf("unassignment is required")
			}
		}
	}
	return nil
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
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	defer r.Body.Close()

	cfg := new(assignmentConfig)
	err = json.Unmarshal(body, &cfg)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	if cfg == nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"invalid assignment config"}})
		return
	}

	err = cfg.validate()
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}

	for _, assignment := range cfg.Assignments {
		_, err := s.configStore.Set("assignments", assignment.Target, assignment)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
	}

	for _, unassignment := range cfg.Unassignments {
		_, _, err = s.configStore.Delete("assignments", unassignment)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
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
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"assignment not found"}})
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
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
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
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
			return
		}
		return
	}
	assignment, ok, err := s.configStore.Get("assignments", id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{"assignment not found"}})
		return
	}
	err = json.NewEncoder(w).Encode(assignment)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(APIErrors{Errors: []string{err.Error()}})
		return
	}
}
