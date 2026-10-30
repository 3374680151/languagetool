// health_test.go 验证 /api/health 三态（FR-1003）。
//
// @file internal/httpapi/handler/health_test.go
// 文件职责：db ok/fail 与 gateway ok/fail 的组合可逐态演示；
// 探测失败时 HTTP 仍 200，用 body 表达状态。
//
// 模块边界：纯单元测试，httptest 假上游 + 临时 SQLite。
package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"fluentread/internal/config"
	"fluentread/internal/gateway"
	"fluentread/internal/store"
)

func healthApp(t *testing.T, base string) *App {
	t.Helper()
	db, err := store.Open(filepath.Join(t.TempDir(), "h.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(db); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Listen: ":0", DBPath: "x", BaseURL: base, APIKey: "k", UpstreamTimeout: 5 * time.Second}
	pool := gateway.NewPool(gateway.New(base, []string{"k"}, 5*time.Second), nil, cfg, nil, nil, 5*time.Second)
	return NewApp(cfg, nil, db, pool, nil, nil, nil, nil)
}

func callHealth(t *testing.T, app *App) (int, map[string]any) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/health", nil)
	app.Health(c)
	var body map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w.Code, body
}

// TestHealthAllOK 配好假上游时 db 与 gateway 都是 ok。
func TestHealthAllOK(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer up.Close()

	code, body := callHealth(t, healthApp(t, up.URL))
	if code != 200 {
		t.Fatalf("status = %d", code)
	}
	if body["db"] != "ok" || body["gateway"] != "ok" || body["ok"] != true {
		t.Fatalf("body = %v", body)
	}
}

// TestHealthGatewayFail 网关不可达 → gateway:fail（服务本身仍 200）。
func TestHealthGatewayFail(t *testing.T) {
	_, body := callHealth(t, healthApp(t, "http://127.0.0.1:1"))
	if body["gateway"] != "fail" {
		t.Fatalf("gateway = %v", body["gateway"])
	}
	if body["ok"] != false {
		t.Fatalf("ok = %v", body["ok"])
	}
}

// TestHealthDBFail 数据库不可达 → db:fail（运行时可演示的第三态，FR-1003）。
func TestHealthDBFail(t *testing.T) {
	app := healthApp(t, "")
	// 关闭底层连接池，模拟"库坏了但进程还活着"。
	sqlDB, err := app.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	_ = sqlDB.Close()

	_, body := callHealth(t, app)
	if body["db"] != "fail" {
		t.Fatalf("db = %v", body["db"])
	}
	if body["ok"] != false {
		t.Fatalf("ok = %v", body["ok"])
	}
	if body["gateway"] != "fail" {
		t.Fatalf("gateway = %v（未配置）", body["gateway"])
	}
}
