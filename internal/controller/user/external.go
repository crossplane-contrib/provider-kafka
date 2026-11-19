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

package user

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	commonv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/v1alpha1"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka"
	kafkauser "github.com/crossplane-contrib/provider-kafka/internal/clients/kafka/user"
	"github.com/crossplane-contrib/provider-kafka/internal/controller/common"
)

// Resource is a User managed resource of either scope. The scope decides which
// namespace the password and connection Secrets live in.
type Resource interface {
	resource.Managed
	GetMechanisms() []commonv1alpha1.Mechanism
	GetUserObservation() *commonv1alpha1.UserObservation
	GetPasswordSecret() *commonv1alpha1.SecretKeySelector
	GetConnectionSecret() *types.NamespacedName
}

// External observes, creates, updates and deletes a Kafka SCRAM user.
type External struct {
	kafkaClient kafkauser.ScramClient
	release     func()
	brokers     []string
	kube        client.Client
}

// Connect parses the provider credentials creds for the broker list and
// returns an External backed by the Kafka client cached for them.
func Connect(ctx context.Context, cache common.ClientCache, kube client.Client, creds []byte, newFn common.NewClientFn) (*External, error) {
	cfg := kafka.Config{}
	if err := json.Unmarshal(creds, &cfg); err != nil {
		return nil, fmt.Errorf("%s: %w", ErrParseCreds, err)
	}

	svc, release, err := common.KafkaClient(ctx, cache, kube, creds, newFn)
	if err != nil {
		return nil, err
	}
	return &External{kafkaClient: svc, release: release, brokers: cfg.Brokers, kube: kube}, nil
}

// Disconnect releases the cached client.
func (e *External) Disconnect(_ context.Context) error {
	e.kafkaClient = nil
	if e.release != nil {
		e.release()
	}
	return nil
}

// Observe reports whether the user exists and has the desired mechanisms.
func (e *External) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(Resource)
	if !ok {
		return managed.ExternalObservation{}, errors.New(ErrNotUser)
	}

	username := meta.GetExternalName(cr)
	exists, mechs, err := kafkauser.Describe(ctx, e.kafkaClient, username)
	if err != nil {
		return managed.ExternalObservation{}, fmt.Errorf("%s: %w", ErrObserveUser, err)
	}
	if !exists {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.GetUserObservation().Mechanisms = mechs
	cr.SetConditions(xpv2.Available())

	// Kafka SCRAM API does not expose stored password hashes, so only mechanism
	// changes are detectable. Changing passwordSecretRef alone won't trigger an
	// Update. Bump the external-name annotation to force re-upsert.
	obs := managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: kafkauser.IsUpToDate(mechs, DesiredMechanisms(cr.GetMechanisms())),
	}

	// When a passwordSecretRef is provided the password is known, so we can
	// populate the connection Secret during Observe. This allows importing a
	// pre-existing Kafka user into Crossplane management without triggering
	// an unnecessary credential rotation.
	if ref := cr.GetPasswordSecret(); ref != nil {
		pw, err := e.readPassword(ctx, ref)
		if err != nil {
			return managed.ExternalObservation{}, err
		}
		obs.ConnectionDetails = ConnectionDetails(username, pw, e.brokers)
	}

	return obs, nil
}

// Create creates the user with the resolved password.
func (e *External) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(Resource)
	if !ok {
		return managed.ExternalCreation{}, errors.New(ErrNotUser)
	}

	username := meta.GetExternalName(cr)
	password, err := e.resolvePassword(ctx, cr)
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	if err := kafkauser.Upsert(ctx, e.kafkaClient, username, password, DesiredMechanisms(cr.GetMechanisms())); err != nil {
		return managed.ExternalCreation{}, fmt.Errorf("%s: %w", ErrUpsertUser, err)
	}

	return managed.ExternalCreation{
		ConnectionDetails: ConnectionDetails(username, password, e.brokers),
	}, nil
}

// Update re-enrolls the user and removes mechanisms dropped from the spec.
func (e *External) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(Resource)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(ErrNotUser)
	}

	username := meta.GetExternalName(cr)
	password, err := e.resolvePassword(ctx, cr)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	mechs := DesiredMechanisms(cr.GetMechanisms())
	if err := kafkauser.Upsert(ctx, e.kafkaClient, username, password, mechs); err != nil {
		return managed.ExternalUpdate{}, fmt.Errorf("%s: %w", ErrUpsertUser, err)
	}

	// Mechanisms dropped from the spec stay enrolled in Kafka unless deleted,
	// which would leave the resource permanently not up to date.
	if removed := kafkauser.Removed(cr.GetUserObservation().Mechanisms, mechs); len(removed) > 0 {
		if err := kafkauser.Delete(ctx, e.kafkaClient, username, removed); err != nil {
			return managed.ExternalUpdate{}, fmt.Errorf("%s: %w", ErrDeleteUser, err)
		}
	}

	return managed.ExternalUpdate{
		ConnectionDetails: ConnectionDetails(username, password, e.brokers),
	}, nil
}

// Delete removes the user's SCRAM credentials.
func (e *External) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(Resource)
	if !ok {
		return managed.ExternalDelete{}, errors.New(ErrNotUser)
	}
	cr.SetConditions(xpv2.Deleting())

	mechs := cr.GetUserObservation().Mechanisms
	if len(mechs) == 0 {
		mechs = DesiredMechanisms(cr.GetMechanisms())
	}

	if err := kafkauser.Delete(ctx, e.kafkaClient, meta.GetExternalName(cr), mechs); err != nil {
		return managed.ExternalDelete{}, fmt.Errorf("%s: %w", ErrDeleteUser, err)
	}

	return managed.ExternalDelete{}, nil
}

// resolvePassword implements the three-branch password resolution logic:
//  1. PasswordSecretRef is set → read from that Secret
//  2. Output Secret exists with a "password" key → reuse (preserves auto-generated password)
//  3. Neither → generate a new random password
func (e *External) resolvePassword(ctx context.Context, cr Resource) (string, error) {
	// Branch 1: explicit password secret reference
	if ref := cr.GetPasswordSecret(); ref != nil {
		return e.readPassword(ctx, ref)
	}

	// Branch 2: reuse password from existing output Secret
	if ref := cr.GetConnectionSecret(); ref != nil {
		s := &corev1.Secret{}
		if err := e.kube.Get(ctx, *ref, s); err == nil {
			if pw, ok := s.Data[PasswordSecretKey]; ok && len(pw) > 0 {
				return string(pw), nil
			}
		}
	}

	// Branch 3: auto-generate
	return GeneratePassword()
}

// readPassword reads the password from the Secret key ref selects.
func (e *External) readPassword(ctx context.Context, ref *commonv1alpha1.SecretKeySelector) (string, error) {
	s := &corev1.Secret{}
	if err := e.kube.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: ref.Namespace}, s); err != nil {
		return "", fmt.Errorf("%s: %w", ErrGetPasswordSecret, err)
	}
	pw := s.Data[ref.Key]
	if len(pw) == 0 {
		return "", errors.New(ErrEmptyPasswordSecretKey)
	}
	return string(pw), nil
}
