package repository

import (
	"context"
	"log"
	"time"

	"commercial-transactions-service/internal/config"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	DB    *gorm.DB
	RDB   *redis.Client
)

func InitMySQL(cfg *config.MySQLConfig) {
	var err error
	DB, err = gorm.Open(mysql.Open(cfg.DSN()), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("MySQL连接失败: %v", err)
	}
	sqlDB, _ := DB.DB()
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Hour)
	log.Println("MySQL 连接成功")
}

func InitRedis(cfg *config.RedisConfig) {
	RDB = redis.NewClient(&redis.Options{
		Addr:     cfg.Addr(),
		Password: cfg.Password,
		DB:       cfg.DB,
		PoolSize: cfg.PoolSize,
	})
	if err := RDB.Ping(context.Background()).Err(); err != nil {
		log.Printf("⚠️ Redis未连接（秒杀功能暂不可用）: %v", err)
		return
	}
	log.Println("Redis 连接成功")
}

func Close() {
	if DB != nil {
		sqlDB, _ := DB.DB()
		sqlDB.Close()
	}
	if RDB != nil {
		RDB.Close()
	}
}
