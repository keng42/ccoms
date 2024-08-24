package model

import (
	"github.com/shopspring/decimal"
)

// Balance model
// There are more than 300 common cryptocurrencies, and creating a separate table for each currency would result in too many tables, so user ID is used for partitioning
type Balance struct {
	ID int64 `json:"id" gorm:"omitempty; primaryKey;"`

	Owner int64  `json:"owner" gorm:"omitempty; not null; default:0; uniqueindex:idx_b_owner_coin;"`
	Coin  string `json:"coin" gorm:"omitempty; not null; default:''; type:varchar(8); uniqueindex:idx_b_owner_coin;"`

	Free   decimal.Decimal `json:"free" gorm:"omitempty; not null; default:0; type:decimal(36,18);"`
	Freeze decimal.Decimal `json:"freeze" gorm:"omitempty; not null; default:0; type:decimal(36,18);"`

	Model
}
