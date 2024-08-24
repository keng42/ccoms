package model

import (
	"strings"

	"gorm.io/gorm"
)

// TicketTable generates different table names based on the trading pair
func TicketTable(symbol string, side string) func(tx *gorm.DB) *gorm.DB {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Table(strings.ToLower(symbol + "_" + side + "_tickets"))
	}
}

// OrderTable generates different table names based on the trading pair
func OrderTable(symbol string) func(tx *gorm.DB) *gorm.DB {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Table(strings.ToLower(symbol + "_orders"))
	}
}

// TradeTable generates different table names based on the trading pair
func TradeTable(symbol string) func(tx *gorm.DB) *gorm.DB {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Table(strings.ToLower(symbol + "_trades"))
	}
}

// BalanceSnapTable generates different table names based on the trading pair
func BalanceSnapTable(coin string) func(tx *gorm.DB) *gorm.DB {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.Table(strings.ToLower(coin + "_balance_snaps"))
	}
}
