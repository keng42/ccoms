package bank

import (
	"ccoms/pkg/config"
	"ccoms/pkg/filedb"
	"ccoms/pkg/model"
	"ccoms/pkg/xgrpc"
	"ccoms/pkg/xlog"
	"ccoms/pkg/xnats"
	"encoding/json"
	"errors"
	"path"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/shopspring/decimal"
	"gorm.io/gorm/clause"
)

// Worker is the bank system
type Worker struct {
	Name    string   // e.g. Bank_USDT
	Coin    string   // e.g. USDT
	Symbols []string // e.g. BTC_USDT
	State   string

	LogID int64 // ID of the latest log

	// Auto-incrementing ticket ID, one bank supports multiple symbols, so we need to use a map here
	// TicketID int64
	TicketIDs map[string]int64

	Assets map[int64]*UserAsset // userid -> coin balance

	ch           chan BankMsg     // Other worker threads send requests (OrderReq, BalanceChange) to the main thread for processing via this chan
	OmeReasonIDs map[string]int64 // symbol -> reasonID, the latest BalanceChanges received from each ome, the ID of the latest one
	LatestMsgSeq uint64           // ID of the latest NATS message received
	SavedLogID   int64            // ID of the log already processed (written to MySQL)

	fdb *filedb.Filedb
}

var logger = xlog.GetLogger()

// New returns a Worker instance and completes some preparatory work before the worker starts working
func New(coin string) (w *Worker, err error) {
	coin = strings.ToUpper(coin)
	w = &Worker{
		Name:    "Bank_" + coin,
		Coin:    coin,
		Symbols: []string{"BTC_USDT"}, // TODO

		// LogID: load from filedb

		TicketIDs: map[string]int64{},

		Assets: map[int64]*UserAsset{},

		ch:           make(chan BankMsg, 1024),
		OmeReasonIDs: map[string]int64{},
		// LatestMsgSeq: load from filedb

		// fdb: -

		State: "Init",
	}

	// Open filedb
	_, err = w.Filedb()
	if err != nil {
		return nil, err
	}

	// Read the last logID from filedb
	txt, err := w.fdb.ReadLastLine()
	if err != nil {
		return nil, err
	}
	if txt != "" {
		bl := BankLog{}
		err = json.Unmarshal([]byte(txt), &bl)
		if err != nil {
			// TODO
			// One possible error is that the log was not written completely
			// Are there any other errors?
			return nil, err
		}
		w.LogID = bl.LogID
		w.LatestMsgSeq = bl.MsgSeq
	}

	logger.Info("bank worker created")
	return
}

// Run starts the service
//
//	a. Main thread: Use `chan BankMsg` to receive requests from ingress and ome, process requests sequentially in a single thread (create ticket, update balance in memory, write to filedb)
//	a1. Writer handles all filedb logs before the main thread
//	a2. Cache existing OmeReasonIDs, LatestMsgSeq, TicketID, LogID (this is read from MySQL or filedb?)
//	a3. Cache existing Assets (read from MySQL)
//	a4. Preparation complete, start other worker threads
//
//	b. natscli thread: Connect to NATS service, subscribe to messages from ingress, and forward them to the main thread via chan for processing
//	b1. Get LatestMsgSeq and start getting subsequent updates
//
//	c. grpcsrv thread: Start bank service server, mainly two functions: push tickets to ome, receive balanceChange pushed from ome
//	c1. Directly start grpc server, wait for ome to initiate requests
//	c2. Tickets: Push subsequent tickets to ome based on the ID in the request parameters, monitor filedb in real-time
//	c3. BalanceChanges: When first requested, send OmeReasonID to ome, ome will then push subsequent balance change requests
//
//	d. writer thread: Read filedb logs, write to MySQL in batches
//	d1. This thread is started immediately after the main thread task preparation, monitor filedb updates in real-time, and write to MySQL
//	Can run as a separate process because the main thread task completion judgment is based on the lastLogID in filedb and the lastLogID in MySQL, so it can run independently
func (w *Worker) Run() (err error) {

	go w.StartWriter()

	// wait for mysql.lastLogID == w.LogID(last logID in filedb)
	w.State = "WaitForFiledb"

	err = w.WaitForFiledb()
	if err != nil {
		return
	}

	w.State = "LoadingAssets"
	// load Assets, ticketID, LatestMsgSeq (nats), OmeReasonIDs (grpc) from mysql
	err = w.LoadAllAssets()
	if err != nil {
		return
	}

	// set status=ready
	w.State = "Working"

	go w.StartSubNats()
	go w.StartServeGrpc()

	err = w.HandleBankMsgs()

	return
}

// StartWriter reads data from filedb and writes it to MySQL
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

func (w *Worker) StartSubNats() (err error) {
	round := 0
	for {
		round++
		logger.Infof("StartSubNats round:%d started", round)
		err = w.SubNats()
		if err != nil {
			logger.Errorf("StartSubNats round:%d failed with err:%s", round, err)
		} else {
			logger.Infof("StartSubNats round:%d done", round)
		}
		time.Sleep(time.Second)
	}
}

func (w *Worker) StartServeGrpc() (err error) {
	round := 0
	for {
		round++
		logger.Infof("StartServeGrpc round:%d started", round)
		err = w.ServeGrpc()
		if err != nil {
			logger.Errorf("StartServeGrpc round:%d failed with err:%s", round, err)
		} else {
			logger.Infof("StartServeGrpc round:%d done", round)
		}
		time.Sleep(time.Second)
	}
}

// WaitForFiledb waits for filedb to complete initialization before the service starts
//
//	Read the logID of the latest record to ensure that the previous logs have all been written to MySQL, i.e., savedLogID >= logID
//	Therefore, this method should be called simultaneously or before calling FiledbToMySQL
func (w *Worker) WaitForFiledb() (err error) {
	defer func() {
		if err != nil {
			logger.Errorf("WaitForFiledb failed with err:%s", err)
		}
	}()

	s, err := w.fdb.ReadLastLine()
	if err != nil {
		return
	}
	if s == "" {
		return nil
	}

	var bl BankLog
	err = json.Unmarshal([]byte(s), &bl)
	if err != nil {
		return
	}

	w.LogID = bl.LogID

	for {
		// load w.SavedLogID from mysql
		savedLogID, _ := w.LoadSavedLogID()
		if savedLogID >= bl.LogID {
			logger.Infof("WaitForFiledb done with savedLogID:%d, logID:%d", savedLogID, bl.LogID)
			return
		}
		ts := time.Second
		logger.Infof("WaitForFiledb sleep:%s with savedLogID:%d, logID:%d", ts, savedLogID, bl.LogID)
		time.Sleep(ts)
	}
}

// LoadSavedLogID reads logID from MySQL
func (w *Worker) LoadSavedLogID() (id int64, err error) {
	defer func() {
		if err != nil {
			logger.Errorf("LoadSavedLogID failed with err:%s", err)
		} else {
			logger.Infof("LoadSavedLogID done with id:%d", id)
		}
	}()

	db := model.GetMySQL()

	var lastBs model.BalanceSnap
	err = model.BalanceSnapTable(w.Coin)(db).Order("id desc").Limit(1).Find(&lastBs).Error
	if err != nil {
		logger.Errorf("LoadSavedLogID failed with err:%s", err)
		return
	}
	id = lastBs.LogID

	return
}

// LoadAllAssets loads all orders
//
//	Ensure that the previous filedb has been written to MySQL before continuing
func (w *Worker) LoadAllAssets() (err error) {
	defer func() {
		if err != nil {
			logger.Errorf("LoadAllAssets failed with err:%s", err)
		} else {
			logger.Infof("LoadAllAssets done with ticketIDs:%v, latestMsgSeq:%d, omeReasonIDs:%+v",
				w.TicketIDs, w.LatestMsgSeq, w.OmeReasonIDs)
		}
	}()

	// load from mysql
	db := model.GetMySQL()

	var balances []model.Balance
	err = db.Model(model.Balance{}).Where("`coin`=?", w.Coin).Order("id asc").Find(&balances).Error
	if err != nil {
		return
	}

	// build btree
	for _, bal := range balances {
		w.Assets[bal.Owner] = &UserAsset{
			Free:   bal.Free,
			Freeze: bal.Freeze,
		}
	}

	// Contains multiple trading pairs, need to distinguish ticketID
	var lastTicket model.Ticket
	for _, symbol := range w.Symbols {
		side := w.GetSide(symbol)
		err = db.Scopes(model.TicketTable(symbol, side)).Order("id desc").Limit(1).Find(&lastTicket).Error
		if err != nil {
			return
		}
		w.TicketIDs[symbol] = lastTicket.ID
	}

	var lastkvs []model.Lastkv
	err = db.Model(model.Lastkv{}).Where("`app`=?", strings.ToLower(w.Name)).Find(&lastkvs).Error
	if err != nil {
		return
	}

	for _, item := range lastkvs {
		if item.Key == model.LASTKV_K_NATS_SEQ {
			w.LatestMsgSeq = uint64(item.Val)
		}
		if strings.HasPrefix(item.Key, model.LASTKV_K_OME_REASONID) {
			symbol := strings.ToUpper(strings.Replace(item.Key, model.LASTKV_K_OME_REASONID, "", 1))
			w.OmeReasonIDs[symbol] = item.Val
		}
	}

	return
}

type ackPayload struct {
	msg *nats.Msg
	seq uint64
}

// HandleBankMsgs handles tasks from other worker threads (single-threaded sequentially)
func (w *Worker) HandleBankMsgs() (err error) {
	// send nats msgs through channel to this goroutine from other goroutines,
	// and ack msgs in batch here.
	chAck := make(chan ackPayload, 1024)
	// chAck2 := make(chan ackPayload)

	go func() {
		var latest ackPayload
		for {
			mp := <-chAck
			if mp.seq > latest.seq {
				latest = mp
			}
			// fetch all msgs at once
			l := len(chAck)
			if l > 0 {
				for i := 0; i < l; i++ {
					mp = <-chAck
					if mp.seq > latest.seq {
						latest = mp
					}
				}
			}
			err = latest.msg.Ack()
			if err != nil {
				logger.Errorf("msg(%v) ack failed with err:%s", mp.seq, err)
				continue
			}
			logger.Debugf("msg(%v) ack done", mp.seq)
			latest = mp
		}
	}()

	// go func() {
	// 	var latest ackPayload
	// 	for {
	// 		select {
	// 		case mp := <-chAck:
	// 			if mp.seq > latest.seq {
	// 				latest = mp
	// 			}
	// 		case chAck2 <- latest:
	// 		}
	// 	}
	// }()

	// go func() {
	// 	var latest ackPayload
	// 	for {
	// 		mp := <-chAck2
	// 		if mp.msg == nil || mp.seq <= latest.seq {
	// 			continue
	// 		}
	// 		err = mp.msg.Ack()
	// 		if err != nil {
	// 			logger.Errorf("msg(%v) ack failed with err:%s", mp.seq, err)
	// 			continue
	// 		}
	// 		logger.Infof("msg(%v) ack done", mp.seq)
	// 		latest = mp
	// 	}
	// }()

	// start handling bank msgs
	for {
		bs, ok := <-w.ch
		if !ok {
			return
		}

		// nast msg
		if bs.N != nil {
			msg := bs.N
			switch msg.Subject {
			case "BANK." + w.Coin + ".OrderReq":
				err = w.HandleOrderReq(msg, chAck)
				if err != nil {
					return
				}
			}
		}

		// grpc msg
		if bs.G != nil {
			err = w.HandleBalanceChange(bs.G)
			if err != nil {
				return
			}
		}
	}
}

func (w *Worker) HandleBalanceChange(bc *xgrpc.BalanceChange) (err error) {
	w.OmeReasonIDs[bc.ReasonTable] = bc.ReasonID
	err = w.BalanceChanged(bc)
	if err != nil {
		return
	}
	return
}

func (w *Worker) HandleOrderReq(msg *nats.Msg, chAck chan ackPayload) (err error) {
	var orderReq xnats.OrderReq
	err = json.Unmarshal(msg.Data, &orderReq)
	if err != nil {
		// TODO
		return
	}

	md, err := msg.Metadata()
	if err != nil {
		// TODO
		return
	}

	logger.Tracef("HandleOrderReq msg:%s, seq:%d", msg.Subject, md.Sequence.Stream)

	if md.Sequence.Stream <= w.LatestMsgSeq {
		// TODO
		logger.Warningf("md.Sequence.Stream(%d) <= w.LatestMsgSeq(%d)", md.Sequence.Stream, w.LatestMsgSeq)
		chAck <- ackPayload{msg: msg, seq: md.Sequence.Stream}
		return
	}

	err = w.CreateOrder(md.Sequence.Stream, orderReq)
	if err != nil {
		if errors.Is(err, ErrCreateOrderSafeSkip) {
			chAck <- ackPayload{msg: msg, seq: md.Sequence.Stream}
		}
		return
	}

	// ack
	chAck <- ackPayload{msg: msg, seq: md.Sequence.Stream}

	return
}

// Filedb returns the current working filedb instance
// TODO: According to the current file splitting method, a new instance should be returned when the time is up
func (w *Worker) Filedb() (fdb *filedb.Filedb, err error) {
	if w.fdb != nil {
		return w.fdb, nil
	}

	fdb, err = filedb.New(path.Join(config.Shared.DataDir, "filedb", strings.ToLower(w.Name)+".log"))
	if err != nil {
		return nil, err
	}

	fdb.ToMySQLHandler = w.ParseAndWriteLogs

	w.fdb = fdb
	return w.fdb, nil
}

// CheckoutAsset retrieves user asset information
//
//	If it doesn't exist, create one, not thread-safe!!!
func (w *Worker) CheckoutAsset(owner int64) *UserAsset {
	ua, ok := w.Assets[owner]
	if !ok {
		ua = new(UserAsset)
		w.Assets[owner] = ua
	}
	return ua
}

var ErrCreateOrderSafeSkip = errors.New("create order safe skip")

// - Create a buy order, deduct available money, and increase frozen money
// - Create a sell order, deduct available coins, and increase frozen coins
func (w *Worker) CreateOrder(msgSeq uint64, o xnats.OrderReq) (err error) {
	// prepare data
	ss := strings.Split(o.Symbol, "_")
	if len(ss) != 2 {
		return errors.New("invalid symbol")
	}
	base, quote := ss[0], ss[1]

	var coin string
	var value decimal.Decimal
	var side string
	if o.Side == model.OrderSideBid {
		side = "bid"
		coin = quote
		value = o.Amount
	} else if o.Side == model.OrderSideAsk {
		side = "ask"
		coin = base
		value = o.Quantity
	} else {
		return errors.New("invalid order side")
	}
	if coin != w.Coin {
		// TODO: What if ome sends a BTC order here???
		logger.Errorf("only for %s", w.Coin)
		return ErrCreateOrderSafeSkip
	}

	// Calculate fee and final fee
	feeRate := o.FeeLevel
	fee := value.Mul(decimal.NewFromFloat(feeRate))
	total := value.Add(fee)

	// // add lock
	// w.mu.Lock()
	// defer w.mu.Unlock()

	// get user's coin asset
	uaa := w.CheckoutAsset(o.Owner)

	// update data in memory
	uaa.Free = uaa.Free.Sub(total)
	uaa.Freeze = uaa.Freeze.Add(total)
	w.LogID++
	w.TicketIDs[o.Symbol]++

	defer func() {
		if err != nil {
			uaa.Free = uaa.Free.Add(total)
			uaa.Freeze = uaa.Freeze.Sub(total)
			w.LogID--
			w.TicketIDs[o.Symbol]--
		}
	}()

	// create logs
	logIndex := int64(0)

	logIndex++
	tl := TicketLog{
		LogIndex: logIndex,
		Reason:   "CreateOrder",
		ID:       w.TicketIDs[o.Symbol],
		Owner:    o.Owner,
		Symbol:   o.Symbol,
		Type:     o.Type,
		Side:     o.Side,
		Price:    o.Price.String(),
		Quantity: o.Quantity.String(),
		Amount:   o.Amount.String(),
	}

	logIndex++
	bl := BalanceLog{
		LogIndex:     logIndex,
		Reason:       "CreateOrder",
		ReasonTable:  strings.ToLower(o.Symbol) + "_" + side + "_tickets",
		ReasonID:     w.TicketIDs[o.Symbol],
		Owner:        o.Owner,
		Coin:         coin,
		FreeChange:   "-" + total.String(),
		FreezeChange: total.String(),
		FreeNew:      uaa.Free.String(),
		FreezeNew:    uaa.Freeze.String(),
	}

	bankLog := BankLog{
		LogID:  w.LogID,
		Ts:     time.Now().UnixNano(),
		MsgSeq: msgSeq,

		TicketLogs:  []TicketLog{tl},
		BalanceLogs: []BalanceLog{bl},
	}

	blb, err := json.Marshal(bankLog)
	if err != nil {
		return
	}

	// write to filedb
	_, err = w.Filedb()
	if err != nil {
		return
	}
	err = w.fdb.WriteLine(string(blb) + "\n")
	if err != nil {
		return
	}

	w.LatestMsgSeq = msgSeq

	return
}

// - Cancel buy order, increase available money, and decrease frozen money
// - Cancel sell order, increase available coins, and decrease frozen coins
func (w *Worker) CancelOrder(symbol string, o model.Order) (err error) { return }

// - Match successful, buyer: increase available coins, decrease corresponding frozen money; seller: increase available money, decrease corresponding frozen coins
func (w *Worker) OrderMatched(
	owner1 int64, freeChange1, freezeChange1 decimal.Decimal,
	owner2 int64, freeChange2, freezeChange2 decimal.Decimal,
	reasonTable string, reasonID int64,
) (err error) {

	// // add lock
	// w.mu.Lock()
	// defer w.mu.Unlock()

	// get user's coin asset
	uaa1 := w.CheckoutAsset(owner1)
	uaa2 := w.CheckoutAsset(owner2)

	// update data in memory
	uaa1.Free = uaa1.Free.Add(freeChange1)
	uaa1.Freeze = uaa1.Freeze.Add(freezeChange1)
	uaa2.Free = uaa2.Free.Add(freeChange2)
	uaa2.Freeze = uaa2.Freeze.Add(freezeChange2)
	w.LogID++

	defer func() {
		if err != nil {
			uaa1.Free = uaa1.Free.Sub(freeChange1)
			uaa1.Freeze = uaa1.Freeze.Sub(freezeChange1)
			uaa2.Free = uaa2.Free.Sub(freeChange2)
			uaa2.Freeze = uaa2.Freeze.Sub(freezeChange2)
			w.LogID--
		}
	}()

	// create logs
	logIndex := int64(0)

	logIndex++
	bl := BalanceLog{
		LogIndex:      logIndex,
		Reason:        "OrderMatched",
		ReasonTable:   reasonTable,
		ReasonID:      reasonID,
		Owner:         owner1,
		Coin:          w.Coin,
		FreeChange:    freeChange1.String(),
		FreezeChange:  freezeChange1.String(),
		FreeNew:       uaa1.Free.String(),
		FreezeNew:     uaa1.Freeze.String(),
		Owner2:        owner2,
		FreeChange2:   freeChange2.String(),
		FreezeChange2: freezeChange2.String(),
		FreeNew2:      uaa2.Free.String(),
		FreezeNew2:    uaa2.Freeze.String(),
	}

	bankLog := BankLog{
		LogID: w.LogID,
		Ts:    time.Now().UnixNano(),

		BalanceLogs: []BalanceLog{bl},
	}

	blb, err := json.Marshal(bankLog)
	if err != nil {
		return
	}

	// write to filedb
	_, err = w.Filedb()
	if err != nil {
		return
	}
	err = w.fdb.WriteLine(string(blb) + "\n")
	if err != nil {
		return
	}

	return
}

func (w *Worker) BalanceChanged(bc *xgrpc.BalanceChange) (err error) {

	// // add lock
	// w.mu.Lock()
	// defer w.mu.Unlock()

	// get user's coin asset
	uaa1 := w.CheckoutAsset(bc.Owner)
	var uaa2 *UserAsset
	if bc.Owner2 > 0 {
		uaa2 = w.CheckoutAsset(bc.Owner2)
	}

	// update data in memory
	freeChange, _ := decimal.NewFromString(bc.FreeChange)     // TODO handle error
	freezeChange, _ := decimal.NewFromString(bc.FreezeChange) // TODO handle error
	uaa1.Free = uaa1.Free.Add(freeChange)
	uaa1.Freeze = uaa1.Freeze.Add(freezeChange)

	var freeChange2, freezeChange2 decimal.Decimal
	if bc.Owner2 > 0 {
		freeChange2, _ = decimal.NewFromString(bc.FreeChange2)     // TODO handle error
		freezeChange2, _ = decimal.NewFromString(bc.FreezeChange2) // TODO handle error
		uaa2.Free = uaa2.Free.Add(freeChange2)
		uaa2.Freeze = uaa2.Freeze.Add(freezeChange2)
	}
	w.LogID++

	defer func() {
		if err != nil {
			uaa1.Free = uaa1.Free.Sub(freeChange)
			uaa1.Freeze = uaa1.Freeze.Sub(freezeChange)
			if bc.Owner2 > 0 {
				uaa2.Free = uaa2.Free.Sub(freeChange2)
				uaa2.Freeze = uaa2.Freeze.Sub(freezeChange2)
			}
			w.LogID--
		}
	}()

	// create logs
	logIndex := int64(0)

	logIndex++
	bl := BalanceLog{
		LogIndex:      logIndex,
		Reason:        bc.Reason,
		ReasonTable:   bc.ReasonTable,
		ReasonID:      bc.ReasonID,
		Owner:         bc.Owner,
		Coin:          w.Coin,
		FreeChange:    bc.FreeChange,
		FreezeChange:  bc.FreezeChange,
		FreeNew:       uaa1.Free.String(),
		FreezeNew:     uaa1.Freeze.String(),
		Owner2:        bc.Owner2,
		FreeChange2:   bc.FreeChange2,
		FreezeChange2: bc.FreezeChange2,
		FreeNew2:      uaa2.Free.String(),
		FreezeNew2:    uaa2.Freeze.String(),
	}

	bankLog := BankLog{
		LogID: w.LogID,
		Ts:    time.Now().UnixNano(),

		BalanceLogs: []BalanceLog{bl},
	}

	blb, err := json.Marshal(bankLog)
	if err != nil {
		return
	}

	// write to filedb
	_, err = w.Filedb()
	if err != nil {
		return
	}
	err = w.fdb.WriteLine(string(blb) + "\n")
	if err != nil {
		return
	}

	return
}

// - Deposit, increase available coins
// - Withdraw, decrease available coins
// - Management, increase or decrease balance
func (w *Worker) DirectChange(
	owner int64, freeChange, freezeChange decimal.Decimal,
	reasonTable string, reasonID int64,
) (err error) {

	// // add lock
	// w.mu.Lock()
	// defer w.mu.Unlock()

	// get user's coin asset
	uaa := w.CheckoutAsset(owner)

	// update data in memory
	uaa.Free = uaa.Free.Add(freeChange)
	uaa.Freeze = uaa.Freeze.Add(freezeChange)
	w.LogID++

	defer func() {
		if err != nil {
			uaa.Free = uaa.Free.Sub(freeChange)
			uaa.Freeze = uaa.Freeze.Sub(freezeChange)
			w.LogID--
		}
	}()

	// create logs
	logIndex := int64(0)

	logIndex++
	bl := BalanceLog{
		LogIndex:     logIndex,
		Reason:       "DirectChange",
		ReasonTable:  reasonTable,
		ReasonID:     reasonID,
		Owner:        owner,
		Coin:         w.Coin,
		FreeChange:   freeChange.String(),
		FreezeChange: freezeChange.String(),
		FreeNew:      uaa.Free.String(),
		FreezeNew:    uaa.Freeze.String(),
	}
	bankLog := BankLog{
		LogID: w.LogID,
		Ts:    time.Now().UnixNano(),

		BalanceLogs: []BalanceLog{bl},
	}

	blb, err := json.Marshal(bankLog)
	if err != nil {
		return
	}

	// write to filedb
	_, err = w.Filedb()
	if err != nil {
		return
	}
	err = w.fdb.WriteLine(string(blb) + "\n")
	// lastFiledbedTime = time.Now()
	// filedbedLines += 1
	if err != nil {
		return
	}

	return
}

func (w *Worker) GetSide(symbol string) (side string) {
	side = "bid"
	if strings.HasPrefix(strings.ToUpper(symbol), w.Coin) {
		side = "ask"
	}
	return
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
