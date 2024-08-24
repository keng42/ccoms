package bank

import (
	"ccoms/pkg/xgrpc"
	"math/big"

	"github.com/nats-io/nats.go"
	"github.com/shopspring/decimal"
)

type BankMsg struct {
	N *nats.Msg
	G *xgrpc.BalanceChange
}

// UserAsset  User's coin balance
type UserAsset struct {
	Free   decimal.Decimal
	Freeze decimal.Decimal
}

// BankLog  Bank log
type BankLog struct {
	LogID  int64  `json:"logID"`
	Ts     int64  `json:"ts"`
	MsgSeq uint64 `json:"msgSeq"` //  NATS msg stream sequence

	BalanceLogs []BalanceLog `json:"balances,omitempty"`
	TicketLogs  []TicketLog  `json:"tickets,omitempty"`
}

// BalanceLog  Balance log
type BalanceLog struct {
	LogIndex int64 `json:"logIndex"`

	Reason      string `json:"reason"`      // e.g. trade, cancel order
	ReasonTable string `json:"reasonTable"` // e.g. ome_btc_usdt_logs
	ReasonID    int64  `json:"reasonID"`    // e.g. log id

	Owner        int64  `json:"owner"`
	Coin         string `json:"coin"`
	FreeChange   string `json:"freeChange"`
	FreezeChange string `json:"freezeChange"`
	FreeNew      string `json:"freeNew"`
	FreezeNew    string `json:"freezeNew"`

	Owner2        int64  `json:"owner2,omitempty"`
	Coin2         string `json:"coin2,omitempty"`
	FreeChange2   string `json:"freeChange2,omitempty"`
	FreezeChange2 string `json:"freezeChange2,omitempty"`
	FreeNew2      string `json:"freeNew2,omitempty"`
	FreezeNew2    string `json:"freezeNew2,omitempty"`
}

// TicketLog  Ticket log
type TicketLog struct {
	LogIndex int64 `json:"logIndex"`

	Reason      string `json:"reason"`
	ReasonTable string `json:"reasonTable"`
	ReasonID    int64  `json:"reasonID"`

	ID       int64  `json:"id"`
	Owner    int64  `json:"owner"`
	Symbol   string `json:"symbol"`
	Type     int8   `json:"type"`
	Side     int8   `json:"side"`
	Price    string `json:"price"`
	Quantity string `json:"quantity"`
	Amount   string `json:"amount"`
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
