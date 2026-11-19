/*
Copyright 2026 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package connector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/v1alpha1"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka/connect"
)

// Resource is a Connector managed resource of either scope.
type Resource interface {
	resource.Managed
	GetConnectorParameters() commonv1alpha1.ConnectorParameters
	SetConnectorObservation(o commonv1alpha1.ConnectorObservation)
}

// External observes, creates, updates and deletes a connector through the
// Kafka Connect REST API. Both Connector controllers use it.
type External struct {
	client  *connect.Client
	release func()

	// Observe sets observed and status so Update sends only the calls it
	// needs. A config PUT restarts the connector even when nothing changed.
	observed map[string]string
	status   *connect.Status
}

// Connect parses the provider credentials creds and returns an External
// backed by the Kafka Connect client cached for them.
func Connect(ctx context.Context, cache *kafka.Cache[*connect.Client], kube client.Client, creds []byte) (managed.ExternalClient, error) {
	cfg := kafka.Config{}
	if err := json.Unmarshal(creds, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", ErrParseCreds, err)
	}

	key, err := kafka.CacheKey(ctx, creds, kube)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrNewClient, err)
	}

	// Reuse the client so its idle connections survive between reconciles.
	cl, release, err := cache.GetOrCreate(key, func() (*connect.Client, error) {
		return connect.New(ctx, cfg.Connect, kube)
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrNewClient, err)
	}
	return &External{client: cl, release: release}, nil
}

// Disconnect releases the cached client.
func (e *External) Disconnect(_ context.Context) error {
	if e.release != nil {
		e.release()
	}
	return nil
}

// Observe reports whether the connector exists and matches the spec.
func (e *External) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(Resource)
	if !ok {
		return managed.ExternalObservation{}, errors.New(ErrNotConnector)
	}

	name := meta.GetExternalName(cr)
	observed, exists, err := e.client.GetConfig(ctx, name)
	if err != nil {
		return managed.ExternalObservation{}, fmt.Errorf("%s: %w", ErrObserve, err)
	}
	if !exists {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	st, err := e.client.GetStatus(ctx, name)
	if err != nil {
		return managed.ExternalObservation{}, fmt.Errorf("%s: %w", ErrObserve, err)
	}
	e.observed, e.status = observed, st

	p := cr.GetConnectorParameters()
	desired := Desired(p, name)
	if st == nil {
		cr.SetConditions(xpv2.Creating())
		return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: UpToDate(desired, observed)}, nil
	}

	cr.SetConnectorObservation(Observation(st))
	cr.SetConditions(Condition(p.State, st))

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: UpToDate(desired, observed) && StateUpToDate(p.State, st),
	}, nil
}

// Create creates the connector and moves it to the desired state.
func (e *External) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(Resource)
	if !ok {
		return managed.ExternalCreation{}, errors.New(ErrNotConnector)
	}
	cr.SetConditions(xpv2.Creating())

	// NOTE: a paused or stopped connector runs briefly between these two
	// calls. POST /connectors with initial_state avoids it but needs Kafka 3.7+.
	name := meta.GetExternalName(cr)
	p := cr.GetConnectorParameters()
	if err := e.client.PutConfig(ctx, name, Desired(p, name)); err != nil {
		return managed.ExternalCreation{}, fmt.Errorf("%s: %w", ErrPutConfig, err)
	}
	if action := StateAction(p.State); action != connect.ActionResume {
		if err := e.client.SetState(ctx, name, action); err != nil {
			return managed.ExternalCreation{}, fmt.Errorf("%s: %w", ErrSetState, err)
		}
	}
	return managed.ExternalCreation{}, nil
}

// Update sends only the config and state changes Observe found.
func (e *External) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(Resource)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(ErrNotConnector)
	}

	name := meta.GetExternalName(cr)
	p := cr.GetConnectorParameters()
	if desired := Desired(p, name); !UpToDate(desired, e.observed) {
		if err := e.client.PutConfig(ctx, name, desired); err != nil {
			return managed.ExternalUpdate{}, fmt.Errorf("%s: %w", ErrPutConfig, err)
		}
	}
	if e.status != nil && !StateUpToDate(p.State, e.status) {
		if err := e.client.SetState(ctx, name, StateAction(p.State)); err != nil {
			return managed.ExternalUpdate{}, fmt.Errorf("%s: %w", ErrSetState, err)
		}
	}
	return managed.ExternalUpdate{}, nil
}

// Delete deletes the connector.
func (e *External) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(Resource)
	if !ok {
		return managed.ExternalDelete{}, errors.New(ErrNotConnector)
	}
	cr.SetConditions(xpv2.Deleting())

	if err := e.client.Delete(ctx, meta.GetExternalName(cr)); err != nil {
		return managed.ExternalDelete{}, fmt.Errorf("%s: %w", ErrDelete, err)
	}
	return managed.ExternalDelete{}, nil
}
