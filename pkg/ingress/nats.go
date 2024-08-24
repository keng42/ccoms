package ingress

import (
	"ccoms/pkg/xetcd"
	"ccoms/pkg/xnats"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nats-io/nats.go"
)

func (w *Worker) GetNats(coin string) (js nats.JetStreamContext, err error) {
	if w.Nats[coin] != nil {
		return w.Nats[coin], nil
	}

	natsUrl, err := xetcd.Get(xetcd.KeyNatsService(coin))
	if err != nil {
		return
	}

	// Connect to NATS
	nc, err := nats.Connect(natsUrl)
	if err != nil {
		return
	}

	// Create JetStream Context
	js, err = nc.JetStream(nats.PublishAsyncMaxPending(256))
	if err != nil {
		return
	}
	w.Nats[coin] = js

	return
}

func (w *Worker) SendOrderReq(bankCoin string, msg xnats.OrderReq) (err error) {
	js, err := w.GetNats(bankCoin)
	if err != nil {
		return
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}
	_, err = js.Publish(fmt.Sprintf("BANK.%s.OrderReq", strings.ToUpper(bankCoin)), data)

	return
}
