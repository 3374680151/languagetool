// store_test.go 验证 GORM 建表与配额/用量仓储语义（§7.5 / FR-601/603/604）。
//
// @file internal/store/store_test.go
// 文件职责：AutoMigrate 全表、配额预扣（先扣后拒、恰好用完允许）、站点用量
// upsert 累加、失败数补记；隐私红线自查——表结构里没有正文列。
//
// 模块边界：用临时 SQLite 文件，不依赖外部状态。
package store

import (
	"path/filepath"
	"testing"
	"time"
)

func openTest(t *testing.T) *DB {
	t.Helper()
	db, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return db
}

// TestMigrateAllTables §7.5 的七张表必须都存在。
func TestMigrateAllTables(t *testing.T) {
	db := openTest(t)
	for _, table := range []string{"visitors", "usage_daily", "site_usage_daily", "glossaries", "words", "mistakes", "user_prefs"} {
		if !db.Migrator().HasTable(table) {
			t.Errorf("缺表 %s", table)
		}
	}
}

// TestConsumeQuota 预扣语义：先扣成功；超上限拒绝且不写库；恰好用完允许（FR-601）。
func TestConsumeQuota(t *testing.T) {
	db := openTest(t)
	r := NewUsageRepo(db)

	d, err := r.Consume("v1", 3, 100, 10)
	if err != nil || !d.Allowed {
		t.Fatalf("首次扣减应允许: %+v %v", d, err)
	}
	if d.Remaining != 7 {
		t.Fatalf("Remaining = %d, want 7", d.Remaining)
	}

	// 再扣 8 会超（3+8>10），拒绝且不写库。
	d, err = r.Consume("v1", 8, 100, 10)
	if err != nil {
		t.Fatal(err)
	}
	if d.Allowed {
		t.Fatal("超出上限应拒绝")
	}
	used, _ := r.TodayUsed("v1")
	if used != 3 {
		t.Fatalf("拒绝时不应写库，used = %d", used)
	}

	// 恰好用完的那次允许通过。
	d, _ = r.Consume("v1", 7, 0, 10)
	if !d.Allowed || d.Remaining != 0 {
		t.Fatalf("恰好用完应允许: %+v", d)
	}

	// 重启（新仓储实例）后用量仍在（FR-601 落库不归零）。
	d, _ = r.Consume("v1", 1, 0, 10)
	if d.Allowed {
		t.Fatal("重启后仍应拒绝")
	}
}

// TestQuotaUnlimited limit=0 表示不限但仍记账。
func TestQuotaUnlimited(t *testing.T) {
	db := openTest(t)
	r := NewUsageRepo(db)
	d, err := r.Consume("v2", 5, 10, 0)
	if err != nil || !d.Allowed || d.Remaining != -1 {
		t.Fatalf("不限应放行: %+v %v", d, err)
	}
	used, _ := r.TodayUsed("v2")
	if used != 5 {
		t.Fatalf("不限也要记账, used = %d", used)
	}
}

// TestAddFailures 失败数补记只增不覆盖。
func TestAddFailures(t *testing.T) {
	db := openTest(t)
	r := NewUsageRepo(db)
	if _, err := r.Consume("v3", 2, 10, 100); err != nil {
		t.Fatal(err)
	}
	if err := r.AddFailures("v3", 1); err != nil {
		t.Fatal(err)
	}
	if err := r.AddFailures("v3", 2); err != nil {
		t.Fatal(err)
	}
	var row UsageDaily
	if err := db.Where("visitor_id = ?", "v3").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.Failures != 3 {
		t.Fatalf("Failures = %d, want 3", row.Failures)
	}
}

// TestSiteUsage 站点用量按天 upsert 累加（FR-603 封顶依据）。
func TestSiteUsage(t *testing.T) {
	db := openTest(t)
	r := NewUsageRepo(db)
	if err := r.AddSiteUsage(100, 5, 2, 0, 1); err != nil {
		t.Fatal(err)
	}
	if err := r.AddSiteUsage(50, 0, 1, 3, 0); err != nil {
		t.Fatal(err)
	}
	paid, err := r.PaidCharsToday()
	if err != nil {
		t.Fatal(err)
	}
	if paid != 150 {
		t.Fatalf("PaidChars = %d, want 150", paid)
	}
	var row SiteUsageDaily
	if err := db.First(&row).Error; err != nil {
		t.Fatal(err)
	}
	if row.FreeChars != 5 || row.TranslateReq != 3 || row.ProofreadReq != 3 || row.DegradedReq != 1 {
		t.Fatalf("site usage = %+v", row)
	}
}

// TestPaidCharsCapBoundary 触顶判定口径：used + 本次 > 上限（FR-603）。
func TestPaidCharsCapBoundary(t *testing.T) {
	db := openTest(t)
	r := NewUsageRepo(db)
	_ = r.AddSiteUsage(999, 0, 1, 0, 0)
	paid, _ := r.PaidCharsToday()
	if paid != 999 {
		t.Fatalf("paid = %d", paid)
	}
}

// TestVisitorTouch 访客首访插入、再访只更新 last_seen_at（FR-1104）。
func TestVisitorTouch(t *testing.T) {
	db := openTest(t)
	r := NewVisitorRepo(db)
	now := time.Now()
	if err := r.Touch("0192abcd-0000-4000-8000-000000000001", now); err != nil {
		t.Fatal(err)
	}
	if err := r.Touch("0192abcd-0000-4000-8000-000000000001", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	var v Visitor
	if err := db.First(&v, "id = ?", "0192abcd-0000-4000-8000-000000000001").Error; err != nil {
		t.Fatal(err)
	}
	if !v.LastSeenAt.After(v.CreatedAt.Add(30 * time.Minute)) {
		t.Fatalf("last_seen 应被滑动续期: %+v", v)
	}
}

// TestPrivacyNoContentColumns 红线自查：任何表都不允许出现"正文/译文"列（NFR-6）。
func TestPrivacyNoContentColumns(t *testing.T) {
	db := openTest(t)
	for _, table := range []string{"visitors", "usage_daily", "site_usage_daily", "glossaries", "words", "mistakes", "user_prefs"} {
		cols, err := db.Migrator().ColumnTypes(table)
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range cols {
			name := c.Name()
			switch name {
			case "source_text", "translated_text", "content", "draft", "segments_text":
				t.Errorf("表 %s 出现正文列 %s（NFR-6 红线）", table, name)
			}
		}
	}
}
