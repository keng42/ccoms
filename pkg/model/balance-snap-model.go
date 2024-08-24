package model

import (
	"github.com/shopspring/decimal"
)

// BalanceSnap model
type BalanceSnap struct {
	ID int64 `json:"id" gorm:"omitempty; primaryKey;"`

	LogType   int64 `json:"logType" gorm:"omitempty; not null; default:0; uniqueindex:idx_log_type_id_index"`
	LogID     int64 `json:"logID" gorm:"omitempty; not null; default:0; uniqueindex:idx_log_type_id_index"`
	LogIndex  int64 `json:"logIndex" gorm:"omitempty; not null; default:0; uniqueindex:idx_log_type_id_index"`
	LogOffset int64 `json:"logOffset" gorm:"omitempty; not null; default:0;"` // 在文件中的位置

	Owner int64 `json:"owner" gorm:"omitempty; not null; default:0; index;"`

	FreeChange   decimal.Decimal `json:"freeChange" gorm:"omitempty; not null; default:0; type:decimal(36,18);"`
	FreezeChange decimal.Decimal `json:"freezeChange" gorm:"omitempty; not null; default:0; type:decimal(36,18);"`
	FreeNew      decimal.Decimal `json:"freeNew" gorm:"omitempty; not null; default:0; type:decimal(36,18);"`
	FreezeNew    decimal.Decimal `json:"freezeNew" gorm:"omitempty; not null; default:0; type:decimal(36,18);"`

	Model
}
