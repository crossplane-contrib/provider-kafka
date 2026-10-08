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

// Package commontest provides fakes for controller tests.
package commontest

import "github.com/twmb/franz-go/pkg/kadm"

// ClientCache is a fake common.ClientCache that counts acquired and released clients.
type ClientCache struct {
	Acquired int
	Released int
}

// GetOrCreate returns a new client from newFn on every call.
func (c *ClientCache) GetOrCreate(_ []byte, newFn func() (*kadm.Client, error)) (*kadm.Client, func(), error) {
	client, err := newFn()
	if err != nil {
		return nil, nil, err
	}
	c.Acquired++
	return client, func() { c.Released++ }, nil
}
