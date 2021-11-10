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
	"github.com/crossplane-contrib/provider-kafka/apis/topic/v1alpha1"
	apisv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/v1alpha1"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka"
	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/crossplane/crossplane-runtime/pkg/event"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
	"github.com/crossplane/crossplane-runtime/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/pkg/resource"
	"github.com/pkg/errors"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kerr"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

const (
	errNotTopic     = "managed resource is not a Topic custom resource"
	errTrackPCUsage = "cannot track ProviderConfig usage"
	errGetPC        = "cannot get ProviderConfig"
	errGetCreds     = "cannot get credentials"

	errNewClient = "cannot create new Kafka client"

)

const (
	CleanupPolicy = "cleanup.policy"
	CompressionType = "compression.type"
	DeleteRetentionMs = "delete.retention.ms"
	FileDeleteDelayMs = "file.delete.delay.ms"
	FlushMessages = "flush.messages"
	FlushMs = "flush.ms"
	FollowerReplicationThrottledReplicas = "follower.replication.throttled.replicas"
	IndexIntervalBytes = "index.interval.bytes"
	LeaderReplicationThrottledReplicas = "leader.replication.throttled.replicas"
	LocalRetentionBytes = "local.retention.bytes"
	LocalRetentionMs = "local.retention.ms"
	MaxCompactionLagMs = "max.compaction.lag.ms"
	MaxMessageBytes = "max.message.bytes"
	MessageTimestampDifferenceMaxMs = "message.timestamp.difference.max.ms"
	MessageTimestampType = "message.timestamp.type"
	MinCleanableDirtyRatio = "min.cleanable.dirty.ratio"
	MinCompactionLagMs = "min.compaction.lag.ms"
	MinInsyncReplicas = "min.insync.replicas"
	Preallocate = "preallocate"
	RemoteStorageEnable = "remote.storage.enable"
	RetentionBytes = "retention.bytes"
	RetentionMs = "retention.ms"
	SegmentBytes = "segment.bytes"
	SegmentIndexBytes = "segment.index.bytes"
	SegmentJitterMs = "segment.jitter.ms"
	SegmentMs = "segment.ms"
	UncleanLeaderElectionEnable = "unclean.leader.eleection.enable"
	MessageDownconversionEnable = "message.downconversion.enable"
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
	if cr.Spec.ForProvider.Partitions != len(t.Partitions) {
		upToDate = false
	}
	if len(t.Partitions) > 0 && len(t.Partitions[0].Replicas) != cr.Spec.ForProvider.ReplicationFactor {
		upToDate = false
	}

	//configs := cr.Spec.ForProvider
	//
	//if configs.CleanupPolicy != nil{
	//policy := configs.CleanupPolicy      //[]metav1.List
	//fmt.Println(policy)
	//}
	//if configs.CompressionType != nil {
	//	compression := *configs.CompressionType //*string
	//}
	//retention := *configs.DeleteRetentionMs                    //*int64
	//fmt.Println(retention)
	//if configs.MessageDownconversionEnable != nil{
	//msgdownconvert:= *configs.MessageDownconversionEnable          //*bool
	//fmt.Println(msgdownconvert)
	//}
	//filedelete := configs.FileDeleteDelayMs                    //*int64
	//flushmess := configs.FlushMessages                        //*int64
	//flushms:= configs.FlushMs                              //*int64
	//followerreps:= configs.FollowerReplicationThrottledReplicas //[]metav1.List
	//indexint:= configs.IndexIntervalBytes                   //*int
	//leaderrepthrottled:= configs.LeaderReplicationThrottledReplicas   //[]metav1.List
	//localretbytes:= configs.LocalRetentionBytes                  //*int64
	//localretms:= configs.LocalRetentionMs                     //*int64
	//maxcompact:= configs.MaxCompactionLagMs                   //*int64
	//maxmessage:= configs.MaxMessageBytes                      //*int
	//msgtimediff:= configs.MessageTimestampDifferenceMaxMs      //*int64
	//msgtimetype:= configs.MessageTimestampType                 //*string
	//minclean:= configs.MinCleanableDirtyRatio               //*int32
	//mincompact:= configs.MinCompactionLagMs                   //*int64
	//mininsync:= configs.MinInsyncReplicas                    //*int
	//preallocate:= configs.Preallocate                          //*bool
	//remotestorage:= configs.RemoteStorageEnable                  //*bool
	//retbytes:= configs.RetentionBytes                       //*int64
	//retms:= configs.RetentionMs                          //*int64
	//segbytes:= configs.SegmentBytes                         //*int
	//segindxbytes:= configs.SegmentIndexBytes                    //*int
	//segjitter:= configs.SegmentJitterMs                      //*int64
	//segms:= configs.SegmentMs                            //*int64
	//uncleanelect:= configs.UncleanLeaderElectionEnable          //*bool

		//config := kadm.AlterConfig{
	//	Op:    kadm.SetConfig, // Op is the incremental alter operation to perform.
	//	Name:  name,           // Name is the name of the config to alter.
	//	Value: value,          // Value is the value to use when altering, if any.
	//}

	//x := cr.Spec.ForProvider
	//s := reflect.ValueOf(&x).Elem()
	//typeOfT := s.Type()
	//for i := 0; i < s.NumField(); i++ {
	//	key := typeOfT.Field(i).Name
	//	value := s.Field(i).Interface()
	//	if value != nil{
	//		fmt.Println(key)
	//		fmt.Println(value)
	//	}
	//}

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
		return managed.ExternalUpdate{}, errors.Wrapf(p.Err, "cannot get topic")
	}

	comptype := kadm.AlterConfig{
		Op:    kadm.SetConfig,       				// Op is the incremental alter operation to perform.
		Name:  "compression.type",      			// Name is the name of the config to alter.
		Value: cr.Spec.ForProvider.CompressionType, // Value is the value to use when altering, if any.
	}

	//delms := strconv.FormatInt(*cr.Spec.ForProvider.DeleteRetentionMs, 10)
	//
	//deletems := kadm.AlterConfig{
	//	Op:    kadm.SetConfig,       				// Op is the incremental alter operation to perform.
	//	Name:  "delete.retention.ms",      			// Name is the name of the config to alter.
	//	Value: &delms, // Value is the value to use when altering, if any.
	//}

	//boolparse := strconv.FormatBool(*cr.Spec.ForProvider.Preallocate)
	//bools := kadm.AlterConfig{
	//	Op:    kadm.SetConfig,       				// Op is the incremental alter operation to perform.
	//	Name:  "preallocate",      			// Name is the name of the config to alter.
	//	Value: &boolparse, // Value is the value to use when altering, if any.
	//}

	r, err := c.kafkaClient.AlterTopicConfigs(ctx, []kadm.AlterConfig{comptype}, meta.GetExternalName(cr))
	if err != nil {
		return managed.ExternalUpdate{}, err
	}
	if r[0].Err != nil {
		return managed.ExternalUpdate{}, r[0].Err
	}

	l := cr.Spec.ForProvider.Partitions - len(p.Partitions)

	if l < 1 {
		return managed.ExternalUpdate{}, errors.Errorf("cannot decrease partition count from %d to %d", len(p.Partitions), cr.Spec.ForProvider.Partitions)
	} else {
		resp, err := c.kafkaClient.UpdatePartitions(ctx, cr.Spec.ForProvider.Partitions, meta.GetExternalName(cr))

		if err != nil {
			return managed.ExternalUpdate{}, err
		}

		t, ok := resp[meta.GetExternalName(cr)]
		if !ok {
			return managed.ExternalUpdate{}, errors.New("no create partitions response for topic")
		}
		if t.Err != nil {
			return managed.ExternalUpdate{}, errors.Wrap(t.Err, "cannot create partitions")
		}

		cr.Status.AtProvider.ID = p.ID.String()
		return managed.ExternalUpdate{}, nil
	}

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
