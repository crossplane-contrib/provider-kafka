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

package user

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/feature"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
	"github.com/crossplane/crossplane-runtime/v2/pkg/ratelimiter"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	"github.com/crossplane/crossplane-runtime/v2/pkg/statemetrics"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/twmb/franz-go/pkg/kadm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane-contrib/provider-kafka/apis/namespaced/user/v1alpha1"
	apisv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/namespaced/v1alpha1"
	commonv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/v1alpha1"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka"
	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka/user"
)

const (
	errGetCPC       = "cannot get ClusterProviderConfig"
	errGetCreds     = "cannot get credentials"
	errGetPC        = "cannot get ProviderConfig"
	errNewClient    = "cannot create new Kafka client"
	errNotUser      = "managed resource is not a User custom resource"
	errTrackPCUsage = "cannot track ProviderConfig usage"

	errGetPasswordSecret      = "cannot get password secret"
	errEmptyPasswordSecretKey = "password secret key is missing or empty"
	errUpsertUser             = "cannot upsert Kafka user"
	errDeleteUser             = "cannot delete Kafka user"
	errObserveUser            = "cannot observe Kafka user"
)

const (
	passwordAlphabet  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	passwordLength    = 32
	passwordSecretKey = "password"
)

// A connector is expected to produce an ExternalClient when its Connect method is called.
type connector struct {
	cache        *kafka.ClientCache
	kube         client.Client
	newServiceFn func(ctx context.Context, creds []byte, kube client.Client) (*kadm.Client, error)
	usage        *resource.ProviderConfigUsageTracker
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	kafkaClient *kadm.Client
	brokers     []string
	kube        client.Client
}

// Setup adds a controller that reconciles User managed resources.
func Setup(mgr ctrl.Manager, o controller.Options) error {
	name := managed.ControllerName(v1alpha1.UserGroupKind)

	opts := []managed.ReconcilerOption{
		managed.WithExternalConnector(&connector{
			cache:        &kafka.ClientCache{},
			kube:         mgr.GetClient(),
			usage:        resource.NewProviderConfigUsageTracker(mgr.GetClient(), &apisv1alpha1.ProviderConfigUsage{}),
			newServiceFn: kafka.NewAdminClient,
		}),
		managed.WithLogger(o.Logger.WithValues("controller", name)),
		managed.WithPollInterval(o.PollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorderFor(name))), //nolint:staticcheck // crossplane-runtime doesn't support new events API yet
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
			mgr.GetClient(), o.Logger, o.MetricOptions.MRStateMetrics, &v1alpha1.UserList{}, o.MetricOptions.PollStateMetricInterval,
		)
		if err := mgr.Add(stateMetricsRecorder); err != nil {
			return fmt.Errorf("cannot register MR state metrics recorder for kind v1alpha1.UserList: %w", err)
		}
	}

	r := managed.NewReconciler(mgr, resource.ManagedKind(v1alpha1.UserGroupVersionKind), opts...)

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(o.ForControllerRuntime()).
		WithEventFilter(resource.DesiredStateChanged()).
		For(&v1alpha1.User{}).
		Complete(ratelimiter.NewReconciler(name, r, o.GlobalRateLimiter))
}

// SetupGated adds a controller with safe-start support.
func SetupGated(mgr ctrl.Manager, o controller.Options) error {
	o.Gate.Register(func() {
		if err := Setup(mgr, o); err != nil {
			panic(fmt.Errorf("cannot setup User controller: %w", err))
		}
	}, v1alpha1.UserGroupVersionKind)
	return nil
}

func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	cr, ok := mg.(*v1alpha1.User)
	if !ok {
		return nil, errors.New(errNotUser)
	}

	if err := c.usage.Track(ctx, cr); err != nil {
		return nil, fmt.Errorf("%s: %w", errTrackPCUsage, err)
	}

	var cd apisv1alpha1.ProviderCredentials

	m := mg.(resource.ModernManaged)
	ref := m.GetProviderConfigReference()

	switch ref.Kind {
	case "ProviderConfig":
		pc := &apisv1alpha1.ProviderConfig{}
		if err := c.kube.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: m.GetNamespace()}, pc); err != nil {
			return nil, fmt.Errorf("%s: %w", errGetPC, err)
		}
		cd = pc.Spec.Credentials
	case "ClusterProviderConfig":
		cpc := &apisv1alpha1.ClusterProviderConfig{}
		if err := c.kube.Get(ctx, types.NamespacedName{Name: ref.Name}, cpc); err != nil {
			return nil, fmt.Errorf("%s: %w", errGetCPC, err)
		}
		cd = cpc.Spec.Credentials
	default:
		return nil, fmt.Errorf("unsupported provider config kind: %s", ref.Kind)
	}

	data, err := resource.CommonCredentialExtractor(ctx, cd.Source, c.kube, cd.CommonCredentialSelectors)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errGetCreds, err)
	}

	svc, err := c.cache.GetOrCreate(data, func() (*kadm.Client, error) {
		return c.newServiceFn(ctx, data, c.kube)
	})
	if err != nil {
		return nil, fmt.Errorf("%s: %w", errNewClient, err)
	}

	cfg := kafka.Config{}
	if err := json.Unmarshal(data, &cfg); err == nil {
		return &external{kafkaClient: svc, brokers: cfg.Brokers, kube: c.kube}, nil
	}

	return &external{kafkaClient: svc, kube: c.kube}, nil
}

func (c *external) Disconnect(_ context.Context) error {
	c.kafkaClient = nil
	return nil
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1alpha1.User)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotUser)
	}

	username := meta.GetExternalName(cr)
	exists, mechs, err := user.Describe(ctx, c.kafkaClient, username)
	if err != nil {
		return managed.ExternalObservation{}, fmt.Errorf("%s: %w", errObserveUser, err)
	}
	if !exists {
		return managed.ExternalObservation{ResourceExists: false}, nil
	}

	cr.Status.AtProvider.Mechanisms = mechs
	cr.Status.SetConditions(xpv2.Available())

	desiredMechs := desiredMechanisms(cr.Spec.ForProvider.Mechanisms)
	obs := managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: user.IsUpToDate(mechs, desiredMechs),
	}

	// When a passwordSecretRef is provided the password is known, so we can
	// populate the connection Secret during Observe. This allows importing a
	// pre-existing Kafka user into Crossplane management without triggering
	// an unnecessary credential rotation.
	if ref := cr.Spec.ForProvider.PasswordSecretRef; ref != nil {
		s := &corev1.Secret{}
		if err := c.kube.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: cr.GetNamespace()}, s); err != nil {
			return managed.ExternalObservation{}, fmt.Errorf("%s: %w", errGetPasswordSecret, err)
		}
		pw := s.Data[ref.Key]
		if len(pw) == 0 {
			return managed.ExternalObservation{}, errors.New(errEmptyPasswordSecretKey)
		}
		obs.ConnectionDetails = connectionDetails(username, string(pw), c.brokers)
	}

	return obs, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1alpha1.User)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotUser)
	}

	username := meta.GetExternalName(cr)
	password, err := c.resolvePassword(ctx, cr)
	if err != nil {
		return managed.ExternalCreation{}, err
	}

	mechs := desiredMechanisms(cr.Spec.ForProvider.Mechanisms)
	if err := user.Upsert(ctx, c.kafkaClient, username, password, mechs); err != nil {
		return managed.ExternalCreation{}, fmt.Errorf("%s: %w", errUpsertUser, err)
	}

	return managed.ExternalCreation{
		ConnectionDetails: connectionDetails(username, password, c.brokers),
	}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1alpha1.User)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotUser)
	}

	username := meta.GetExternalName(cr)
	password, err := c.resolvePassword(ctx, cr)
	if err != nil {
		return managed.ExternalUpdate{}, err
	}

	mechs := desiredMechanisms(cr.Spec.ForProvider.Mechanisms)
	if err := user.Upsert(ctx, c.kafkaClient, username, password, mechs); err != nil {
		return managed.ExternalUpdate{}, fmt.Errorf("%s: %w", errUpsertUser, err)
	}

	return managed.ExternalUpdate{
		ConnectionDetails: connectionDetails(username, password, c.brokers),
	}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1alpha1.User)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotUser)
	}
	cr.Status.SetConditions(xpv2.Deleting())

	username := meta.GetExternalName(cr)
	mechs := cr.Status.AtProvider.Mechanisms
	if len(mechs) == 0 {
		mechs = desiredMechanisms(cr.Spec.ForProvider.Mechanisms)
	}

	if err := user.Delete(ctx, c.kafkaClient, username, mechs); err != nil {
		return managed.ExternalDelete{}, fmt.Errorf("%s: %w", errDeleteUser, err)
	}

	return managed.ExternalDelete{}, nil
}

// resolvePassword implements the three-branch password resolution logic:
//  1. PasswordSecretRef is set → read from that Secret
//  2. Output Secret exists with a "password" key → reuse (preserves auto-generated password)
//  3. Neither → generate a new random password
func (c *external) resolvePassword(ctx context.Context, cr *v1alpha1.User) (string, error) {
	// Branch 1: explicit password secret reference
	if ref := cr.Spec.ForProvider.PasswordSecretRef; ref != nil {
		s := &corev1.Secret{}
		if err := c.kube.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: cr.GetNamespace()}, s); err != nil {
			return "", fmt.Errorf("%s: %w", errGetPasswordSecret, err)
		}
		pw := s.Data[ref.Key]
		if len(pw) == 0 {
			return "", errors.New(errEmptyPasswordSecretKey)
		}
		return string(pw), nil
	}

	// Branch 2: reuse password from existing output Secret
	if ref := cr.GetWriteConnectionSecretToReference(); ref != nil {
		s := &corev1.Secret{}
		// namespaced: LocalSecretReference has only Name; Secret lives in the CR's namespace
		err := c.kube.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: cr.GetNamespace()}, s)
		if err == nil {
			if pw, ok := s.Data[passwordSecretKey]; ok && len(pw) > 0 {
				return string(pw), nil
			}
		}
	}

	// Branch 3: auto-generate
	return generatePassword()
}

// desiredMechanisms returns the mechanisms from spec, defaulting to SCRAM-SHA-512.
func desiredMechanisms(mechanisms []commonv1alpha1.Mechanism) []string {
	if len(mechanisms) == 0 {
		return []string{"SCRAM-SHA-512"}
	}
	mechs := make([]string, len(mechanisms))
	for i, m := range mechanisms {
		mechs[i] = string(m)
	}
	return mechs
}

// connectionDetails assembles the managed resource connection detail map.
func connectionDetails(username, password string, brokers []string) managed.ConnectionDetails {
	return managed.ConnectionDetails{
		"username":        []byte(username),
		passwordSecretKey: []byte(password),
		"brokers":         []byte(strings.Join(brokers, ",")),
	}
}

// generatePassword returns a cryptographically random alphanumeric password.
func generatePassword() (string, error) {
	b := make([]byte, passwordLength)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(passwordAlphabet))))
		if err != nil {
			return "", err
		}
		b[i] = passwordAlphabet[n.Int64()]
	}
	return string(b), nil
}
