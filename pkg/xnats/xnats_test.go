package xnats_test

import (
	"testing"

	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
)

func TestCreateStream(t *testing.T) {
	// Connect to NATS
	nc, err := nats.Connect(nats.DefaultURL)
	require.Nil(t, err)

	// Create JetStream Context
	js, err := nc.JetStream()
	require.Nil(t, err)

	// Create a Stream
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "BANK",
		Subjects: []string{"BANK.*.*"},
		// Retention: nats.WorkQueuePolicy,
	})
	require.Nil(t, err)
}
