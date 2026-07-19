// Package database 负责初始化并暴露全局 GORM 数据库连接。
// 底层使用 PostgreSQL（通过 gorm.io/driver/postgres 驱动，基于 pgx）。
package database

import (
	"fmt"
	"log"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"yjedu-reading-club-server/internal/config"
)

// globalDB 是全局单例数据库连接。
var globalDB *gorm.DB

// Init 根据配置初始化 PostgreSQL 连接并设置连接池参数。
// 调用时机：main 函数启动时在 config.Load 之后调用。
// DSN 格式：host=... port=5432 user=... password=... dbname=... sslmode=disable TimeZone=Asia/Shanghai
func Init() (*gorm.DB, error) {
	cfg := config.Get()
	if cfg == nil || cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL 未配置")
	}

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
		// 关闭默认事务包装，由业务层显式使用事务，提升性能。
		SkipDefaultTransaction: true,
		// 启用错误翻译，将 PostgreSQL 唯一键冲突等错误翻译为 gorm 标准错误。
		TranslateError: true,
	}

	db, err := gorm.Open(postgres.Open(cfg.DatabaseURL), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("连接数据库失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层 sql.DB 失败: %w", err)
	}

	// 连接池参数：空闲 10、最大 100、连接最长寿命 1 小时。
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	globalDB = db
	log.Println("数据库连接已建立（PostgreSQL）")
	return db, nil
}

// Get 返回已初始化的全局数据库连接。
func Get() *gorm.DB {
	return globalDB
}
