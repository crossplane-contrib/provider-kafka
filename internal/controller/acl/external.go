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

// Package acl holds the AccessControlList controller logic shared by the
// cluster and namespaced scopes.
package acl

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
	kafkaacl "github.com/crossplane-contrib/provider-kafka/internal/clients/kafka/acl"
	"github.com/crossplane-contrib/provider-kafka/internal/controller/common"
)

// Resource is an AccessControlList managed resource of either scope.
type Resource interface {
	resource.Managed
	GetACLParameters() *commonv1alpha1.AccessControlListParameters
	GetACLObservation() *commonv1alpha1.AccessControlListObservation
}

// External observes, creates and deletes a Kafka ACL.
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

// Observe reports whether the ACL exists and matches the spec.
func (e *External) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(Resource)
	if !ok {
		return managed.ExternalObservation{}, errors.New(common.ErrNotAccessControlList)
	}

	// Check if the external name is set, to determine if ACL has been created or not
	ext := meta.GetExternalName(cr)
	if ext == "" {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	extName, err := kafkaacl.ConvertFromJSON(ext)
	if err != nil {
		return managed.ExternalObservation{}, fmt.Errorf("could not convert external name from JSON: %w", err)
	}
	if extName == nil {
		return managed.ExternalObservation{}, fmt.Errorf("could not convert external name from JSON: nil result")
	}
	generated := kafkaacl.Generate(cr.GetACLParameters())
	compare := kafkaacl.CompareAcls(*extName, *generated)
	diff := kafkaacl.Diff(*extName, *generated)

	if !compare {
		err := strings.Join(diff, " ")
		return managed.ExternalObservation{
			ResourceExists:   true,
			ResourceUpToDate: false,
		}, errors.New(err)
	}

	ae, err := kafkaacl.List(ctx, e.kafkaClient, extName)
	if err != nil {
		return managed.ExternalObservation{}, fmt.Errorf("%s: %w", common.ErrListACL, err)
	}

	if ae == nil {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	obs := cr.GetACLObservation()
	obs.ResourceName = ae.ResourceName
	obs.ResourceType = ae.ResourceType
	obs.ResourcePrincipal = ae.ResourcePrincipal
	obs.ResourceHost = ae.ResourceHost
	obs.ResourceOperation = ae.ResourceOperation
	obs.ResourcePermissionType = ae.ResourcePermissionType
	obs.ResourcePatternTypeFilter = ae.ResourcePatternTypeFilter
	cr.SetConditions(xpv2.Available())

	return managed.ExternalObservation{
		ResourceExists:          true,
		ResourceUpToDate:        true,
		ResourceLateInitialized: false,
	}, nil
}

// Create creates the ACL and stores it as JSON in the external name.
func (e *External) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(Resource)
	if !ok {
		return managed.ExternalCreation{}, errors.New(common.ErrNotAccessControlList)
	}

	generated := kafkaacl.Generate(cr.GetACLParameters())
	extName, err := kafkaacl.ConvertToJSON(generated)
	if err != nil {
		return managed.ExternalCreation{}, fmt.Errorf("could not convert external name to JSON: %w", err)
	}
	// Always set the external name to the JSON form to ensure it's valid,
	// even if it was previously set to a non-JSON value (e.g., by default initializers).
	meta.SetExternalName(cr, extName)
	return managed.ExternalCreation{}, kafkaacl.Create(ctx, e.kafkaClient, generated)
}

// Update is not supported: Kafka ACLs are immutable.
func (e *External) Update(_ context.Context, _ resource.Managed) (managed.ExternalUpdate, error) {
	return managed.ExternalUpdate{}, errors.New(common.ErrUpdateNotSupported)
}

// Delete deletes the ACL.
func (e *External) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(Resource)
	if !ok {
		return managed.ExternalDelete{}, errors.New(common.ErrNotAccessControlList)
	}
	cr.SetConditions(xpv2.Deleting())

	return managed.ExternalDelete{}, kafkaacl.Delete(ctx, e.kafkaClient, kafkaacl.Generate(cr.GetACLParameters()))
}
