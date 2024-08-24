package xnats

import "github.com/shopspring/decimal"

// OrderReq structure for creating an order request, sent from ingress to bank
type OrderReq struct {
	Symbol   string          `json:"symbol"`
	Owner    int64           `json:"owner"`
	Side     int8            `json:"side"`     // 0 sell ask, 1 buy bid
	Type     int8            `json:"type"`     // 0 limit, 1 market
	Price    decimal.Decimal `json:"price"`    // price
	Quantity decimal.Decimal `json:"quantity"` // remaining quantity
	OrigQty  decimal.Decimal `json:"origQty"`  // quantity at the time of order creation
	Amount   decimal.Decimal `json:"amount"`   // current total transaction amount
	Time     int64           `json:"time"`     // order creation time, in nanoseconds
	FeeLevel float64         `json:"feeLevel"` // creator's fee rate level
}

type BalancesReq struct {
	Items []BalanceReq `json:"items"`
}

type BalanceReq struct {
	User         int64           `json:"user"`
	Coin         string          `json:"coin"`
	FreeChange   decimal.Decimal `json:"freeChange"`
	FreezeChange decimal.Decimal `json:"freezeChange"`
	Time         int64           `json:"time"`
	Reason       string          `json:"reason"`
}

type BankMsg struct {
	Type string `json:"type"`

	OrderReq   *OrderReq
	BalanceReq *BalanceReq
}

const (
	BankMsgTypeOrderReq   = "OrderReq"
	BankMsgTypeBalanceReq = "BalanceReq"
)
