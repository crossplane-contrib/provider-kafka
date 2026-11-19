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

	"github.com/twmb/franz-go/pkg/kadm"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka"
)

// ClientCache returns a cached Kafka client and a func that releases it.
type ClientCache interface {
	GetOrCreate(creds []byte, newFn func() (*kadm.Client, error)) (*kadm.Client, func(), error)
}

// NewClientFn builds a Kafka client from provider credentials.
type NewClientFn func(ctx context.Context, creds []byte, kube client.Client) (*kadm.Client, error)

// KafkaClient returns the client cached for the provider credentials creds, or
// builds one with newFn, and a func that releases it. A rotated TLS Secret or
// CA file gets a new client.
func KafkaClient(ctx context.Context, cache ClientCache, kube client.Client, creds []byte, newFn NewClientFn) (*kadm.Client, func(), error) {
	key, err := kafka.CacheKey(ctx, creds, kube)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", ErrNewClient, err)
	}
	svc, release, err := cache.GetOrCreate(key, func() (*kadm.Client, error) {
		return newFn(ctx, creds, kube)
	})
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", ErrNewClient, err)
	}
	return svc, release, nil
}
