package bank

import (
	"encoding/json"
	"strings"

	"ccoms/pkg/model"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// FiledbToMySQL retrieves the content of filedb in real-time and writes it to MySQL
func (w *Worker) FiledbToMySQL() (err error) {
	ch := make(chan string, 1000)

	w.SavedLogID, err = w.LoadSavedLogID()
	if err != nil {
		return
	}

	// checkout lastkv
	_, err = w.CheckoutLastKv("", model.LASTKV_K_NATS_SEQ)
	if err != nil {
		return
	}

	go func() {
		err = w.fdb.Tailf(ch)
		if err != nil {
			close(ch)
		}
	}()

	err2 := w.fdb.ToMySQL(ch)
	if err == nil {
		err = err2
	}

	return
}

// ParseAndWriteLogs parses and writes logs to MySQL
func (w *Worker) ParseAndWriteLogs(ss []string) (err error) {
	latestLogID := 0
	latestMsgSeq := int64(0)

	newTicketsMap := make(map[string][]model.Ticket, 0)
	newBalanceSnaps := make([]model.BalanceSnap, 0)
	updateBalances := make(map[int64]*model.Balance)

	// ----- Parse the last log, if the latest log ID is less than or equal to the saved log ID, skip it
	ol := new(BankLog)
	err = json.Unmarshal([]byte(ss[len(ss)-1]), ol)
	if err != nil {
		logger.Errorf("ParseAndWriteLogs failed with data:%s, err:%s", ss[len(ss)-1], err)
		return
	}
	if ol.LogID <= w.SavedLogID {
		// The latest log ID is less than or equal to the saved log ID, skip it
		logger.Debugf("ParseAndWriteLogs skip latestLogID:%d <= saveLogID:%d", ol.LogID, w.SavedLogID)
		return
	}

	// ----- Parse all logs and cache them as variables for further processing
	for _, s := range ss {
		ol := new(BankLog)
		err = json.Unmarshal([]byte(s), ol)
		if err != nil {
			// TODO When concurrency is high, incomplete logs may sometimes appear here
			logger.Errorf("Unmarshal BankLog failed with data:%s, err:%s", s, err)
			return
		}

		// if ol.LogID != int64(ol.MsgSeq) {
		// 	fmt.Println("=====", s)
		// 	panic("stop")
		// }

		// ----- If the latest log ID is less than or equal to the saved log ID, skip it
		if ol.LogID <= w.SavedLogID {
			latestLogID = int(ol.LogID)
			continue
		}

		// ----- Update the latest message sequence number
		if int64(ol.MsgSeq) > latestMsgSeq {
			latestMsgSeq = int64(ol.MsgSeq)
		}

		var logIndex int64

		// ticket log
		if ol.TicketLogs != nil && len(ol.TicketLogs) > 0 {
			ml := ol.TicketLogs[0]
			price, _ := decimal.NewFromString(ml.Price)
			quantity, _ := decimal.NewFromString(ml.Quantity)
			amount, _ := decimal.NewFromString(ml.Amount)

			// create ticket
			logIndex++
			ticket := model.Ticket{
				LogType:  1,
				LogID:    ol.LogID,
				LogIndex: logIndex,
				Price:    price,
				Quantity: quantity,
				Amount:   amount,
				// Time:     ml.Time, // TODO
			}

			if _, ok := newTicketsMap[ml.Symbol]; !ok {
				newTicketsMap[ml.Symbol] = make([]model.Ticket, 0)
			}
			newTicketsMap[ml.Symbol] = append(newTicketsMap[ml.Symbol], ticket)
		}

		// balance log
		if ol.BalanceLogs != nil && len(ol.BalanceLogs) > 0 {
			ml := ol.BalanceLogs[0]
			freeChange, _ := decimal.NewFromString(ml.FreeChange)
			freezeChange, _ := decimal.NewFromString(ml.FreezeChange)
			freeNew, _ := decimal.NewFromString(ml.FreeNew)
			freezeNew, _ := decimal.NewFromString(ml.FreezeNew)

			// create balSnap
			logIndex++
			balSnap := model.BalanceSnap{
				LogType:      1,
				LogID:        ol.LogID,
				LogIndex:     logIndex,
				Owner:        ml.Owner,
				FreeChange:   freeChange,
				FreezeChange: freezeChange,
				FreeNew:      freeNew,
				FreezeNew:    freezeNew,
			}
			newBalanceSnaps = append(newBalanceSnaps, balSnap)
			updateBalances[balSnap.Owner] = &model.Balance{
				Free:   balSnap.FreeNew,
				Freeze: balSnap.FreezeNew,
			}

			if ml.Owner2 > 0 {
				freeChange, _ := decimal.NewFromString(ml.FreeChange2)
				freezeChange, _ := decimal.NewFromString(ml.FreezeChange2)
				freeNew, _ := decimal.NewFromString(ml.FreeNew2)
				freezeNew, _ := decimal.NewFromString(ml.FreezeNew2)

				// create balSnap
				logIndex++
				balSnap := model.BalanceSnap{
					LogType:      1,
					LogID:        ol.LogID,
					LogIndex:     logIndex,
					Owner:        ml.Owner2,
					FreeChange:   freeChange,
					FreezeChange: freezeChange,
					FreeNew:      freeNew,
					FreezeNew:    freezeNew,
				}
				newBalanceSnaps = append(newBalanceSnaps, balSnap)
				updateBalances[balSnap.Owner] = &model.Balance{
					Free:   balSnap.FreeNew,
					Freeze: balSnap.FreezeNew,
				}
			}
		}

		latestLogID = int(ol.LogID)
	}

	// ----- If there are no new balance snapshots, skip it
	if len(newBalanceSnaps) == 0 {
		logger.Debugf("ParseAndWriteLogs skip because no newBalanceSnaps with latestLogID:%d, saveLogID:%d", latestLogID, w.SavedLogID)
		return
	}

	// update balances
	updateBalanceValues := make([]any, 0)
	updateBalanceValues2 := make([]any, 0)
	ids := make([]any, 0)

	for uid, bal := range updateBalances {
		ids = append(ids, uid)
		updateBalanceValues = append(updateBalanceValues, uid, bal.Free)
		updateBalanceValues2 = append(updateBalanceValues2, uid, bal.Freeze)
	}

	updateBalanceValues = append(updateBalanceValues, updateBalanceValues2...)
	updateBalanceValues = append(updateBalanceValues, ids...)
	updateBalanceValues = append(updateBalanceValues, strings.ToLower(w.Coin))

	sql4 := strings.Repeat("?,", len(ids))
	if len(sql4) > 0 {
		sql4 = sql4[:len(sql4)-1]
	}

	sql := "UPDATE `balances` " +
		"SET " +
		"`free` = CASE `owner`\n" +
		strings.Repeat("WHEN ? THEN ?\n", len(ids)) +
		"ELSE `free`\n" +
		"END,\n" +
		"`freeze` = CASE `owner`\n" +
		strings.Repeat("WHEN ? THEN ?\n", len(ids)) +
		"ELSE `freeze`\n" +
		"END\n" +
		"WHERE `owner` IN (" + sql4 + ") and `coin`=?;"

	db := model.GetMySQLSlience()
	err = db.Transaction(func(tx *gorm.DB) (err error) {
		// upsert lastkv
		if latestMsgSeq > 0 {
			err = tx.Model(model.Lastkv{}).Where(model.Lastkv{
				App: strings.ToLower(w.Name),
				Key: model.LASTKV_K_NATS_SEQ,
			}).
				Updates(&model.Lastkv{
					Val: latestMsgSeq,
				}).
				Error
			if err != nil {
				return
			}
		}

		// create balanceSnaps
		if len(newBalanceSnaps) > 0 {
			err = tx.Scopes(model.BalanceSnapTable(w.Coin)).CreateInBatches(newBalanceSnaps, len(newBalanceSnaps)).Error
			if err != nil {
				return
			}
		}

		// create tickets
		for symbol, newTickets := range newTicketsMap {
			if len(newTickets) > 0 {
				side := w.GetSide(symbol)
				err = tx.Scopes(model.TicketTable(symbol, side)).CreateInBatches(newTickets, len(newTickets)).Error
				if err != nil {
					return
				}
			}
		}

		// update balances
		if len(ids) > 0 && len(updateBalanceValues) > 0 {
			err = tx.Exec(sql, updateBalanceValues...).Error
			if err != nil {
				return
			}
		}

		return nil
	})

	if int64(latestLogID) > w.SavedLogID {
		w.SavedLogID = int64(latestLogID)
	}

	return
}
