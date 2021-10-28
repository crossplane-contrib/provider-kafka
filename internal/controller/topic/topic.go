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
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka"

	"github.com/pkg/errors"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"

	"github.com/crossplane-contrib/provider-kafka/apis/topic/v1alpha1"
	apisv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/v1alpha1"
)

const (
	errNotTopic     = "managed resource is not a Topic custom resource"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"

	errNewClient = "cannot create new Kafka client"
)

// Setup adds a controller that reconciles Topic managed resources.
func Setup(mgr ctrl.Manager, l logging.Logger, rl workqueue.RateLimiter) error {
	name := managed.ControllerName(v1alpha1.TopicGroupKind)

	o := controller.Options{
		RateLimiter: ratelimiter.NewDefaultManagedRateLimiter(rl),
	}

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1alpha1.TopicGroupVersionKind),
		managed.WithExternalConnecter(&connector{
			kube:         mgr.GetClient(),
			usage:        resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			log:          l,
			newServiceFn: kafka.NewAdminClient}),
		managed.WithLogger(l.WithValues("controller", name)),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o).
		For(&v1alpha1.Topic{}).
		Complete(r)
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube         client.Client
	usage        resource.Tracker
	log          logging.Logger
	newServiceFn func(creds []byte) (*kadm.Client, error)
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
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

	svc, err := c.newServiceFn(data)
	if err != nil {
		return nil, errors.Wrap(err, errNewClient)
	}

	return &external{kafkaClient: svc, log: c.log}, nil
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

	td, err := c.kafkaClient.ListTopics(ctx, meta.GetExternalName(cr))
	if err != nil {
		return managed.ExternalObservation{}, err
	}

	t, ok := td[meta.GetExternalName(cr)]
	if !ok || errors.Is(t.Err, kerr.UnknownTopicOrPartition) {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}
	if t.Err != nil {
		return managed.ExternalObservation{}, errors.Wrapf(t.Err, "cannot get topic")
	}

	cr.Status.AtProvider.ID = t.ID.String()
	cr.Status.SetConditions(v1.Available())

	upToDate := true
	if len(t.Partitions) != cr.Spec.ForProvider.Partitions {
		upToDate = false
	}
	if len(t.Partitions) > 0 && len(t.Partitions[0].Replicas) != cr.Spec.ForProvider.ReplicationFactor {
		upToDate = false
	}

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.Topic)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotTopic)
	}

	resp, err := c.kafkaClient.CreateTopics(ctx, int32(cr.Spec.ForProvider.Partitions), int16(cr.Spec.ForProvider.ReplicationFactor), nil, meta.GetExternalName(cr))
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	t, ok := resp[meta.GetExternalName(cr)]
	if !ok {
		return managed.ExternalCreation{}, errors.New("no create response for topic")
	}
	if t.Err != nil {
		return managed.ExternalCreation{}, errors.Wrap(t.Err, "cannot create")
	}

	cr.Status.AtProvider.ID = t.ID.String()
	return managed.ExternalCreation{}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.Topic)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotTopic)
	}

	td, err := c.kafkaClient.ListTopics(ctx, meta.GetExternalName(cr))
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	p, ok := td[meta.GetExternalName(cr)]
	if !ok || errors.Is(p.Err, kerr.UnknownTopicOrPartition) {
		return managed.ExternalUpdate{}, nil
	}
	if p.Err != nil {
		return managed.ExternalUpdate{}, errors.Wrapf(t.Err, "cannot get topic")
	}
	resp, err := c.kafkaClient.CreatePartitions(ctx, cr.Spec.ForProvider.Partitions - len(p.Partitions), meta.GetExternalName(cr))
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	t, ok := resp[meta.GetExternalName(cr)]
	if !ok {
		return managed.ExternalUpdate{}, errors.New("no create response for topic")
	}
	if t.Err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(t.Err, "cannot create")
	}

	cr.Status.AtProvider.ID = p.ID.String()
	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) error {
	cr, ok := mg.(*v1alpha1.Topic)
	if !ok {
		return errors.New(errNotTopic)
	}

	resp, err := c.kafkaClient.DeleteTopics(ctx, meta.GetExternalName(cr))
	if err != nil {
		return err
	}

	t, ok := resp[meta.GetExternalName(cr)]
	if !ok {
		return errors.New("no delete response for topic")
	}
	if t.Err != nil {
		return errors.Wrap(t.Err, "cannot delete")
	}

	return nil
}
