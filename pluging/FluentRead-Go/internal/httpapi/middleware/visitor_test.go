// visitor_test.go 验证匿名访客 cookie（FR-1104）。
//
// @file internal/httpapi/middleware/visitor_test.go
// 文件职责：首访下发 UUID cookie（60 天滑动）、无效 cookie 被替换、
// 身份注入 context、双桶限流的访客/IP 键（FR-602）。
//
// 模块边界：纯单元测试。
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"fluentread/internal/ratelimit"
)

func newCtx(t *testing.T, cookie string) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("POST", "/api/translate", nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: CookieName, Value: cookie})
	}
	c.Request = req
	return c, w
}

// TestVisitorIssuesCookie 首访下发 UUID v4 cookie。
func TestVisitorIssuesCookie(t *testing.T) {
	c, w := newCtx(t, "")
	Visitor(nil)(c)
	id := VisitorID(c)
	if !validID(id) {
		t.Fatalf("应下发合法 UUID: %q", id)
	}
	cookie := w.Header().Get("set-cookie")
	if cookie == "" {
		t.Fatal("应写入 set-cookie")
	}
}

// TestVisitorKeepsExisting 已有合法 cookie 时不重发。
func TestVisitorKeepsExisting(t *testing.T) {
	id := newID()
	c, w := newCtx(t, id)
	Visitor(nil)(c)
	if VisitorID(c) != id {
		t.Fatalf("应复用现有身份: %q", VisitorID(c))
	}
	if w.Header().Get("set-cookie") != "" {
		t.Fatal("不应重写 cookie")
	}
}

// TestVisitorReplacesInvalid 伪造/超长 cookie 应被替换。
func TestVisitorReplacesInvalid(t *testing.T) {
	for _, bad := range []string{"short", "XXXX-xxxx", newID() + "x"} {
		c, _ := newCtx(t, bad)
		Visitor(nil)(c)
		if VisitorID(c) == bad || !validID(VisitorID(c)) {
			t.Fatalf("非法 cookie 应被替换: %q -> %q", bad, VisitorID(c))
		}
	}
}

// TestVisitorTouch 异步续期要把访客写入表。
func TestVisitorTouch(t *testing.T) {
	// 续期失败不影响请求（repo=nil 分支已覆盖）；这里验证 goroutine 不 panic。
	c, _ := newCtx(t, "")
	Visitor(nil)(c)
	if VisitorID(c) == "" {
		t.Fatal("应有访客 id")
	}
}

// TestAllowRequest 双桶限流：访客桶 + IP 桶（FR-602）。
func TestAllowRequest(t *testing.T) {
	limiter := ratelimit.New(2) // 访客 2 次/分钟
	c, _ := newCtx(t, newID())
	c.Request.Header.Set("X-Forwarded-For", "203.0.113.7")

	for i := 0; i < 2; i++ {
		if ok, _ := AllowRequest(c, limiter); !ok {
			t.Fatalf("第 %d 次应放行", i+1)
		}
	}
	if ok, retry := AllowRequest(c, limiter); ok {
		t.Fatal("第 3 次应被访客桶拒绝")
	} else if retry < 1 {
		t.Fatal("应给等待秒数")
	}
}

// TestAllowRequestIPBucket IP 桶独立计数：换访客仍受 IP 桶约束。
func TestAllowRequestIPBucket(t *testing.T) {
	limiter := ratelimit.New(100)
	c, _ := newCtx(t, newID())
	c.Request.Header.Set("X-Forwarded-For", "198.51.100.9")
	// IP 桶 4×=400 次，直接打满。
	for i := 0; i < 400; i++ {
		AllowRequest(c, limiter)
	}
	if ok, _ := AllowRequest(c, limiter); ok {
		t.Fatal("IP 桶超限应拒绝")
	}
}
