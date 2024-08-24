package model_test

import (
	"ccoms/pkg/config"
	"ccoms/pkg/model"
	"ccoms/pkg/xlog"
	"os"
	"path"
	"testing"

	"gorm.io/gorm"
)

var db *gorm.DB

func TestMain(m *testing.M) {
	config.Shared = &config.Config{
		IsDebug: true,
	}

	config.Shared.MySQL.Main = config.MySQLServer{
		Host:         "127.0.0.1",
		User:         "aaronn",
		Pass:         "localdbtestpwd",
		DB:           "ccoms",
		Port:         3306,
		MaxOpenConns: 8,
	}

	xlog.Init("test", path.Join(config.DEVDATA, "logs/ccoms-test.log"), nil)

	db = model.OpenMySQL()
	os.Exit(m.Run())
}

func TestMigrate(t *testing.T) {
	db.Scopes(model.OrderTable("btc_usdt")).AutoMigrate(model.Order{})
	db.Scopes(model.TradeTable("btc_usdt")).AutoMigrate(model.Trade{})
	db.Scopes(model.TicketTable("btc_usdt", "ask")).AutoMigrate(model.Ticket{})
	db.Scopes(model.TicketTable("btc_usdt", "bid")).AutoMigrate(model.Ticket{})

	db.Scopes(model.OrderTable("eth_usdt")).AutoMigrate(model.Order{})
	db.Scopes(model.TradeTable("eth_usdt")).AutoMigrate(model.Trade{})
	db.Scopes(model.TicketTable("eth_usdt", "ask")).AutoMigrate(model.Ticket{})
	db.Scopes(model.TicketTable("eth_usdt", "bid")).AutoMigrate(model.Ticket{})

	db.Scopes(model.OrderTable("eth_btc")).AutoMigrate(model.Order{})
	db.Scopes(model.TradeTable("eth_btc")).AutoMigrate(model.Trade{})
	db.Scopes(model.TicketTable("eth_btc", "ask")).AutoMigrate(model.Ticket{})
	db.Scopes(model.TicketTable("eth_btc", "bid")).AutoMigrate(model.Ticket{})

	db.Scopes(model.BalanceSnapTable("btc")).AutoMigrate(model.BalanceSnap{})
	db.Scopes(model.BalanceSnapTable("usdt")).AutoMigrate(model.BalanceSnap{})
	db.Scopes(model.BalanceSnapTable("eth")).AutoMigrate(model.BalanceSnap{})

	db.AutoMigrate(model.Lastkv{})
	db.AutoMigrate(model.Balance{})
	db.AutoMigrate(model.User{})
}
