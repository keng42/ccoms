package ome

import (
	"ccoms/pkg/xgrpc"
	"math/big"

	"github.com/google/btree"
	"github.com/shopspring/decimal"
)

type OmeMsg struct {
	G *xgrpc.Ticket
}

type OmeLog struct {
	LogID int64 `json:"logID"`
	Ts    int64 `json:"ts"`

	OrderLogs []OrderLog `json:"orders,omitempty"`
	MatchLogs []MatchLog `json:"matchs,omitempty"`
}

type MatchLog struct {
	LogIndex int64 `json:"logIndex"`

	// Latest information of the sell order, BTC
	Asker       int64    `json:"asker"`
	AskID       int64    `json:"askID"`
	AskPrice    *big.Int `json:"askPrice"`
	AskQuantity *big.Int `json:"askQuantity"` // Remaining quantity

	// Latest information of the buy order, USDT
	Bider       int64    `json:"bider"`
	BidID       int64    `json:"bidID"`
	BidPrice    *big.Int `json:"bidPrice"`
	BidQuantity *big.Int `json:"bidQuantity"` // Remaining quantity

	// Information of this transaction
	Price    *big.Int `json:"price"`    // BTC/USDT
	Quantity *big.Int `json:"quantity"` // BTC
	Amount   *big.Int `json:"amount"`   // USDT
	AskFee   *big.Int `json:"askFee"`   // Fee, BTC
	BidFee   *big.Int `json:"bidFee"`   // Fee, USDT

	Time int64 `json:"time"`
}

type OrderLog struct {
	LogIndex int64 `json:"logIndex"`

	ID       int64    `json:"id"`
	TicketID int64    `json:"ticketID"`
	Owner    int64    `json:"owner"`
	FeeRate  int64    `json:"feeRate"`
	Time     int64    `json:"time"`
	Side     int8     `json:"side"`
	Type     int8     `json:"type"`
	Price    *big.Int `json:"price"`
	Quantity *big.Int `json:"quantity"`
}

type NewOrder struct {
	ID       int64    `json:"id"`
	TicketID int64    `json:"ticketID"`
	Owner    int64    `json:"owner"`
	FeeRate  int64    `json:"feeRate"`
	Time     int64    `json:"time"`
	Side     int8     `json:"side"`
	Type     int8     `json:"type"`
	Price    *big.Int `json:"price"`
	Quantity *big.Int `json:"quantity"`
}

// Order minimal order information
type Order struct {
	ID       int64
	TicketID int64
	Owner    int64
	FeeRate  int64
	Price    *big.Int
	Quantity *big.Int
}

// AskOrder minimal sell order information
type AskOrder Order

// BidOrder minimal buy order information
type BidOrder Order

// Less compare the size of two Orders
func (a Order) Less(item btree.Item) bool {
	b, _ := item.(Order)

	if a.ID == b.ID {
		return false
	}

	f := a.Price.Cmp(b.Price)
	if f == 0 {
		return a.ID < b.ID
	}

	// a.Price > b.Price
	return f > 0
}

// Less compare the size of two AskOrders
func (a AskOrder) Less(item btree.Item) bool {
	b, _ := item.(AskOrder)

	if a.ID == b.ID {
		return false
	}

	f := a.Price.Cmp(b.Price)
	if f == 0 {
		return a.ID < b.ID
	}

	// a.Price < b.Price
	return f < 0
}

// Less compare the size of two BidOrders
func (a BidOrder) Less(item btree.Item) bool {
	b, _ := item.(BidOrder)

	if a.ID == b.ID {
		return false
	}

	f := a.Price.Cmp(b.Price)
	if f == 0 {
		return a.ID < b.ID
	}

	// a.Price < b.Price
	return f < 0
}

var Exp = decimal.New(1, 12)
var ExpInt = Exp.BigInt()

func DecimalToInt(d decimal.Decimal) *big.Int {
	return d.Mul(Exp).BigInt()
}

func IntToDecimal(i *big.Int) decimal.Decimal {
	return decimal.NewFromBigInt(i, -12)
}

func Equal(a, b *big.Int) bool {
	return a.Cmp(b) == 0
}

func Less(a, b *big.Int) bool {
	return a.Cmp(b) < 0
}

func Greater(a, b *big.Int) bool {
	return a.Cmp(b) > 0
}

func IsZero(i *big.Int) bool {
	return len(i.Bits()) == 0
}
