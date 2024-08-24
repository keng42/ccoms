package model

import (
	"github.com/shopspring/decimal"
)

// Ticket model, created by bank, used to create orders for ome, partitioned by symbol and side
type Ticket struct {
	ID int64 `json:"id" gorm:"omitempty; primaryKey;"`

	LogType   int64 `json:"logType" gorm:"omitempty; not null; default:0; uniqueindex:idx_t_log_type_id_index"`
	LogID     int64 `json:"logID" gorm:"omitempty; not null; default:0; uniqueindex:idx_t_log_type_id_index"`
	LogIndex  int64 `json:"logIndex" gorm:"omitempty; not null; default:0; uniqueindex:idx_t_log_type_id_index"`
	LogOffset int64 `json:"logOffset" gorm:"omitempty; not null; default:0;"` // Position in the file

	Owner    int64   `json:"owner" gorm:"omitempty; not null; default:0; index;"`
	Type     int8    `json:"type" gorm:"omitempty; not null; default:0; type:tinyint(1);"` // 0 limit, 1 market
	Time     int64   `json:"time" gorm:"omitempty; not null; default:0;"`                  // Ticket creation time, nanoseconds
	FeeLevel float64 `json:"feeLevel" gorm:"omitempty; not null; default:0;"`              // Creator's fee rate level

	Price    decimal.Decimal `json:"price" gorm:"omitempty; not null; default:0; type:decimal(36,18);"`    // Price
	Quantity decimal.Decimal `json:"quantity" gorm:"omitempty; not null; default:0; type:decimal(36,18);"` // Quantity
	Amount   decimal.Decimal `json:"amount" gorm:"omitempty; not null; default:0; type:decimal(36,18);"`   // Total amount

	Model
}
