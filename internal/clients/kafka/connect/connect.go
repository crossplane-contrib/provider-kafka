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

// Package connect is a minimal client for the Kafka Connect REST API.
package connect

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane-contrib/provider-kafka/internal/clients/kafka"
)

// Connector state actions, used as the last path segment of
// PUT /connectors/{name}/{action}.
const (
	ActionPause  = "pause"
	ActionResume = "resume"
	ActionStop   = "stop"
)

const (
	errMissingConnect = "provider config has no connect block with a url"

	requestTimeout = 30 * time.Second
	maxBodyBytes   = 1 << 20
)

// Client talks to one Kafka Connect cluster.
type Client struct {
	http       *http.Client
	base       string
	user, pass string
}

// Status is the response of GET /connectors/{name}/status.
type Status struct {
	Connector struct {
		State    string `json:"state"`
		WorkerID string `json:"worker_id"`
	} `json:"connector"`
	Tasks []TaskStatus `json:"tasks"`
}

// TaskStatus is the state of one connector task.
type TaskStatus struct {
	ID       int32  `json:"id"`
	State    string `json:"state"`
	WorkerID string `json:"worker_id"`
}

// New builds a Client from the connect block of the provider config.
func New(ctx context.Context, c *kafka.Connect, kube client.Client) (*Client, error) {
	if c == nil || c.URL == "" {
		return nil, errors.New(errMissingConnect)
	}
	tc, err := kafka.NewTLSConfig(ctx, c.TLS, kube)
	if err != nil {
		return nil, err
	}
	cl := &Client{
		base: strings.TrimSuffix(c.URL, "/"),
		http: &http.Client{
			Timeout:   requestTimeout,
			Transport: &http.Transport{TLSClientConfig: tc, Proxy: http.ProxyFromEnvironment},
		},
	}
	if c.BasicAuth != nil {
		cl.user, cl.pass = c.BasicAuth.Username, c.BasicAuth.Password
	}
	return cl, nil
}

// GetConfig returns the stored connector config. exists is false when the
// connector does not exist.
func (c *Client) GetConfig(ctx context.Context, name string) (cfg map[string]string, exists bool, err error) {
	code, err := c.do(ctx, http.MethodGet, connectorPath(name, "config"), nil, &cfg)
	if code == http.StatusNotFound {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return cfg, true, nil
}

// PutConfig creates the connector or replaces its config.
func (c *Client) PutConfig(ctx context.Context, name string, cfg map[string]string) error {
	_, err := c.do(ctx, http.MethodPut, connectorPath(name, "config"), cfg, nil)
	return err
}

// GetStatus returns the connector and task states.
func (c *Client) GetStatus(ctx context.Context, name string) (*Status, error) {
	st := &Status{}
	if _, err := c.do(ctx, http.MethodGet, connectorPath(name, "status"), nil, st); err != nil {
		return nil, err
	}
	return st, nil
}

// SetState pauses, resumes or stops the connector. action is one of
// ActionPause, ActionResume or ActionStop.
func (c *Client) SetState(ctx context.Context, name, action string) error {
	_, err := c.do(ctx, http.MethodPut, connectorPath(name, action), nil, nil)
	return err
}

// Delete deletes the connector. A connector that does not exist is not an error.
func (c *Client) Delete(ctx context.Context, name string) error {
	code, err := c.do(ctx, http.MethodDelete, connectorPath(name, ""), nil, nil)
	if code == http.StatusNotFound {
		return nil
	}
	return err
}

// Close releases idle connections.
func (c *Client) Close() {
	c.http.CloseIdleConnections()
}

func connectorPath(name, sub string) string {
	p := "/connectors/" + url.PathEscape(name)
	if sub != "" {
		p += "/" + sub
	}
	return p
}

// do sends a request with an optional JSON body and decodes a 2xx response
// into out when out is non-nil. It returns the HTTP status code, or 0 when the
// request did not complete. Request bodies never appear in errors because
// connector configs can hold secrets.
func (c *Client) do(ctx context.Context, method, path string, in, out any) (int, error) {
	req, err := c.newRequest(ctx, method, path, in)
	if err != nil {
		return 0, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close() //nolint:errcheck // read-only body

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return resp.StatusCode, fmt.Errorf("%s %s: cannot read response: %w", method, path, err)
	}

	if resp.StatusCode >= http.StatusMultipleChoices {
		return resp.StatusCode, fmt.Errorf("%s %s: %d: %s", method, path, resp.StatusCode, errorMessage(resp.StatusCode, data))
	}

	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return resp.StatusCode, fmt.Errorf("%s %s: cannot decode response: %w", method, path, err)
		}
	}
	return resp.StatusCode, nil
}

func (c *Client) newRequest(ctx context.Context, method, path string, in any) (*http.Request, error) {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return nil, fmt.Errorf("cannot encode request: %w", err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return nil, fmt.Errorf("cannot build request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.user != "" {
		req.SetBasicAuth(c.user, c.pass)
	}
	return req, nil
}

// errorMessage extracts the message field of a Connect error response.
func errorMessage(code int, data []byte) string {
	var e struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(data, &e) == nil && e.Message != "" {
		return e.Message
	}
	return http.StatusText(code)
}
