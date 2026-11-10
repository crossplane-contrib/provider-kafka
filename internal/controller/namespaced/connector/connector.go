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

// Package connector reconciles namespaced Kafka Connect Connectors.
package connector

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane-contrib/provider-kafka/apis/namespaced/connector/v1alpha1"
	apisv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/namespaced/v1alpha1"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka/connect"
	"github.com/crossplane-contrib/provider-kafka/internal/controller/common"
	connhelpers "github.com/crossplane-contrib/provider-kafka/internal/controller/connector"
)

// A connector is expected to produce an ExternalClient when its Connect method is called.
type connector struct {
	kube  client.Client
	usage *resource.ProviderConfigUsageTracker
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	client *connect.Client

	// Observe sets observed and status so Update sends only the calls it
	// needs. A config PUT restarts the connector even when nothing changed.
	observed map[string]string
	status   *connect.Status
}

// Setup adds a controller that reconciles Connector managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	return common.SetupManaged(mgr, o, v1alpha1.ConnectorGroupVersionKind, &v1alpha1.Connector{}, &v1alpha1.ConnectorList{},
		&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
		})
}

// SetupGated adds a controller with safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	return common.SetupGated(mgr, o, v1alpha1.ConnectorGroupVersionKind, Setup)
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.Connector)
	if !ok {
		return nil, errors.New(connhelpers.ErrNotConnector)
	}

	if err := c.usage.Track(ctx, cr); err != nil {
		return nil, fmt.Errorf("%s: %w", common.ErrTrackPCUsage, err)
	}

	var cd apisv1alpha1.ProviderCredentials
	ref := cr.GetProviderConfigReference()
	switch ref.Kind {
	case "ProviderConfig":
		pc := &apisv1alpha1.ProviderConfig{}
		if err := c.kube.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: cr.GetNamespace()}, pc); err != nil {
			return nil, fmt.Errorf("%s: %w", common.ErrGetPC, err)
		}
		cd = pc.Spec.Credentials
	case "ClusterProviderConfig":
		cpc := &apisv1alpha1.ClusterProviderConfig{}
		if err := c.kube.Get(ctx, types.NamespacedName{Name: ref.Name}, cpc); err != nil {
			return nil, fmt.Errorf("%s: %w", common.ErrGetCPC, err)
		}
		cd = cpc.Spec.Credentials
	default:
		return nil, fmt.Errorf("unsupported provider config kind: %s", ref.Kind)
	}

	data, err := resource.CommonCredentialExtractor(ctx, cd.Source, c.kube, cd.CommonCredentialSelectors)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", common.ErrGetCreds, err)
	}

	cfg := kafka.Config{}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", connhelpers.ErrParseCreds, err)
	}

	cl, err := connect.New(ctx, cfg.Connect, c.kube)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", connhelpers.ErrNewClient, err)
	}
	return &external{client: cl}, nil
}

func (c *external) Disconnect(_ context.Context) error {
	if c.client != nil {
		c.client.Close()
	}
	return nil
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Connector)
	if !ok {
		return managed.ExternalObservation{}, errors.New(connhelpers.ErrNotConnector)
	}

	name := meta.GetExternalName(cr)
	observed, exists, err := c.client.GetConfig(ctx, name)
	if err != nil {
		return managed.ExternalObservation{}, fmt.Errorf("%s: %w", connhelpers.ErrObserve, err)
	}
	if !exists {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	st, err := c.client.GetStatus(ctx, name)
	if err != nil {
		return managed.ExternalObservation{}, fmt.Errorf("%s: %w", connhelpers.ErrObserve, err)
	}
	c.observed, c.status = observed, st

	p := cr.Spec.ForProvider
	desired := connhelpers.Desired(p, name)
	if st == nil {
		cr.Status.SetConditions(xpv2.Creating())
		return managed.ExternalObservation{ResourceExists: true, ResourceUpToDate: connhelpers.UpToDate(desired, observed)}, nil
	}

	cr.Status.AtProvider = connhelpers.Observation(st)
	cr.Status.SetConditions(connhelpers.Condition(p.State, st))

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: connhelpers.UpToDate(desired, observed) && connhelpers.StateUpToDate(p.State, st),
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Connector)
	if !ok {
		return managed.ExternalCreation{}, errors.New(connhelpers.ErrNotConnector)
	}
	cr.Status.SetConditions(xpv2.Creating())

	// NOTE: a paused or stopped connector runs briefly between these two
	// calls. POST /connectors with initial_state avoids it but needs Kafka 3.7+.
	name := meta.GetExternalName(cr)
	p := cr.Spec.ForProvider
	if err := c.client.PutConfig(ctx, name, connhelpers.Desired(p, name)); err != nil {
		return managed.ExternalCreation{}, fmt.Errorf("%s: %w", connhelpers.ErrPutConfig, err)
	}
	if action := connhelpers.StateAction(p.State); action != connect.ActionResume {
		if err := c.client.SetState(ctx, name, action); err != nil {
			return managed.ExternalCreation{}, fmt.Errorf("%s: %w", connhelpers.ErrSetState, err)
		}
	}
	return managed.ExternalCreation{}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Connector)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(connhelpers.ErrNotConnector)
	}

	name := meta.GetExternalName(cr)
	p := cr.Spec.ForProvider
	if desired := connhelpers.Desired(p, name); !connhelpers.UpToDate(desired, c.observed) {
		if err := c.client.PutConfig(ctx, name, desired); err != nil {
			return managed.ExternalUpdate{}, fmt.Errorf("%s: %w", connhelpers.ErrPutConfig, err)
		}
	}
	if c.status != nil && !connhelpers.StateUpToDate(p.State, c.status) {
		if err := c.client.SetState(ctx, name, connhelpers.StateAction(p.State)); err != nil {
			return managed.ExternalUpdate{}, fmt.Errorf("%s: %w", connhelpers.ErrSetState, err)
		}
	}
	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.Connector)
	if !ok {
		return managed.ExternalDelete{}, errors.New(connhelpers.ErrNotConnector)
	}
	cr.Status.SetConditions(xpv2.Deleting())

	if err := c.client.Delete(ctx, meta.GetExternalName(cr)); err != nil {
		return managed.ExternalDelete{}, fmt.Errorf("%s: %w", connhelpers.ErrDelete, err)
	}
	return managed.ExternalDelete{}, nil
}
