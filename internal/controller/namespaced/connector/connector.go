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
	"errors"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
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
	cache *kafka.Cache[*connect.Client]
}

var _ connhelpers.Resource = &v1alpha1.Connector{}

// Setup adds a controller that reconciles Connector managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	return common.SetupManaged(mgr, o, v1alpha1.ConnectorGroupVersionKind, &v1alpha1.Connector{}, &v1alpha1.ConnectorList{},
		&connector{
			kube:  mgr.GetClient(),
			usage: resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			cache: &kafka.Cache[*connect.Client]{},
		}, managed.WithTimeout(connhelpers.ReconcileTimeout))
}

// SetupGated adds a controller with safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	return common.SetupGated(mgr, o, v1alpha1.ConnectorGroupVersionKind, Setup)
}

// Connect returns a client for the Kafka Connect cluster in the Connector's provider config.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.Connector)
	if !ok {
		return nil, errors.New(connhelpers.ErrNotConnector)
	}
	data, err := common.NamespacedCredentials(ctx, c.kube, c.usage, cr)
	if err != nil {
		return nil, err
	}
	return connhelpers.Connect(ctx, c.cache, c.kube, data)
}
