package cluster_manager

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	apiconst "github.com/openconfig/gnmic/pkg/collector/api/const"
	"github.com/openconfig/gnmic/pkg/collector/managers/testutil"
	"github.com/openconfig/gnmic/pkg/config"
)

type captureAssigner struct {
	mu          sync.Mutex
	assignments map[string]string
}

func (c *captureAssigner) Assign(_ context.Context, targetToMember map[string]*Member) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.assignments == nil {
		c.assignments = make(map[string]string)
	}
	for target, member := range targetToMember {
		if member == nil {
			continue
		}
		c.assignments[target] = member.ID
	}
	return nil
}

func (c *captureAssigner) Unassign(context.Context, *Member, ...string) error { return nil }

func (c *captureAssigner) snapshot() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]string, len(c.assignments))
	for k, v := range c.assignments {
		out[k] = v
	}
	return out
}

func TestRestAssigner_postsAssignments(t *testing.T) {
	var got assignmentConfig
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != apiconst.AssignmentsAPIv1URL {
			t.Fatalf("path = %s", r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)

	st := testutil.NewTestStore(t)
	assigner := NewAssigner(st, nil).(*restAssigner)
	assigner.client = srv.Client()

	addr := strings.TrimPrefix(srv.URL, "http://")
	member := &Member{ID: "node1", Address: addr}
	err := assigner.Assign(context.Background(), map[string]*Member{
		"router1": member,
	})
	if err != nil {
		t.Fatalf("Assign: %v", err)
	}
	if len(got.Assignments) != 1 {
		t.Fatalf("assignments: %#v", got.Assignments)
	}
	if got.Assignments[0].Target != "router1" || got.Assignments[0].Member != "node1" {
		t.Fatalf("assignment: %#v", got.Assignments[0])
	}
}

func TestElection_Campaign_acquiresLeaderLock(t *testing.T) {
	fl := testutil.NewFakeLocker()
	el, err := NewElection(fl, &config.Clustering{
		ClusterName:  "lab",
		InstanceName: "node1",
		Locker:       map[string]interface{}{"session-ttl": "10s"},
	}, slog.Default())
	if err != nil {
		t.Fatalf("NewElection: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	term, err := el.Campaign(ctx)
	if err != nil {
		t.Fatalf("Campaign: %v", err)
	}
	if term < 1 {
		t.Fatalf("term = %d", term)
	}

	held, err := fl.IsLocked(ctx, "gnmic/lab/leader")
	if err != nil {
		t.Fatal(err)
	}
	if !held {
		t.Fatal("expected leader lock held")
	}

	if err := el.Withdraw(); err != nil {
		t.Fatalf("Withdraw: %v", err)
	}
}

func TestClusterManager_reconcileAssignments_assignsTargets(t *testing.T) {
	st := testutil.NewTestStore(t)
	if _, err := st.Config.Set("targets", "router1", map[string]any{"name": "router1"}); err != nil {
		t.Fatalf("seed target: %v", err)
	}
	if _, err := st.Config.Set("targets", "router2", map[string]any{"name": "router2"}); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	ca := &captureAssigner{}
	cm := NewClusterManager(st)
	cm.logger = slog.Default()
	cm.clusteringConfig = &config.Clustering{ClusterName: "lab", InstanceName: "node1"}
	cm.locker = testutil.NewFakeLocker()
	cm.assigner = ca
	cm.members = map[string]*Member{
		"node1": {ID: "node1", Address: "127.0.0.1:1"},
		"node2": {ID: "node2", Address: "127.0.0.1:2"},
	}

	if err := cm.reconcileAssignments(context.Background(), cm.members); err != nil {
		t.Fatalf("reconcileAssignments: %v", err)
	}

	got := ca.snapshot()
	if len(got) != 2 {
		t.Fatalf("assignments = %#v", got)
	}
	for _, target := range []string{"router1", "router2"} {
		if got[target] == "" {
			t.Fatalf("target %q not assigned: %#v", target, got)
		}
	}
}

func TestCalculateMembersQuota_distributesRemainder(t *testing.T) {
	members := map[string]*Member{
		"m1": {ID: "m1"},
		"m2": {ID: "m2"},
		"m3": {ID: "m3"},
	}
	quota := calculateMembersQuota(members, 7)
	sum := int64(0)
	for _, q := range quota {
		sum += q
	}
	if sum != 7 {
		t.Fatalf("quota sum = %d, want 7: %#v", sum, quota)
	}
	if quota["m1"] < quota["m3"] {
		t.Fatalf("sorted remainder not applied: %#v", quota)
	}
}
