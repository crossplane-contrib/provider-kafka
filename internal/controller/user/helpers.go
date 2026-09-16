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
	"crypto/rand"
	"math/big"
	"strings"

	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"

	commonv1alpha1 "github.com/crossplane-contrib/provider-kafka/apis/v1alpha1"
)

const (
	PasswordAlphabet  = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	PasswordLength    = 32
	PasswordSecretKey = "password"
)

const (
	ErrParseCreds   = "cannot parse provider credentials for broker list"
	ErrNotUser      = "managed resource is not a User custom resource"

	ErrGetPasswordSecret      = "cannot get password secret"
	ErrEmptyPasswordSecretKey = "password secret key is missing or empty"
	ErrUpsertUser             = "cannot upsert Kafka user"
	ErrDeleteUser             = "cannot delete Kafka user"
	ErrObserveUser            = "cannot observe Kafka user"
)

// DesiredMechanisms returns the mechanisms from spec, defaulting to SCRAM-SHA-512.
func DesiredMechanisms(mechanisms []commonv1alpha1.Mechanism) []string {
	if len(mechanisms) == 0 {
		return []string{"SCRAM-SHA-512"}
	}
	mechs := make([]string, len(mechanisms))
	for i, m := range mechanisms {
		mechs[i] = string(m)
	}
	return mechs
}

// ConnectionDetails assembles the managed resource connection detail map.
func ConnectionDetails(username, password string, brokers []string) managed.ConnectionDetails {
	return managed.ConnectionDetails{
		"username":        []byte(username),
		PasswordSecretKey: []byte(password),
		"brokers":         []byte(strings.Join(brokers, ",")),
	}
}

// GeneratePassword returns a cryptographically random alphanumeric password.
func GeneratePassword() (string, error) {
	b := make([]byte, PasswordLength)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(PasswordAlphabet))))
		if err != nil {
			return "", err
		}
		b[i] = PasswordAlphabet[n.Int64()]
	}
	return string(b), nil
}
