package main

import (
	"ccoms/pkg/model"
	"ccoms/pkg/xetcd"
	"fmt"
	"os"

	"github.com/nats-io/nats.go"
)

// PrepareForBenchmark prepare mysql, nats, etcd for benchmark with docker compose
func PrepareForBenchmark() (err error) {

	// 0. Check if prepared

	filePath := "/tmp/ccoms_bm_prepared_flag"

	_, err = os.Stat(filePath)
	if err == nil || !os.IsNotExist(err) {
		// already prepared, just wait
		select {}
	}

	// 1. Prepare database

	db := model.GetMySQL()

	type TableName struct {
		TableName string `gorm:"column:TABLE_NAME"`
	}
	var tableNames []TableName
	db.Raw("SELECT TABLE_NAME FROM information_schema.tables WHERE table_schema = DATABASE()").Scan(&tableNames)

	for _, t := range tableNames {
		db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS `%s`", t.TableName))
	}

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

	// 2. Prepare nats

	// Connect to nats and create jetstreams
	natsUrls := []string{"nats_btc:4222", "nats_usdt:4222"}
	for _, natsUrl := range natsUrls {
		var nc *nats.Conn

		logger.Infof("nats connecting %s", natsUrl)
		nc, err = nats.Connect(natsUrl)
		if err != nil {
			logger.Debugf("bm prepare failed with err:%s", err)
			return
		}
		logger.Infof("nats connected %s", natsUrl)

		// Create JetStream Context
		var js nats.JetStreamContext
		js, err = nc.JetStream()
		if err != nil {
			logger.Debugf("bm prepare failed with err:%s", err)
			return
		}

		// Create a Stream
		_, err = js.AddStream(&nats.StreamConfig{
			Name:     "BANK",
			Subjects: []string{"BANK.*.*"},
		})
		if err != nil {
			logger.Debugf("bm prepare failed with err:%s", err)
			return
		}
	}

	// 3. Prepare etcd

	err = xetcd.Put(xetcd.KeyNatsService("usdt"), "nats_usdt:4222")
	if err != nil {
		logger.Debugf("bm prepare failed with err:%s", err)
		return
	}
	err = xetcd.Put(xetcd.KeyNatsService("btc"), "nats_btc:4222")
	if err != nil {
		logger.Debugf("bm prepare failed with err:%s", err)
		return
	}
	err = xetcd.Put(xetcd.KeyNatsService("eth"), "nats_eth:4222")
	if err != nil {
		logger.Debugf("bm prepare failed with err:%s", err)
		return
	}
	err = xetcd.Put(xetcd.KeyBankService("usdt"), "bank_usdt:12341")
	if err != nil {
		logger.Debugf("bm prepare failed with err:%s", err)
		return
	}
	err = xetcd.Put(xetcd.KeyBankService("btc"), "bank_btc:12342")
	if err != nil {
		logger.Debugf("bm prepare failed with err:%s", err)
		return
	}
	err = xetcd.Put(xetcd.KeyBankService("eth"), "bank_eth:12343")
	if err != nil {
		logger.Debugf("bm prepare failed with err:%s", err)
		return
	}

	// 4. Create flag file -- set prepared

	_, err = os.Create(filePath)
	if err != nil {
		logger.Debugf("bm prepare failed with err:%s", err)
		return
	}

	logger.Infof("bm prepared")
	select {}
}
