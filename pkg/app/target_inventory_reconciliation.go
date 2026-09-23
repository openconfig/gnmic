// SPDX-License-Identifier: Apache-2.0

package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/openconfig/gnmic/pkg/lockers"
	"github.com/openconfig/gnmic/pkg/logging"
)

const targetDeletionWorkers = 16

type targetDeletion struct {
	name    string
	service *lockers.Service
}

func (a *App) apiServicesSnapshot() map[string]*lockers.Service {
	a.configLock.RLock()
	defer a.configLock.RUnlock()
	services := make(map[string]*lockers.Service, len(a.apiServices))
	for name, service := range a.apiServices {
		services[name] = service
	}
	return services
}

func (a *App) reconcileDeletedTargets(ctx context.Context) {
	a.configLock.RLock()
	ready := a.loaderSnapshotReady
	desired := make(map[string]struct{}, len(a.Config.Targets))
	for name := range a.Config.Targets {
		desired[name] = struct{}{}
	}
	a.configLock.RUnlock()
	if !ready {
		return
	}
	if err := a.createAPIClient(); err != nil {
		logging.LogErrUnlessCanceled(a.Logger, err, "failed to create cluster API client")
		return
	}

	services := a.apiServicesSnapshot()
	deletions := make(chan targetDeletion)
	var workers sync.WaitGroup
	for range targetDeletionWorkers {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for deletion := range deletions {
				if a.targetConfigExists(deletion.name) {
					continue
				}
				if err := a.deleteTargetConfigFromService(ctx, deletion.name, deletion.service); err != nil {
					logging.LogErrUnlessCanceled(a.Logger, err, "failed to reconcile deleted target", "target", deletion.name, "service", deletion.service.ID)
				}
			}
		}()
	}
	var readers sync.WaitGroup
	for serviceID, service := range services {
		readers.Add(1)
		go func() {
			defer readers.Done()
			names, err := a.targetNamesOnService(ctx, service)
			if err != nil {
				logging.LogErrUnlessCanceled(a.Logger, err, "failed to read service targets", "service", serviceID)
				return
			}
			for name := range names {
				if _, exists := desired[name]; exists {
					continue
				}
				select {
				case deletions <- targetDeletion{name: name, service: service}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	readers.Wait()
	close(deletions)
	workers.Wait()
}

func (a *App) targetNamesOnService(ctx context.Context, service *lockers.Service) (map[string]struct{}, error) {
	names := make(map[string]struct{})
	for _, resource := range []string{"config/targets", "targets"} {
		endpoint := fmt.Sprintf("%s://%s/api/v1/%s", a.getServiceScheme(service), service.Address, resource)
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, err
		}
		response, err := a.clusteringClient.Do(request)
		if err != nil {
			return nil, err
		}
		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			return nil, fmt.Errorf("read %s on %s: HTTP %d", resource, service.ID, response.StatusCode)
		}
		var targets map[string]json.RawMessage
		err = json.NewDecoder(response.Body).Decode(&targets)
		response.Body.Close()
		if err != nil {
			return nil, err
		}
		for name := range targets {
			names[name] = struct{}{}
		}
	}
	return names, nil
}

func (a *App) deleteTargetConfigFromService(ctx context.Context, name string, service *lockers.Service) error {
	if err := a.createAPIClient(); err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s://%s/api/v1/config/targets/%s", a.getServiceScheme(service), service.Address, name)
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	response, err := a.clusteringClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusNotFound && (response.StatusCode < 200 || response.StatusCode >= 300) {
		return fmt.Errorf("delete target %q on %s: HTTP %d", name, service.ID, response.StatusCode)
	}
	return nil
}
