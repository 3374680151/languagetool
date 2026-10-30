// Package store 是 GORM + SQLite 的持久层。
//
// @file internal/store/store.go
// 文件职责：打开数据库（glebarez 纯 Go 驱动，DR-3：禁 CGO）、执行 AutoMigrate
//（§7.5 全表：visitors / usage_daily / site_usage_daily / glossaries / words /
// mistakes / user_prefs）、提供 Ping 供健康检查使用。
//
// 主要内容：Open、Ping、Close。
//
// 模块边界：各表模型定义在 model.go，仓储在各自的 *_repo.go；本包不写业务查询。
// 红线自查：§7.5 没有一张表存正文/译文（NFR-6）；usage_* 表只有数字。
package store

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB 是全局数据库句柄的类型别名，方便仓储构造函数签名。
type DB = gorm.DB

// Open 打开 SQLite 数据库并设置连接参数。
// WAL + busy_timeout：读写并发时写方短暂等待而不是报错。
// 连接池压到 1 写 + 少量读会序列化所有查询；这里保留默认池，靠 busy_timeout 消化写冲突。
func Open(path string) (*gorm.DB, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("创建数据目录失败: %w", err)
		}
	}

	// glebarez/sqlite 走 modernc，纯 Go；DSN 用 _pragma 传参启用 WAL 与忙等。
	dsn := "file:" + path +
		"?_pragma=busy_timeout(5000)" +
		"&_pragma=journal_mode(WAL)" +
		"&_pragma=foreign_keys(1)" +
		"&_pragma=synchronous(NORMAL)"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		// AutoMigrate 前不需要 SQL 日志刷屏；出问题时 slog 层会记录。
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("打开数据库失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("取底层连接失败: %w", err)
	}
	// SQLite 单文件：连接数过多只会加剧锁竞争，收着点。
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(4)
	sqlDB.SetConnMaxLifetime(0)

	if err := db.Exec("SELECT 1").Error; err != nil {
		return nil, fmt.Errorf("数据库不可用: %w", err)
	}
	return db, nil
}

// Migrate 执行 AutoMigrate，把 §7.5 全表建齐。幂等，启动时调用。
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&Visitor{},
		&UsageDaily{},
		&SiteUsageDaily{},
		&Glossary{},
		&Word{},
		&Mistake{},
		&UserPref{},
	)
}

// Ping 供健康检查探测数据库是否可达（FR-1003 的 db 态）。
func Ping(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("数据库未初始化")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}

// Close 关闭底层连接池。
func Close(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
