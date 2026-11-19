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

// Package topic holds the Topic controller logic shared by the cluster and
// namespaced scopes.
package topic

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/twmb/franz-go/pkg/kadm"

	commonv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/v1alpha1"
	kafkatopic "github.com/crossplane-contrib/provider-kafka/internal/clients/kafka/topic"
	"github.com/crossplane-contrib/provider-kafka/internal/controller/common"
)

// Resource is a Topic managed resource of either scope.
type Resource interface {
	resource.Managed
	GetTopicParameters() *commonv1alpha1.TopicParameters
	GetTopicObservation() *commonv1alpha1.TopicObservation
}

// External observes, creates, updates and deletes a Kafka topic.
type External struct {
	kafkaClient *kadm.Client
	release     func()
}

// NewExternal returns an External that uses cl and calls release on Disconnect.
func NewExternal(cl *kadm.Client, release func()) *External {
	return &External{kafkaClient: cl, release: release}
}

// Disconnect releases the cached client.
func (e *External) Disconnect(_ context.Context) error {
	e.kafkaClient = nil
	if e.release != nil {
		e.release()
	}
	return nil
}

// Observe reports whether the topic exists and matches the spec.
func (e *External) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(Resource)
	if !ok {
		return managed.ExternalObservation{}, errors.New(common.ErrNotTopic)
	}

	tpc, err := kafkatopic.Get(ctx, e.kafkaClient, meta.GetExternalName(cr))
	if err != nil { // Discern whether the topic doesn't exist or something went wrong
		if strings.HasPrefix(err.Error(), kafkatopic.ErrTopicDoesNotExist) {
			return managed.ExternalObservation{ResourceExists: false}, nil
		}
		return managed.ExternalObservation{}, fmt.Errorf(common.ErrGetTopic+": %w", err)
	}

	// On the first reconcile, AddFinalizer performs a full-object Update that
	// resets the in-memory status before it can be persisted. Returning
	// ResourceUpToDate=false forces an Update call which runs after
	// AddFinalizer and re-populates status there.
	statusPopulated := cr.GetTopicObservation().ID != ""

	*cr.GetTopicObservation() = tpc.ToObservation()
	cr.SetConditions(xpv2.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: isResourceUpToDate(cr, statusPopulated, tpc),
	}, nil
}

func isResourceUpToDate(cr Resource, statusPopulated bool, observed *kafkatopic.Topic) bool {
	return statusPopulated && kafkatopic.IsUpToDate(cr.GetTopicParameters(), observed)
}

// Create creates the topic.
func (e *External) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(Resource)
	if !ok {
		return managed.ExternalCreation{}, errors.New(common.ErrNotTopic)
	}
	return managed.ExternalCreation{}, kafkatopic.Create(ctx, e.kafkaClient, kafkatopic.Generate(meta.GetExternalName(cr), cr.GetTopicParameters()))
}

// Update applies the spec to the topic.
func (e *External) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(Resource)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(common.ErrNotTopic)
	}

	name := meta.GetExternalName(cr)

	if err := kafkatopic.Update(ctx, e.kafkaClient, kafkatopic.Generate(name, cr.GetTopicParameters())); err != nil {
		return managed.ExternalUpdate{}, err
	}

	if cr.GetTopicObservation().ID == "" {
		tpc, err := kafkatopic.Get(ctx, e.kafkaClient, name)
		if err != nil {
			return managed.ExternalUpdate{}, fmt.Errorf(common.ErrGetTopic+": %w", err)
		}

		*cr.GetTopicObservation() = tpc.ToObservation()
		cr.SetConditions(xpv2.Available())
	}

	return managed.ExternalUpdate{}, nil
}

// Delete deletes the topic.
func (e *External) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(Resource)
	if !ok {
		return managed.ExternalDelete{}, errors.New(common.ErrNotTopic)
	}
	cr.SetConditions(xpv2.Deleting())

	return managed.ExternalDelete{}, kafkatopic.Delete(ctx, e.kafkaClient, meta.GetExternalName(cr))
}
