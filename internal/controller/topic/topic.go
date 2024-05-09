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

package topic

import (
	"context"
	"strings"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/controller"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/pkg/errors"
	"github.com/twmb/franz-go/pkg/kadm"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane-contrib/provider-kafka/apis/topic/v1alpha1"
	apisv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/v1alpha1"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka/topic"
)

const (
	errNotTopic     = "managed resource is not a Topic custom resource"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"
	errGetTopic     = "cannot get topic spec from topic client"

	errNewClient = "cannot create new Kafka client"
)

// Setup adds a controller that reconciles Topic managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.TopicGroupKind)

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.TopicGroupVersionKind),
		managed.WithExternalConnectDisconnecter(&connectDisconnector{
			kube:         mgr.GetClient(),
			usage:        resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			newServiceFn: kafka.NewAdminClient}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		For(&v1alpha1.Topic{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// A connectDisconnector is expected to produce an ExternalClient when its Connect method
// is called and close it when its Disconnect method is called.
type connectDisconnector struct {
	kube         client.Client
	usage        resource.Tracker
	log          logging.Logger
	newServiceFn func(ctx context.Context, creds []byte, kube client.Client) (*kadm.Client, error)
	cachedClient *kadm.Client
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connectDisconnector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.Topic)
	if !ok {
		return nil, errors.New(errNotTopic)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	pc := &apisv1alpha1.ProviderConfig{}
	if err := c.kube.Get(ctx, types.NamespacedName{Name: cr.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, errors.Wrap(err, errGetPC)
	}

	cd := pc.Spec.Credentials
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

func (c *connectDisconnector) Disconnect(ctx context.Context) error {
	if c.cachedClient != nil {
		c.cachedClient.Close()
	}
	c.cachedClient = nil
	return nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	kafkaClient *kadm.Client
	log         logging.Logger
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.Topic)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotTopic)
	}

	tpc, err := topic.Get(ctx, c.kafkaClient, meta.GetExternalName(cr))
	if err != nil { // Discern whether the topic doesn't exist or something went wrong
		if strings.HasPrefix(err.Error(), topic.ErrTopicDoesNotExist) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, errors.Wrapf(err, errGetTopic)
	}

	cr.Status.AtProvider.ID = tpc.ID
	cr.Status.SetConditions(v1.Available())

	lateInitialized := topic.LateInitializeSpec(&cr.Spec.ForProvider, tpc)

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        topic.IsUpToDate(&cr.Spec.ForProvider, tpc),
		ResourceLateInitialized: lateInitialized,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Topic)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotTopic)
	}
	if len(cr.Spec.ForProvider.TopicName) != 0 {
		meta.SetExternalName(cr, cr.Spec.ForProvider.TopicName)
	} else {
		meta.SetExternalName(cr, meta.GetExternalName(cr))
	}
	return managed.ExternalCreation{}, topic.Create(ctx, c.kafkaClient, topic.Generate(meta.GetExternalName(cr), &cr.Spec.ForProvider))
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Topic)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotTopic)
	}

	return managed.ExternalUpdate{}, topic.Update(ctx, c.kafkaClient, topic.Generate(meta.GetExternalName(cr), &cr.Spec.ForProvider))
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Topic)
	if !ok {
		return errors.New(errNotTopic)
	}
	return topic.Delete(ctx, c.kafkaClient, meta.GetExternalName(cr))
}
