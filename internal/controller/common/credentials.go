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

package common

import (
	"context"
	"fmt"

	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/cluster/v1alpha1"
	namespacedv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/namespaced/v1alpha1"
)

// ClusterCredentials records that the cluster-scoped resource mg uses its
// ProviderConfig and returns the credentials that ProviderConfig points to.
func ClusterCredentials(ctx context.Context, kube client.Client, usage *resource.LegacyProviderConfigUsageTracker, mg resource.LegacyManaged) ([]byte, error) { //nolint:staticcheck // cluster-scoped resources still use the legacy interface
	if err := usage.Track(ctx, mg); err != nil {
		return nil, fmt.Errorf("%s: %w", ErrTrackPCUsage, err)
	}

	pc := &clusterv1alpha1.ProviderConfig{}
	if err := kube.Get(ctx, types.NamespacedName{Name: mg.GetProviderConfigReference().Name}, pc); err != nil {
		return nil, fmt.Errorf("%s: %w", ErrGetPC, err)
	}
	return extract(ctx, kube, pc.Spec.Credentials.Source, pc.Spec.Credentials.CommonCredentialSelectors)
}

// NamespacedCredentials records that the namespaced resource mg uses its
// ProviderConfig or ClusterProviderConfig and returns the credentials it points to.
func NamespacedCredentials(ctx context.Context, kube client.Client, usage *resource.ProviderConfigUsageTracker, mg resource.ModernManaged) ([]byte, error) {
	if err := usage.Track(ctx, mg); err != nil {
		return nil, fmt.Errorf("%s: %w", ErrTrackPCUsage, err)
	}

	var cd namespacedv1alpha1.ProviderCredentials
	ref := mg.GetProviderConfigReference()
	switch ref.Kind {
	case "ProviderConfig":
		pc := &namespacedv1alpha1.ProviderConfig{}
		if err := kube.Get(ctx, types.NamespacedName{Name: ref.Name, Namespace: mg.GetNamespace()}, pc); err != nil {
			return nil, fmt.Errorf("%s: %w", ErrGetPC, err)
		}
		cd = pc.Spec.Credentials
	case "ClusterProviderConfig":
		cpc := &namespacedv1alpha1.ClusterProviderConfig{}
		if err := kube.Get(ctx, types.NamespacedName{Name: ref.Name}, cpc); err != nil {
			return nil, fmt.Errorf("%s: %w", ErrGetCPC, err)
		}
		cd = cpc.Spec.Credentials
	default:
		return nil, fmt.Errorf("unsupported provider config kind: %s", ref.Kind)
	}
	return extract(ctx, kube, cd.Source, cd.CommonCredentialSelectors)
}

func extract(ctx context.Context, kube client.Client, source xpv2.CredentialsSource, selectors xpv2.CommonCredentialSelectors) ([]byte, error) {
	data, err := resource.CommonCredentialExtractor(ctx, source, kube, selectors)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", ErrGetCreds, err)
	}
	return data, nil
}
