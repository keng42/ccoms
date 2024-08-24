package model

import (
	"github.com/shopspring/decimal"
)

// Order model
type Order struct {
	ID int64 `json:"id" gorm:"omitempty; primaryKey;"`

	LogType   int64 `json:"logType" gorm:"omitempty; not null; default:0; uniqueindex:idx_t_log_type_id_index"`
	LogID     int64 `json:"logID" gorm:"omitempty; not null; default:0; uniqueindex:idx_t_log_type_id_index"`
	LogIndex  int64 `json:"logIndex" gorm:"omitempty; not null; default:0; uniqueindex:idx_t_log_type_id_index"`
	LogOffset int64 `json:"logOffset" gorm:"omitempty; not null; default:0;"` // Position in the file

	TicketID int64 `json:"ticketID" gorm:"omitempty; not null; default:0; index;"`

	Owner    int64   `json:"owner" gorm:"omitempty; not null; default:0; index;"`
	Side     int8    `json:"side" gorm:"omitempty; not null; default:0; type:tinyint(1);"` // 1 sell ask, 2 buy bid
	Type     int8    `json:"type" gorm:"omitempty; not null; default:0; type:tinyint(1);"` // 1 limit, 2 market
	Trades   int64   `json:"trades" gorm:"omitempty; not null; default:0;"`                // Current number of trades
	Time     int64   `json:"time" gorm:"omitempty; not null; default:0;"`                  // Order creation time
	FeeLevel float64 `json:"feeLevel" gorm:"omitempty; not null; default:0;"`              // Creator's fee rate level

	Price    decimal.Decimal `json:"price" gorm:"omitempty; not null; default:0; type:decimal(36,18);"`    // Price
	Quantity decimal.Decimal `json:"quantity" gorm:"omitempty; not null; default:0; type:decimal(36,18);"` // Remaining quantity
	OrigQty  decimal.Decimal `json:"origQty" gorm:"omitempty; not null; default:0; type:decimal(36,18);"`  // Quantity at the time of order creation
	Amount   decimal.Decimal `json:"amount" gorm:"omitempty; not null; default:0; type:decimal(36,18);"`   // Current total transaction amount

	Model
}

const (
	OrderStatusDeleted int8 = -1 // deleted

	OrderStatusDraft      int8 = 0  // Draft, not shown to users
	OrderStatusFrozen     int8 = 10 // Freezing funds stage
	OrderStatusFreezeFail int8 = 11
	OrderStatusMatching   int8 = 20 // Matching stage
	OrderStatusMatched    int8 = 21
	OrderStatusDone       int8 = 40 // Finishing stage
	OrderStatusCancel     int8 = 41 // User canceled
	OrderStatusBanned     int8 = 42 // Banned by the system
	OrderStatusAppealing  int8 = 43 // Under appeal
	OrderStatusAppealed   int8 = 44 // Appeal ended

	OrderSideAsk int8 = 1
	OrderSideBid int8 = 2

	OrderTypeLimit  int8 = 1
	OrderTypeMarket int8 = 2
)

// Price limits for market orders
var (
	OrderPriceMin = decimal.NewFromInt(0)
	OrderPriceMax = decimal.NewFromInt(999999999)
)
