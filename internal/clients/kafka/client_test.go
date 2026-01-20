package kafka

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var credentials struct {
	Brokers []string `json:"brokers"`
	SASL    struct {
		Mechanism string `json:"mechanism"`
		Username  string `json:"username"`
		Password  string `json:"password"`
	} `json:"sasl"`
}

var dataTesting = []byte(os.Getenv("KAFKA_CONFIG"))

func TestNewAdminClient_ValidCredentials(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := NewAdminClient(ctx, dataTesting, nil)
	require.NoError(t, err, "expected no error getting the client, got: %v", err)
	assert.NotNil(t, client, "expected client to be non-nil")

	brokers, err := client.ListBrokers(ctx)
	require.NoError(t, err, "expected no error listing brokers, got: %v", err)
	assert.NotEmpty(t, brokers, "expected non-empty list of brokers")

}

func TestNewAdminClient_WrongCredentials(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := json.Unmarshal(dataTesting, &credentials)
	if err != nil {
		t.Fatalf("failed to unmarshal dataTesting: %v", err)
	}
	brokersFromDataTesting, err := json.Marshal(credentials.Brokers)
	if err != nil {
		t.Fatalf("failed to marshal brokers: %v", err)
	}

	badCredentials := []byte(`{
		"brokers": ` + string(brokersFromDataTesting) + `,
		"sasl": {
			"mechanism": "` + credentials.SASL.Mechanism + `",
			"username": "wrong-user",
			"password": "wrong-pass"
		}
	}`)
	client, err := NewAdminClient(ctx, badCredentials, nil)
	require.NoError(t, err, "expected no error getting the client with wrong credentials, got: %v", err)
	assert.NotNil(t, client, "expected client to be non-nil even with wrong credentials")

	brokers, err := client.ListBrokers(ctx)
	assert.Nil(t, brokers, "expected brokers to be nil on error")
	require.Error(t, err, "expected error when using a client with wrong credentials, got: %v", err)
}

func TestNewAdminClient_EmptyPassword(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	brokersFromDataTesting, err := json.Marshal(credentials.Brokers)
	if err != nil {
		t.Fatalf("failed to marshal brokers: %v", err)
	}
	data := []byte(`{
		"brokers": ` + string(brokersFromDataTesting) + `,
		"sasl": {
			"mechanism": "` + credentials.SASL.Mechanism + `",
			"username": "` + credentials.SASL.Username + `",
			"password": ""
		}
	}`)
	client, err := NewAdminClient(ctx, data, nil)
	assert.Nil(t, client, "expected client to be nil on SASL and empty password")
	assert.Error(t, err, "expected error with empty password, got nil")
}

func TestNewAdminClient_MissingSASLConfig(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	brokersFromDataTesting, err := json.Marshal(credentials.Brokers)
	if err != nil {
		t.Fatalf("failed to marshal brokers: %v", err)
	}
	data := []byte(`{ "brokers": ` + string(brokersFromDataTesting) + ` }`)
	client, err := NewAdminClient(ctx, data, nil)
	assert.Nil(t, client, "expected client to be nil on error")
	assert.Error(t, err, "expected error with missing SASL config, got nil")
}
