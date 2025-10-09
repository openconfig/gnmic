package cluster_manager

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	apiconst "github.com/openconfig/gnmic/pkg/collector/api/const"
	"github.com/openconfig/gnmic/pkg/config/store"
)

type Assignment struct {
	Target string `json:"target,omitempty"`
	Member string `json:"member,omitempty"`
	// Epoch  int64  `json:"epoch,omitempty"`
}

type assignmentConfig struct {
	Assignments []*Assignment `json:"assignments"`
}

type Assigner interface {
	Assign(ctx context.Context, targetToMember map[string]*Member) error
	Unassign(ctx context.Context, target string, member *Member) error
}

const (
	httpScheme    = "http"
	httpsScheme   = "https"
	protocolLabel = "__protocol"
)

type restAssigner struct {
	client *http.Client
	store  store.Store[any]
	logger *slog.Logger
}

func NewAssigner(store store.Store[any]) Assigner {
	return &restAssigner{
		store:  store,
		logger: slog.With("component", "assignment-pusher"),
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (p *restAssigner) Assign(ctx context.Context, targetToMember map[string]*Member) error {
	// epoch := time.Now().Unix()
	// TODO: group by address
	for targetName, member := range targetToMember {
		if member == nil || member.Address == "" {
			p.logger.Warn("member is nil or address is empty", "target", targetName, "member", member)
			continue
		}
		scheme := GetAPIScheme(member)
		address := scheme + "://" + member.Address + apiconst.AssignmentsAPIv1URL
		err := p.assignOne(ctx, address, []*Assignment{
			{
				Target: targetName,
				Member: member.ID,
				// Epoch: epoch
			},
		})
		if err != nil {
			return err
		}

	}

	return nil
}

func (p *restAssigner) assignOne(ctx context.Context, address string, assignmentSet []*Assignment) error {
	b, err := json.Marshal(&assignmentConfig{Assignments: assignmentSet})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, address, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		body, _ := io.ReadAll(resp.Body)
		p.logger.Error("failed to assign", "address", address, "assignmentSet", assignmentSet, "status", resp.Status, "body", string(body))
		return fmt.Errorf("assign: %s", resp.Status)
	}
	return nil
}

func (p *restAssigner) Unassign(ctx context.Context, target string, member *Member) error {
	if member == nil || member.Address == "" {
		return fmt.Errorf("member is nil or address is empty")
	}
	scheme := GetAPIScheme(member)
	address := scheme + "://" + member.Address + apiconst.AssignmentsAPIv1URL + "/" + target
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, address, nil)
	if err != nil {
		return err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("unassign: %s", resp.Status)
	}
	return nil
}
