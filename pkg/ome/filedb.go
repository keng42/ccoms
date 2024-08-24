package ome

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
	_, err = w.CheckoutLastKv("", model.LASTKV_K_SAVED_LOG_ID)
	if err != nil {
		return
	}
	_, err = w.CheckoutLastKv("", model.LASTKV_K_LATEST_ORDER_ID)
	if err != nil {
		return
	}
	_, err = w.CheckoutLastKv("", model.LASTKV_K_LATEST_ASK_TICKET_ID)
	if err != nil {
		return
	}
	_, err = w.CheckoutLastKv("", model.LASTKV_K_LATEST_BID_TICKET_ID)
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

func (w *Worker) ParseAndWriteLogs(ss []string) (err error) {
	latestLogID := 0
	latestOrderID := int64(0)
	latestAskTicketID := int64(0)
	latestBidTicketID := int64(0)

	newTrades := make([]model.Trade, 0)
	newOrders := make([]model.Order, 0)
	updateOrders := make(map[int64]*model.Order)

	for _, s := range ss {
		ol := new(OmeLog)
		err = json.Unmarshal([]byte(s), ol)
		if err != nil {
			return
		}

		if ol.LogID <= w.SavedLogID {
			latestLogID = int(ol.LogID)
			continue
		}

		var logIndex int64

		if ol.MatchLogs != nil && len(ol.MatchLogs) > 0 {
			ml := ol.MatchLogs[0]
			price := IntToDecimal(ml.Price)
			quantity := IntToDecimal(ml.Quantity)
			amount := IntToDecimal(ml.Amount)
			askFee := IntToDecimal(ml.AskFee)
			bidFee := IntToDecimal(ml.BidFee)

			// create trade
			logIndex++
			trade := model.Trade{
				LogType:  1,
				LogID:    ol.LogID,
				LogIndex: logIndex,
				Price:    price,
				Quantity: quantity,
				Amount:   amount,
				Time:     ml.Time,
				AskOrder: ml.AskID,
				BidOrder: ml.BidID,
				Asker:    ml.Asker,
				Bider:    ml.Bider,
				AskFee:   askFee,
				BidFee:   bidFee,
			}
			newTrades = append(newTrades, trade)

			// updateOrders
			logIndex++
			o1 := model.Order{
				ID:       ml.AskID,
				Owner:    ml.Asker,
				Quantity: IntToDecimal(ml.AskQuantity),
				Amount:   IntToDecimal(ml.Amount),
				Trades:   1,
			}
			logIndex++
			o2 := model.Order{
				ID:       ml.BidID,
				Owner:    ml.Bider,
				Quantity: IntToDecimal(ml.BidQuantity),
				Amount:   IntToDecimal(ml.Amount),
				Trades:   1,
			}
			_, ok := updateOrders[ml.AskID]
			if !ok {
				updateOrders[ml.AskID] = &o1
			} else {
				updateOrders[ml.AskID].Quantity = o1.Quantity
				updateOrders[ml.AskID].Amount = o1.Amount
				updateOrders[ml.AskID].Trades = updateOrders[ml.AskID].Trades + 1
			}
			_, ok = updateOrders[ml.BidID]
			if !ok {
				updateOrders[ml.BidID] = &o2
			} else {
				updateOrders[ml.BidID].Quantity = o2.Quantity
				updateOrders[ml.BidID].Amount = o2.Amount
				updateOrders[ml.BidID].Trades = updateOrders[ml.BidID].Trades + 1
			}
		}

		if ol.OrderLogs != nil && len(ol.OrderLogs) > 0 {
			ml := ol.OrderLogs[0]
			price := IntToDecimal(ml.Price)
			quantity := IntToDecimal(ml.Quantity)

			// create order
			logIndex++
			order := model.Order{
				ID:       ml.ID,
				LogType:  1,
				LogID:    ol.LogID,
				LogIndex: logIndex,
				Price:    price,
				Quantity: quantity,
				OrigQty:  quantity,
				Amount:   quantity, // TODO
				Time:     ml.Time,
				TicketID: ml.TicketID,
				Owner:    ml.Owner,
				Side:     ml.Side,
				Type:     ml.Type,
				FeeLevel: float64(ml.FeeRate),
			}
			newOrders = append(newOrders, order)
			latestOrderID = order.ID
			if order.Side == model.OrderSideAsk {
				latestAskTicketID = order.TicketID
			} else {
				latestBidTicketID = order.TicketID
			}
		}

		latestLogID = int(ol.LogID)
	}

	if len(newTrades) == 0 && len(newOrders) == 0 && len(updateOrders) == 0 {
		logger.Tracef("ParseAndWriteLogs skip because no newTrades/newOrders with latestLogID:%d, saveLogID:%d", latestLogID, w.SavedLogID)
		return
	}

	// prepare sqls
	delOrders := make([]int64, 0)
	updateOrderValues := make([]interface{}, 0)
	updateOrderValues2 := make([]interface{}, 0)
	updateOrderValues3 := make([]interface{}, 0)
	ids := make([]interface{}, 0)

	_sql1 := "WHEN ? THEN ?\n"
	_sql2 := "WHEN ? THEN ?\n"
	_sql3 := "WHEN ? THEN `trades` + ?\n"
	_sql4 := "(?,?)"
	_sqlCount := 0

	for oid, o := range updateOrders {
		// NOTE quantity needs to be updated even when it becomes 0, because delOrders here only means status=-1.
		// As for why the data is not actually deleted but the status is modified, it is probably to get data like logID.
		// However, it can be replaced with lastkv, so it can be changed back to actually delete the data here?
		_sqlCount++
		ids = append(ids, oid)
		updateOrderValues = append(updateOrderValues, oid, o.Quantity)
		updateOrderValues2 = append(updateOrderValues2, oid, o.Amount)
		updateOrderValues3 = append(updateOrderValues3, oid, o.Trades)
		if o.Quantity.Equal(decimal.Zero) {
			delOrders = append(delOrders, oid)
		}

		// if !o.Quantity.Equal(decimal.Zero) {
		// 	_sqlCount++
		// 	ids = append(ids, oid)
		// 	updateOrderValues = append(updateOrderValues, oid, o.Quantity)
		// 	updateOrderValues2 = append(updateOrderValues2, oid, o.Amount)
		// 	updateOrderValues3 = append(updateOrderValues3, oid, o.Trades)
		// } else {
		// 	delOrders = append(delOrders, oid)
		// 	// deletes
		// }
	}

	updateOrderValues = append(updateOrderValues, updateOrderValues2...)
	updateOrderValues = append(updateOrderValues, updateOrderValues3...)
	updateOrderValues = append(updateOrderValues, ids...)

	_sql4 = strings.Repeat("?,", len(ids))
	if len(_sql4) > 0 {
		_sql4 = _sql4[:len(_sql4)-1]
	}

	_sql := "UPDATE `" + strings.ToLower(w.Symbol) + "_orders` " +
		"SET " +
		"`quantity` = CASE id\n" +
		strings.Repeat(_sql1, _sqlCount) +
		"ELSE `quantity`\n" +
		"END,\n" +
		"`amount` = CASE id\n" +
		strings.Repeat(_sql2, _sqlCount) +
		"ELSE `amount`\n" +
		"END,\n" +
		"`trades` = CASE id\n" +
		strings.Repeat(_sql3, _sqlCount) +
		"ELSE `trades`\n" +
		"END\n" +
		"WHERE `id` IN (" + _sql4 + ");"

	// write to mysql
	db := model.GetMySQLSlience()
	err = db.Transaction(func(tx *gorm.DB) (err error) {
		if len(newOrders) > 0 {
			err = tx.Scopes(model.OrderTable(w.Symbol)).CreateInBatches(newOrders, len(newOrders)).Error
			if err != nil {
				return
			}
		}

		if len(newTrades) > 0 {
			err = tx.Scopes(model.TradeTable(w.Symbol)).CreateInBatches(newTrades, len(newTrades)).Error
			if err != nil {
				return
			}
		}

		if _sqlCount > 0 && len(updateOrderValues) > 0 {
			err = tx.Exec(_sql, updateOrderValues...).Error
			if err != nil {
				return
			}
		}

		if len(delOrders) > 0 {
			err = tx.Scopes(model.OrderTable(w.Symbol)).
				Where("`id` in (?)", delOrders).Limit(len(delOrders)).
				Update("status", model.OrderStatusDeleted).Error
			// err = tx.Scopes(model.OrderTable(w.Symbol)).Delete(new([]model.Order), delOrders).Error
			if err != nil {
				return
			}
		}

		err = tx.Model(model.Lastkv{}).
			Where("`app`=? and `key`=? and `val`<?", strings.ToLower(w.Name), model.LASTKV_K_SAVED_LOG_ID, latestLogID).
			Limit(1).Update("`val`", latestLogID).Error
		if err != nil {
			return
		}

		if latestOrderID > 0 {
			err = tx.Model(model.Lastkv{}).
				Where("`app`=? and `key`=? and `val`<?", strings.ToLower(w.Name), model.LASTKV_K_LATEST_ORDER_ID, latestOrderID).
				Limit(1).Update("`val`", latestOrderID).Error
			if err != nil {
				return
			}
		}
		if latestAskTicketID > 0 {
			err = tx.Model(model.Lastkv{}).
				Where("`app`=? and `key`=? and `val`<?", strings.ToLower(w.Name), model.LASTKV_K_LATEST_ASK_TICKET_ID, latestAskTicketID).
				Limit(1).Update("`val`", latestAskTicketID).Error
			if err != nil {
				return
			}
		}
		if latestBidTicketID > 0 {
			err = tx.Model(model.Lastkv{}).
				Where("`app`=? and `key`=? and `val`<?", strings.ToLower(w.Name), model.LASTKV_K_LATEST_BID_TICKET_ID, latestBidTicketID).
				Limit(1).Update("`val`", latestBidTicketID).Error
			if err != nil {
				return
			}
		}

		return nil
	})
	if err != nil {
		return
	}

	w.SavedLogID = int64(latestLogID)

	return
}
