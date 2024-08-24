package ome

// func (w *Worker) FiledbToBank() (err error) {
// 	ch := make(chan string, 1000)

// 	go func() {
// 		err = w.filedb.Tailf(ch)
// 		if err != nil {
// 			close(ch)
// 		}
// 	}()

// 	err2 := w.LogToBank(ch)
// 	if err == nil {
// 		err = err2
// 	}

// 	return
// }

// func (w *Worker) LogToBank(ch <-chan string) (err error) {
// 	ss := make([]string, 100)

// 	start := time.Now()
// 	idx := 0
// 	go func() {
// 		for {
// 			time.Sleep(time.Second)
// 			fmt.Println("=====", idx, time.Since(start), float64(idx)/float64(time.Since(start).Seconds()))
// 		}
// 	}()
// 	for {
// 		size := 1
// 		if len(ch) > 1 {
// 			if len(ch) < len(ss) {
// 				size = len(ch)
// 			} else {
// 				size = len(ss)
// 			}
// 		}

// 		// size = 5
// 		var ok bool
// 		for i := 0; i < size; i++ {
// 			ss[i], ok = <-ch
// 			if !ok {
// 				return
// 			}
// 		}

// 		err = w.ParseLogsAndToBank(ss[:size])
// 		if err != nil {
// 			return
// 		}

// 		idx += size
// 	}
// }

// func (w *Worker) ParseLogsAndToBank(ss []string) (err error) {
// 	latestLogID := 0

// 	for _, s := range ss {
// 		ol := new(OmeLog)
// 		err = json.Unmarshal([]byte(s), ol)
// 		if err != nil {
// 			return
// 		}

// 		if ol.MatchLogs != nil && len(ol.MatchLogs) > 0 {
// 			ml := ol.MatchLogs[0]

// 			quantity := IntToDecimal(ml.Quantity)
// 			amount := IntToDecimal(ml.Amount)

// 			newBalanceReq1 := make([]xnats.BalanceReq, 0) // BTC
// 			newBalanceReq2 := make([]xnats.BalanceReq, 0) // USDT

// 			newBalanceReq1 = append(newBalanceReq1, xnats.BalanceReq{
// 				User:         ml.Bider,
// 				Coin:         "BTC",
// 				FreeChange:   quantity,
// 				FreezeChange: decimal.Zero,
// 				Time:         ml.Time,
// 				Reason:       "",
// 			})

// 			// create 4 balance snap
// 			// bider add btc, minus usdt
// 			newBalanceReq2 = append(newBalanceReq2, xnats.BalanceReq{
// 				User:         ml.Bider,
// 				Coin:         "USDT",
// 				FreeChange:   decimal.Zero,
// 				FreezeChange: amount.Neg(),
// 				Time:         ml.Time,
// 				Reason:       "",
// 			})

// 			newBalanceReq1 = append(newBalanceReq1, xnats.BalanceReq{
// 				User:         ml.Bider,
// 				Coin:         "BTC",
// 				FreeChange:   quantity,
// 				FreezeChange: decimal.Zero,
// 				Time:         ml.Time,
// 				Reason:       "",
// 			})

// 			// asker add usdt, minus btc
// 			newBalanceReq2 = append(newBalanceReq2, xnats.BalanceReq{
// 				User:         ml.Asker,
// 				Coin:         "USDT",
// 				FreeChange:   amount,
// 				FreezeChange: decimal.Zero,
// 				Time:         ml.Time,
// 				Reason:       "",
// 			})

// 			newBalanceReq1 = append(newBalanceReq1, xnats.BalanceReq{
// 				User:         ml.Asker,
// 				Coin:         "BTC",
// 				FreeChange:   decimal.Zero,
// 				FreezeChange: quantity.Neg(),
// 				Time:         ml.Time,
// 				Reason:       "",
// 			})

// 			// send to bank nats
// 			err = w.SendBalancesReq(xnats.BalancesReq{Items: newBalanceReq1})
// 			if err != nil {
// 				return
// 			}
// 			err = w.SendBalancesReq(xnats.BalancesReq{Items: newBalanceReq2})
// 			if err != nil {
// 				return
// 			}
// 		}

// 		latestLogID = int(ol.LogID)
// 	}

// 	w.ToBankLogID = int64(latestLogID)
// 	return
// }
