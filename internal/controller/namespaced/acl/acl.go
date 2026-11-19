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

package acl

import (
	"context"
	"errors"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane-contrib/provider-kafka/apis/namespaced/acl/v1alpha1"
	apisv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/namespaced/v1alpha1"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka"
	aclhelpers "github.com/crossplane-contrib/provider-kafka/internal/controller/acl"
	"github.com/crossplane-contrib/provider-kafka/internal/controller/common"
)

var _ aclhelpers.Resource = &v1alpha1.AccessControlList{}

// A connector is expected to produce an ExternalClient when its Connect method is called.
type connector struct {
	cache        common.ClientCache
	kube         client.Client
	newServiceFn common.NewClientFn
	usage        *resource.ProviderConfigUsageTracker
}

// Setup adds a controller that reconciles AccessControlList managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	return common.SetupManaged(mgr, o, v1alpha1.AccessControlListGroupVersionKind, &v1alpha1.AccessControlList{}, &v1alpha1.AccessControlListList{},
		&connector{
			cache:        &kafka.ClientCache{},
			kube:         mgr.GetClient(),
			usage:        resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			newServiceFn: kafka.NewAdminClient,
		}, managed.WithInitializers())
}

// SetupGated adds a controller that reconciles AccessControlList managed resources with safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	return common.SetupGated(mgr, o, v1alpha1.AccessControlListGroupVersionKind, Setup)
}

// Connect returns a client for the Kafka cluster in the AccessControlList's provider config.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.AccessControlList)
	if !ok {
		return nil, errors.New(common.ErrNotAccessControlList)
	}
	data, err := common.NamespacedCredentials(ctx, c.kube, c.usage, cr)
	if err != nil {
		return nil, err
	}
	svc, release, err := common.KafkaClient(ctx, c.cache, c.kube, data, c.newServiceFn)
	if err != nil {
		return nil, err
	}
	return aclhelpers.NewExternal(svc, release), nil
}
