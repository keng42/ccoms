package ingress

import "github.com/nats-io/nats.go"

type Worker struct {
	Nats map[string]nats.JetStreamContext
}
