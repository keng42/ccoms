package model

import (
	"fmt"
	"log"
	"os"
	"time"

	"ccoms/pkg/config"
	"ccoms/pkg/model/xgorm"
	"ccoms/pkg/xlog"

	"github.com/go-redis/redis/v8"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
)

var (
	db     *gorm.DB
	rds    *redis.Client
	logger = xlog.GetLogger()

	dbSlience *gorm.DB
)

func DBInit() {
	db = OpenMySQL()
	dbSlience = OpenMySQLRaw("slience")
	rds = OpenRedis("main")
}

func OpenMySQL() *gorm.DB {
	return OpenMySQLRaw("main")
}

func OpenMySQLRaw(name string) *gorm.DB {
	cfg := config.Shared.MySQL.Main
	if cfg.Host == "" {
		logger.Fatalf("empty db host for %s", name)
	}

	logger.Infof("mysql(%s) connecting tcp(%s:%d)/%s",
		name, cfg.Host, cfg.Port, cfg.DB,
	)

	url := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local",
		cfg.User, cfg.Pass, cfg.Host, cfg.Port, cfg.DB,
	)

	logMode := gormLogger.Info
	if !config.Shared.IsDebug {
		logMode = gormLogger.Silent
	}
	newLogger := xgorm.New(
		log.New(os.Stdout, "", log.LstdFlags), // io writer
		gormLogger.Config{
			SlowThreshold:             time.Second, // Slow SQL threshold
			LogLevel:                  logMode,     // Log level
			IgnoreRecordNotFoundError: true,        // Ignore ErrRecordNotFound error for logger
			Colorful:                  true,        // Disable color
		},
	)

	if name == "slience" {
		logMode = gormLogger.Silent
		newLogger = gormLogger.Default
		newLogger.LogMode(gormLogger.Silent)
	}

	db, err := gorm.Open(mysql.Open(url), &gorm.Config{
		AllowGlobalUpdate:      true,
		SkipDefaultTransaction: false,
		Logger:                 newLogger,
	})

	if err != nil {
		logger.Fatalf("connect mysql failed #1, err:%s", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		logger.Fatalf("connect mysql failed #2, err:%s", err)
	}

	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(10 * time.Hour)
	sqlDB.SetMaxIdleConns(20)

	logger.Infof("mysql(%s) connected tcp(%s:%d)/%s",
		name, cfg.Host, cfg.Port, cfg.DB,
	)

	return db
}

func OpenRedis(name string) *redis.Client {
	cfg := config.Shared.Redis.Main
	if rds != nil {
		return rds
	}

	logger.Infof("redis(%s) connecting %s[%d]", name, cfg.Addr, cfg.DB)

	opts := redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Pass,
		DB:       cfg.DB,
	}

	rc := redis.NewClient(&opts)
	// _, err := rc.Ping().Result()

	// if err != nil {
	// 	logger.Fatalf("redis(%s) connect failed, err:%s", name, err)
	// }

	logger.Infof("redis(%s) connected %s[%d]", name, cfg.Addr, cfg.DB)

	return rc
}

func GetRedis() *redis.Client {
	return rds
}

func GetMySQL() *gorm.DB {
	return db
}

// GetMySQLSlience this instance reduces sql statement output
func GetMySQLSlience() *gorm.DB {
	return dbSlience
}
