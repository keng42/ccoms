package bank

import (
	"ccoms/pkg/xetcd"

	"github.com/nats-io/nats.go"
)

// SubNats subscribes to messages from ingress via NATS
func (w *Worker) SubNats() (err error) {
	// TODO should retry if etcd get failed
	natsUrl, err := xetcd.Get(xetcd.KeyNatsService(w.Coin))
	if err != nil {
		return
	}

	// Connect to NATS
	nc, err := nats.Connect(natsUrl)
	if err != nil {
		return
	}

	// Create JetStream Context
	js, err := nc.JetStream(nats.PublishAsyncMaxPending(256))
	if err != nil {
		return
	}

	// TODO nats.StartSequence(w.LatestMsgSeq) is not working?
	ch2 := make(chan *nats.Msg, 256)
	_, err = js.ChanSubscribe("BANK."+w.Coin+".*", ch2, nats.StartSequence(w.LatestMsgSeq+1), nats.AckAll())
	if err != nil {
		return
	}

	for {
		m, ok := <-ch2
		if !ok {
			return
		}
		// if firstNatsOrderReqTime.IsZero() {
		// 	firstNatsOrderReqTime = time.Now()
		// }
		w.ch <- BankMsg{N: m}
	}
}
