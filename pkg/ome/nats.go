package ome

// func (w *Worker) GetNats() (js nats.JetStreamContext, err error) {
// 	if w.Nats != nil {
// 		return w.Nats, nil
// 	}

// 	// Connect to NATS
// 	nc, err := nats.Connect(nats.DefaultURL)
// 	if err != nil {
// 		return
// 	}

// 	// Create JetStream Context
// 	js, err = nc.JetStream(nats.PublishAsyncMaxPending(256))
// 	if err != nil {
// 		return
// 	}
// 	w.Nats = js

// 	return
// }

// func (w *Worker) SendBalancesReq(msg xnats.BalancesReq) (err error) {
// 	_, err = w.GetNats()
// 	if err != nil {
// 		return
// 	}
// 	data, err := json.Marshal(msg)
// 	if err != nil {
// 		return

// 	}
// 	_, err = w.Nats.Publish("BANK.USDT.BalancesReq", data)

// 	return
// }
