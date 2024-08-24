package model

import (
	"github.com/shopspring/decimal"
)

// Trade model
type Trade struct {
	ID int64 `json:"id" gorm:"omitempty; primaryKey;"`

	LogType   int64 `json:"logType" gorm:"omitempty; not null; default:0; uniqueindex:idx_t_log_type_id_index"`
	LogID     int64 `json:"logID" gorm:"omitempty; not null; default:0; uniqueindex:idx_t_log_type_id_index"`
	LogIndex  int64 `json:"logIndex" gorm:"omitempty; not null; default:0; uniqueindex:idx_t_log_type_id_index"`
	LogOffset int64 `json:"logOffset" gorm:"omitempty; not null; default:0;"` // Position in the file

	Price    decimal.Decimal `json:"price" gorm:"omitempty; not null; default:0; type:decimal(36,18);"`
	Quantity decimal.Decimal `json:"quantity" gorm:"omitempty; not null; default:0; type:decimal(36,18);"`
	Amount   decimal.Decimal `json:"amount" gorm:"omitempty; not null; default:0; type:decimal(36,18);"`
	Time     int64           `json:"time" gorm:"omitempty; not null; default:0;"` // Adding index to creation time for hourly snapshots

	AskOrder int64           `json:"askOrder" gorm:"omitempty; not null; default:0; index;"`
	BidOrder int64           `json:"bidOrder" gorm:"omitempty; not null; default:0; index;"`
	Asker    int64           `json:"asker" gorm:"omitempty; not null; default:0; index;"`
	Bider    int64           `json:"bider" gorm:"omitempty; not null; default:0; index;"`
	AskFee   decimal.Decimal `json:"askFee" gorm:"omitempty; not null; default:0; type:decimal(36,18);"`
	BidFee   decimal.Decimal `json:"bidFee" gorm:"omitempty; not null; default:0; type:decimal(36,18);"`

	Model
}
