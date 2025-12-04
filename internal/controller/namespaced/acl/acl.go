/*
Copyright 2020 The Crossplane Authors.

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
	"strings"

	"github.com/crossplane-contrib/provider-kafka/v2/internal/clients/kafka"
	"github.com/crossplane-contrib/provider-kafka/v2/internal/clients/kafka/acl"
	"github.com/twmb/franz-go/pkg/kadm"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/crossplane/crossplane-runtime/v2/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"

	"github.com/crossplane-contrib/provider-kafka/v2/apis/namespaced/acl/v1alpha1"
	apisv1alpha1 "github.com/crossplane-contrib/provider-kafka/v2/apis/namespaced/v1alpha1"
)

const (
	errNotAccessControlList = "managed resource is not a AccessControlList custom resource"
	errTrackPCUsage         = "cannot track ProviderConfig usage"
	errGetPC                = "cannot get ProviderConfig"
	errGetCPC               = "cannot get ClusterProviderConfig"
	errGetCreds             = "cannot get credentials"
	errListACL              = "cannot List ACLs"
	errNewClient            = "cannot create new Service"
	errUpdateNotSupported   = "updates are not supported"
)

// Setup adds a controller that reconciles AccessControlList managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.AccessControlListGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithExternalConnector(&connector{
			kube:         mgr.GetClient(),
			usage:        resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			newServiceFn: kafka.NewAdminClient,
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))),
		managed.WithInitializers(),
	}

	if o.Features.Enabled(feature.EnableBetaManagementPolicies) {
		opts = append(opts, managed.WithManagementPolicies())
	}

	if o.Features.Enabled(feature.EnableAlphaChangeLogs) {
		opts = append(opts, managed.WithChangeLogger(o.ChangeLogOptions.ChangeLogger))
	}

	if o.MetricOptions != nil {
		opts = append(opts, managed.WithMetricRecorder(o.MetricOptions.MRMetrics))
	}

	if o.MetricOptions != nil && o.MetricOptions.MRStateMetrics != nil {
		stateMetricsRecorder := statemetrics.NewMRStateRecorder(
			mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &v1alpha1.AccessControlListList{}, o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return errors.Wrap(err, "cannot register MR state metrics recorder for kind v1alpha1.AccessControlListList")
		}
	}

	r := managed.NewReconciler(mgr, resource.ManagedKind(v1alpha1.AccessControlListGroupVersionKind), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.AccessControlList{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// SetupGated adds a controller that reconciles MyType managed resources with safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(errors.Wrap(err, "cannot setup AccessControlList controller"))
		}
	}, v1alpha1.AccessControlListGroupVersionKind)
	return nil
}

// A connector is expected to produce an ExternalClient when its Connect method is called.
type connector struct {
	kube         client.Client
	usage        *resource.ProviderConfigUsageTracker
	log          logging.Logger
	newServiceFn func(ctx context.Context, creds []byte, kube client.Client) (*kadm.Client, error)
	cachedClient *kadm.Client
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.AccessControlList)
	if !ok {
		return nil, errors.New(errNotAccessControlList)
	}

	if err := c.usage.Track(ctx, cr); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	var cd apisv1alpha1.ProviderCredentials

	// Switch to ModernManaged resource to get ProviderConfigRef
	m := mg.(resource.ModernManaged)
	ref := m.GetProviderConfigReference()

	switch ref.Kind {
	case "ProviderConfig":
		pc := &apisv1alpha1.ProviderConfig{}
		if err := c.kube.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: m.GetNamespace()}, pc); err != nil {
			return nil, errors.Wrap(err, errGetPC)
		}
		cd = pc.Spec.Credentials
	case "ClusterProviderConfig":
		cpc := &apisv1alpha1.ClusterProviderConfig{}
		if err := c.kube.Get(ctx, types.NamespacedName{Name: ref.Name}, cpc); err != nil {
			return nil, errors.Wrap(err, errGetCPC)
		}
		cd = cpc.Spec.Credentials
	default:
		return nil, errors.Errorf("unsupported provider config kind: %s", ref.Kind)
	}

	data, err := resource.CommonCredentialExtractor(ctx, cd.Source, c.kube, cd.CommonCredentialSelectors)
	if err != nil {
		return nil, errors.Wrap(err, errGetCreds)
	}

	svc, err := c.newServiceFn(ctx, data, c.kube)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}
	c.cachedClient = svc

	return &external{kafkaClient: svc, log: c.log}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	kafkaClient *kadm.Client
	log         logging.Logger
}

func (c *external) Disconnect(ctx context.Context) error {
	if c.kafkaClient != nil {
		c.kafkaClient.Close()
	}
	c.kafkaClient = nil
	return nil
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.AccessControlList)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotAccessControlList)
	}

	// Check if the external name is set, to determine if ACL has been created or not
	ext := meta.GetExternalName(cr)
	if ext == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	extname, _ := acl.ConvertFromJSON(meta.GetExternalName(cr))
	compare := acl.CompareAcls(*extname, *acl.Generate(&cr.Spec.ForProvider))
	diff := acl.Diff(*extname, *acl.Generate(&cr.Spec.ForProvider))

	if !compare {
		err := strings.Join(diff, " ")
		return managed.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: false,
		}, errors.New(err)
	}

	ae, err := acl.List(ctx, c.kafkaClient, extname)

	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, errListACL)
	}

	if ae == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.Status.SetConditions(v1.Available())

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        true,
		ResourceLateInitialized: false,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {

	cr, ok := mg.(*v1alpha1.AccessControlList)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotAccessControlList)
	}

	generated := acl.Generate(&cr.Spec.ForProvider)
	extname, err := acl.ConvertToJSON(generated)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "could not convert external name to JSON")
	}
	if meta.GetExternalName(cr) == "" {
		meta.SetExternalName(cr, extname)
		return managed.ExternalCreation{}, acl.Create(ctx, c.kafkaClient, generated)
	}

	return managed.ExternalCreation{}, acl.Create(ctx, c.kafkaClient, generated)
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {

	return managed.ExternalUpdate{}, errors.New(errUpdateNotSupported)
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {

	cr, ok := mg.(*v1alpha1.AccessControlList)
	cr.Status.SetConditions(v1.Deleting())
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotAccessControlList)
	}

	return managed.ExternalDelete{}, acl.Delete(ctx, c.kafkaClient, acl.Generate(&cr.Spec.ForProvider))
}
