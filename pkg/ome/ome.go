// Package ome matching engine, get orders to be matched from bank/order and try to match them
//  1. Subscribe to new order information from the bank via grpc
//  2. Put the order into the list and try to match
package ome

import (
	"ccoms/pkg/config"
	"ccoms/pkg/xlog"
	"path"

	"encoding/json"
	"errors"
	"math/big"
	"strings"
	"time"

	"ccoms/pkg/filedb"
	"ccoms/pkg/model"
	"ccoms/pkg/xgrpc"

	"github.com/google/btree"
	"github.com/nats-io/nats.go"
	"github.com/shopspring/decimal"
	"gorm.io/gorm/clause"
)

// Worker matching engine worker class
type Worker struct {
	Nats nats.JetStreamContext

	Asks *btree.BTree
	Bids *btree.BTree
	fdb  *filedb.Filedb

	Name        string
	Symbol      string
	BaseAsset   string
	QuoteAsset  string
	TablePrefix string
	State       string

	LatestAskTicketID int64
	LatestBidTicketID int64

	LogID       int64 // auto-increment log ID
	OrderID     int64 // the order ID maintained by ome itself for this trading pair
	SavedLogID  int64 // processed (written to mysql) logID
	ToBankLogID int64 // processed (sent to bank) logID

	ch chan *xgrpc.Ticket
}

var logger = xlog.GetLogger()

// New returns a Worker instance and initializes some data
func New(symbol string) (w *Worker, err error) {
	symbol = strings.ToUpper(symbol)
	ss := strings.Split(symbol, "_")
	if len(ss) != 2 || ss[0] == "" || ss[1] == "" {
		err = errors.New("invalid symbol")
		return
	}

	asks := btree.New(2)
	bids := btree.New(2)

	w = &Worker{
		Asks: asks,
		Bids: bids,

		Name:        "OME_" + symbol,
		Symbol:      symbol,
		BaseAsset:   ss[0],
		QuoteAsset:  ss[1],
		TablePrefix: strings.ToLower(symbol),

		State: "Init",

		ch: make(chan *xgrpc.Ticket, 1024),
	}

	// open filedb
	_, err = w.Filedb()
	if err != nil {
		return
	}

	logger.Info("ome worker created")

	return
}

// Run starts the ome process
//
//	a. Main thread: use `chan OmeMsg` to receive requests from the bank, complete requests (order, trade) sequentially in a single thread
//	a1. writer processes all previous filedb logs
//	a2. cache existing Orders (read through mysql)
//	a3. cache LatestAskTicketID, LatestBidTicketID, to filter out duplicate tickets
//	a4. preparation is complete, start other worker threads
//
//	b. grpccli thread: connect to two bank service servers, mainly two functions: receive tickets pushed by banks, push balanceChange to the bank
//	b1. PullTickets: send LatestTicketID, get subsequent updates, forward to the main thread for processing via chan
//	b2. PushBalanceChanges: receive OmeReasonID from the bank, read filedb, push balanceChange from this id, and monitor in real-time
//	quote coin and base coin each have two subtasks, a total of 4 subtasks
//
//	c. writer thread: read filedb logs and batch write to mysql
//	c1. This thread is started during the preparation phase of the main thread task, and it monitors filedb updates in real-time and writes to mysql
//	It can be a separate process because the a1 task is judged to be completed based on filedb lastLogID and mysql lastLogID, so it can be independent
func (w *Worker) Run() (err error) {
	go w.StartWriter()
	go w.StartBanker(w.BaseAsset)
	go w.StartBanker(w.QuoteAsset)

	w.State = "WaitForFiledb"
	// wait for mysql.lastLogID == w.LogID(last logID in filedb)
	err = w.WaitForFiledb()
	if err != nil {
		return
	}

	w.State = "LoadingOrders"
	// load Orders, LatestAskTicketID, LatestBidTicketID from mysql
	err = w.LoadAllOrders()
	if err != nil {
		return
	}

	w.State = "Matching"
	// start matching
	err = w.StartMatching()
	return
}

// StartMatching main task: matching
func (w *Worker) StartMatching() (err error) {
	logger.Infof("StartMatching started")
	defer func() {
		if err != nil {
			logger.Errorf("StartMatching failed with err:%s", err)
		} else {
			logger.Infof("StartMatching finished")
		}
	}()

	_, err = w.TryMatch(NewOrder{})
	if err != nil {
		logger.Errorf("first TryMatch in Start failed with err:%s", err)
		return
	}
	logger.Info("first TryMatch in Start done")

	go w.StartPullTickets(w.BaseAsset)
	go w.StartPullTickets(w.QuoteAsset)

	for {
		ticket, ok := <-w.ch
		if !ok {
			return
		}
		err = w.TicketToMatchEngine(ticket)
		if err != nil {
			// TODO
			// if err == "order id is not continuous"
			// then should reload latest order or just send a new grpc req with current latest order
			return
		}
	}
}

// StartWriter write data from filedb to mysql
func (w *Worker) StartWriter() (err error) {
	round := 0
	for {
		round++
		logger.Infof("StartWriter round:%d started", round)
		err = w.FiledbToMySQL()
		if err != nil {
			logger.Errorf("StartWriter round:%d failed with err:%s", round, err)
		} else {
			logger.Infof("StartWriter round:%d done", round)
		}
		time.Sleep(time.Second)
	}
}

// StartBanker push balance changes from filedb to the corresponding bank
func (w *Worker) StartBanker(coin string) (err error) {
	round := 0
	for {
		round++
		logger.Infof("StartBanker coin(%s), round:%d started", coin, round)
		err = w.PushBalanceChanges(coin)
		if err != nil {
			logger.Errorf("StartBanker coin(%s), round:%d failed with err:%s", coin, round, err)
		} else {
			logger.Infof("StartBanker coin(%s), round:%d done", coin, round)
		}
		time.Sleep(time.Second)
	}
}

// StartPullTickets pull tickets from the bank
func (w *Worker) StartPullTickets(coin string) (err error) {
	round := 0
	for {
		round++
		logger.Infof("StartPullTickets coin(%s), round:%d started", coin, round)
		err = w.PullTickets(coin, w.ch)
		if err != nil {
			logger.Errorf("StartPullTickets coin(%s), round:%d failed with err:%s", coin, round, err)
		} else {
			logger.Infof("StartPullTickets coin(%s), round:%d done", coin, round)
		}
		time.Sleep(time.Second)
	}
}

// LoadAllOrders load all pending orders
//
//	need to ensure that the previous filedb has been written to mysql before continuing
func (w *Worker) LoadAllOrders() (err error) {
	defer func() {
		if err != nil {
			logger.Errorf("LoadAllOrders failed with err:%s", err)
		} else {
			logger.Infof("LoadAllOrders done with askTicketID:%d, bidTicketID:%d, orderID:%d",
				w.LatestAskTicketID, w.LatestBidTicketID, w.OrderID)
		}
	}()

	// load from mysql
	db := model.GetMySQL()

	var orders []model.Order
	err = db.Scopes(model.OrderTable(w.TablePrefix)).
		Where("`status`>?", model.OrderStatusDeleted).
		Order("id asc").Find(&orders).Error
	if err != nil {
		return
	}

	// build btree
	for _, order := range orders {
		o := Order{
			ID:       order.ID,
			TicketID: order.TicketID,
			Owner:    order.Owner,
			FeeRate:  int64(order.FeeLevel * 10000),
			Price:    DecimalToInt(order.Price),
			Quantity: DecimalToInt(order.Quantity),
		}

		// TODO handle delete orders issue
		if order.Side == model.OrderSideAsk {
			w.Asks.ReplaceOrInsert(AskOrder(o))
			// if o.TicketID > w.LatestAskTicketID {
			// 	w.LatestAskTicketID = o.TicketID
			// }
		} else if order.Side == model.OrderSideBid {
			w.Bids.ReplaceOrInsert(BidOrder(o))
			// if o.TicketID > w.LatestBidTicketID {
			// 	w.LatestBidTicketID = o.TicketID
			// }
		} else {
			return errors.New("invalid order side")
		}
	}

	logger.Infof("loaded asks:%d, bids:%d", w.Asks.Len(), w.Bids.Len())

	// cache latest order
	// NOTE: cannot get it this way because some orders have been deleted
	// w.OrderID = 0
	// if len(orders) > 0 {
	// 	o := orders[len(orders)-1]
	// 	w.OrderID = o.ID
	// }
	// need to ensure that the previous filedb has been processed
	// w.OrderID, err = w.LoadLatestOrderID()
	// if err != nil {
	// 	return
	// }

	// need to ensure that the previous filedb has been processed, and this LoadAllOrders is completed without errors before adding filedb content
	var lastkvs []model.Lastkv
	err = db.Model(model.Lastkv{}).Where("`app`=?", strings.ToLower(w.Name)).Find(&lastkvs).Error
	if err != nil {
		return
	}

	for _, item := range lastkvs {
		switch item.Key {
		case model.LASTKV_K_LATEST_ORDER_ID:
			w.OrderID = item.Val
		case model.LASTKV_K_LATEST_ASK_TICKET_ID:
			w.LatestAskTicketID = item.Val
		case model.LASTKV_K_LATEST_BID_TICKET_ID:
			w.LatestBidTicketID = item.Val
		}
	}

	return
}

// LoadLatestTicketID read ticket id from mysql
func (w *Worker) LoadLatestTicketID() (id int64, err error) {
	defer func() {
		if err != nil {
			logger.Errorf("LoadLatestTicketID failed with err:%s", err)
		} else {
			logger.Infof("LoadLatestTicketID done with id:%d", id)
		}
	}()

	db := model.GetMySQL()

	var lastkv model.Lastkv
	err = db.Model(model.Lastkv{}).
		Where("`app`=? and `key`=?", strings.ToLower(w.Name), model.LASTKV_K_LATEST_ASK_TICKET_ID).
		Limit(1).Find(&lastkv).Error
	if err != nil {
		return
	}

	id = int64(lastkv.Val)
	return
}

// LoadLatestOrderID read order id from mysql
func (w *Worker) LoadLatestOrderID() (id int64, err error) {
	defer func() {
		if err != nil {
			logger.Errorf("LoadLatestOrderID failed with err:%s", err)
		} else {
			logger.Infof("LoadLatestOrderID done with id:%d", id)
		}
	}()

	db := model.GetMySQL()

	var lastkv model.Lastkv
	err = db.Model(model.Lastkv{}).
		Where("`app`=? and `key`=?", strings.ToLower(w.Name), model.LASTKV_K_LATEST_ORDER_ID).
		Limit(1).Find(&lastkv).Error
	if err != nil {
		return
	}

	id = int64(lastkv.Val)
	return
}

// LoadLatestOrderIDFromTables get the latest order id from multiple tables
//
// considering the existence of delete order requests, lastkv should be used to read
func (w *Worker) LoadLatestOrderIDFromTables() (id int64, err error) {
	defer func() {
		if err != nil {
			logger.Errorf("LoadLatestOrderID failed with err:%s", err)
		} else {
			logger.Infof("LoadLatestOrderID done with id:%d", id)
		}
	}()

	db := model.GetMySQL()

	var order model.Order
	err = db.Scopes(model.OrderTable(w.TablePrefix)).Order("id desc").Limit(1).Find(&order).Error
	if err != nil {
		return
	}

	var tradeAsk model.Trade
	err = db.Scopes(model.TradeTable(w.TablePrefix)).Order("ask_order desc").Limit(1).Find(&tradeAsk).Error
	if err != nil {
		return
	}

	var tradeBid model.Trade
	err = db.Scopes(model.TradeTable(w.TablePrefix)).Order("bid_order desc").Limit(1).Find(&tradeBid).Error
	if err != nil {
		return
	}

	id = max(order.ID, tradeAsk.AskOrder, tradeAsk.BidOrder)
	return
}

// LoadSavedLogID read logID from mysql
func (w *Worker) LoadSavedLogID() (id int64, err error) {
	defer func() {
		if err != nil {
			logger.Errorf("LoadSavedLogID failed with err:%s", err)
		} else {
			logger.Infof("LoadSavedLogID done with id:%d", id)
		}
	}()

	db := model.GetMySQL()

	var lastkv model.Lastkv
	err = db.Model(model.Lastkv{}).
		Where("`app`=? and `key`=?", strings.ToLower(w.Name), model.LASTKV_K_SAVED_LOG_ID).
		Limit(1).Find(&lastkv).Error
	if err != nil {
		return
	}

	id = int64(lastkv.Val)
	return
}

// WaitForFiledb wait for filedb to complete initialization before the service starts
//
//	read the logID of the latest record to ensure that the previous logs have all been written to mysql, i.e., savedLogID >= logID
//	therefore, FiledbToMySQL should be called at the same time or before calling this method
func (w *Worker) WaitForFiledb() (err error) {
	logger.Infof("WaitForFiledb started")
	defer func() {
		if err != nil {
			logger.Errorf("WaitForFiledb failed with err:%s", err)
		} else {
			logger.Infof("WaitForFiledb finished")
		}
	}()

	s, err := w.fdb.ReadLastLine()
	if err != nil {
		return
	}
	if s == "" {
		return nil
	}

	var ml OmeLog
	err = json.Unmarshal([]byte(s), &ml)
	if err != nil {
		return
	}

	w.LogID = ml.LogID

	for {
		// load w.SavedLogID from mysql
		savedLogID, _ := w.LoadSavedLogID()
		if w.SavedLogID >= ml.LogID {
			logger.Infof("WaitForFiledb done with savedLogID:%d, logID:%d", savedLogID, ml.LogID)
			return
		}
		ts := time.Second
		logger.Infof("WaitForFiledb sleep:%s with savedLogID:%d, logID:%d", ts, savedLogID, ml.LogID)
		time.Sleep(ts)
	}
}

// LoadLatestMatchLog load the latest log
func (w *Worker) LoadLatestMatchLog() (ml MatchLog, err error) {
	fdb, err := w.Filedb()
	if err != nil {
		return
	}

	s, err := fdb.ReadLastLine()
	if err != nil {
		return
	}

	if s == "" {
		return
	}

	err = json.Unmarshal([]byte(s), &ml)
	return
}

// TicketToMatchEngine process the newly received ticket
//
//	create order, try to match
func (w *Worker) TicketToMatchEngine(ticket *xgrpc.Ticket) (err error) {
	side := int8(ticket.Side)
	if side == model.OrderSideAsk {
		if ticket.Id <= w.LatestAskTicketID {
			// duplicate push, return directly
			return
		}
		if ticket.Id != w.LatestAskTicketID+1 {
			err = errors.New("ticket id is not continuous")
			logger.Errorf("TicketToMatchEngine failed with ticket.id:%d, LatestAskTicketID:%d, err:%s", ticket.Id, w.LatestAskTicketID, err)
			return
		}
	} else if side == model.OrderSideBid {
		if ticket.Id <= w.LatestBidTicketID {
			// duplicate push, return directly
			return
		}
		if ticket.Id != w.LatestBidTicketID+1 {
			err = errors.New("ticket id is not continuous")
			logger.Errorf("TicketToMatchEngine failed with ticket.id:%d, LatestBidTicketID:%d, err:%s", ticket.Id, w.LatestBidTicketID, err)
			return
		}
	} else {
		err = errors.New("invalid order side")
		return
	}

	p, err := decimal.NewFromString(ticket.Price)
	if err != nil {
		return
	}
	q, err := decimal.NewFromString(ticket.Quantity)
	if err != nil {
		return
	}

	w.OrderID++
	w.LogID++
	defer func() {
		if err != nil {
			w.OrderID--
			w.LogID--
		}
	}()
	now := time.Now().Unix()
	o := NewOrder{
		ID:       w.OrderID,
		TicketID: ticket.Id,
		Owner:    ticket.Owner,
		FeeRate:  ticket.FeeRate,
		Time:     now,
		Side:     side,
		Type:     int8(ticket.Type),
		Price:    DecimalToInt(p),
		Quantity: DecimalToInt(q),
	}

	// write new order to filedb
	ol := OrderLog{
		LogIndex: 0,

		ID:       o.ID,
		TicketID: o.TicketID,
		Owner:    o.Owner,
		FeeRate:  o.FeeRate,
		Time:     o.Time,
		Side:     o.Side,
		Type:     o.Type,
		Price:    o.Price,
		Quantity: o.Quantity,
	}

	omeLog := OmeLog{
		LogID: w.LogID,
		Ts:    time.Now().UnixNano(),

		OrderLogs: []OrderLog{ol},
	}

	mlb, _ := json.Marshal(omeLog)

	// write to filedb
	f, err := w.Filedb()
	if err != nil {
		return
	}
	err = f.WriteLine(string(mlb) + "\n")
	if err != nil {
		return
	}

	if side == model.OrderSideAsk {
		err = w.NewAsk(o)
		if err != nil {
			return
		}
		w.LatestAskTicketID = ticket.Id
	} else if side == model.OrderSideBid {
		err = w.NewBid(o)
		if err != nil {
			return
		}
		w.LatestBidTicketID = ticket.Id
	} else {
		err = errors.New("invalid order side")
		return
	}

	return
}

// NewAsk put the new ask order into the list
func (w *Worker) NewAsk(no NewOrder) (err error) {
	o := Order{
		ID:       no.ID,
		TicketID: no.TicketID,
		Owner:    no.Owner,
		FeeRate:  no.FeeRate,
		Price:    no.Price,
		Quantity: no.Quantity,
	}

	if w.Asks.Has(AskOrder(o)) {
		return errors.New("order exists")
	}

	w.Asks.ReplaceOrInsert(AskOrder(o))

	_, err = w.TryMatch(no)

	return
}

// NewBid put the new bid order into the list
func (w *Worker) NewBid(no NewOrder) (err error) {
	o := Order{
		ID:       no.ID,
		TicketID: no.TicketID,
		Owner:    no.Owner,
		FeeRate:  no.FeeRate,
		Price:    no.Price,
		Quantity: no.Quantity,
	}

	if w.Bids.Has(BidOrder(o)) {
		return errors.New("order exists")
	}

	w.Bids.ReplaceOrInsert(BidOrder(o))

	_, err = w.TryMatch(no)

	return
}

// TryMatch try to match, recursively call until it cannot continue to match
func (w *Worker) TryMatch(no NewOrder) (bool, error) {
	ask := w.Asks.Min()
	bid := w.Bids.Max()

	if ask == nil || bid == nil {
		return true, nil
	}

	oa, _ := ask.(AskOrder)
	ob, _ := bid.(BidOrder)

	if Greater(oa.Price, ob.Price) {
		return true, nil
	}

	price := ob.Price
	if oa.ID < ob.ID {
		price = oa.Price
	}
	quantity := ob.Quantity
	if Less(oa.Quantity, ob.Quantity) {
		quantity = oa.Quantity
	}
	amount := big.NewInt(0).Mul(price, quantity)
	amount.Div(amount, ExpInt)

	newAsk := AskOrder(oa)
	newAsk.Price = oa.Price
	newAsk.Quantity = big.NewInt(0).Sub(oa.Quantity, quantity)
	newBid := BidOrder(ob)
	newBid.Price = ob.Price
	newBid.Quantity = big.NewInt(0).Sub(ob.Quantity, quantity)

	// TODO
	askFee := big.NewInt(0)
	bidFee := big.NewInt(0)

	if IsZero(newAsk.Quantity) {
		w.Asks.Delete(AskOrder{ID: oa.ID, Price: oa.Price})
	} else {
		w.Asks.ReplaceOrInsert(newAsk)
	}

	if IsZero(newBid.Quantity) {
		w.Bids.Delete(BidOrder{ID: ob.ID, Price: ob.Price})
	} else {
		w.Bids.ReplaceOrInsert(newBid)
	}

	w.LogID++
	ml := MatchLog{
		// LogID:    w.LogID,
		LogIndex: 0,

		Asker:       oa.Owner,
		AskID:       oa.ID,
		AskPrice:    newAsk.Price,
		AskQuantity: newAsk.Quantity,

		Bider:       ob.Owner,
		BidID:       ob.ID,
		BidPrice:    newBid.Price,
		BidQuantity: newBid.Quantity,

		Price:    price,
		Quantity: quantity,
		Amount:   amount,
		AskFee:   askFee,
		BidFee:   bidFee,

		Time: time.Now().Unix(),
	}

	omeLog := OmeLog{
		LogID: w.LogID,
		Ts:    time.Now().UnixNano(),

		MatchLogs: []MatchLog{ml},
	}

	mlb, _ := json.Marshal(omeLog)

	// write to filedb
	f, err := w.Filedb()
	if err != nil {
		return false, err
	}
	err = f.WriteLine(string(mlb) + "\n")
	// lastFiledbedTime = time.Now()
	// filedbedLines += 1
	if err != nil {
		return false, err
	}

	if IsZero(newAsk.Quantity) || IsZero(newBid.Quantity) {
		return w.TryMatch(NewOrder{})
	}

	return true, nil
}

func (w *Worker) CheckoutLastKv(app, key string) (kv model.Lastkv, err error) {
	if app == "" {
		app = strings.ToLower(w.Name)
	}

	db := model.GetMySQL()

	kv = model.Lastkv{
		App: app,
		Key: key,
	}
	err = db.Model(model.Lastkv{}).Where(kv).Limit(1).Find(&kv).Error
	if err != nil {
		return
	}
	if kv.ID > 0 {
		return
	}

	err = db.Model(model.Lastkv{}).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "app"}, {Name: "key"}},
			DoNothing: true,
		}).
		Create(&kv).Error
	if err != nil {
		return
	}

	return
}

// Filedb returns the current working filedb instance
// TODO: according to the current file splitting method, a new instance should be returned when the time comes
func (w *Worker) Filedb() (*filedb.Filedb, error) {
	if w.fdb != nil {
		return w.fdb, nil
	}

	// fdb, err := filedb.New(config.DEVDATA + "/filedb/log-ome.txt")
	fdb, err := filedb.New(path.Join(config.Shared.DataDir, "filedb", strings.ToLower(w.Name)+".log"))
	if err != nil {
		return nil, err
	}

	fdb.ToMySQLHandler = w.ParseAndWriteLogs

	w.fdb = fdb

	return w.fdb, nil
}
