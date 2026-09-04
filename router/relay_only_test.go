package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 部署契约：RELAY_ONLY=true 的网关容器只暴露 API 协议面，
// /api 控制台、/pg playground 与 Web 前端由管理容器承载。
func TestRelayOnlyModeRouteSurface(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.RelayOnly = true
	defer func() { common.RelayOnly = false }()

	engine := gin.New()
	SetRouter(engine, ClassicAssets{})

	routes := engine.Routes()
	hasRoute := func(method, path string) bool {
		for _, r := range routes {
			if r.Method == method && r.Path == path {
				return true
			}
		}
		return false
	}
	hasPrefix := func(prefix string) bool {
		for _, r := range routes {
			if len(r.Path) >= len(prefix) && r.Path[:len(prefix)] == prefix {
				return true
			}
		}
		return false
	}

	// 健康检查与 relay 协议面必须保留
	assert.True(t, hasRoute(http.MethodGet, "/healthz"))
	assert.True(t, hasRoute(http.MethodPost, "/v1/chat/completions"))
	assert.True(t, hasRoute(http.MethodPost, "/v1/messages"))
	assert.True(t, hasRoute(http.MethodPost, "/v1/video/generations"))
	assert.True(t, hasRoute(http.MethodPost, "/mj/submit/imagine"))
	assert.True(t, hasRoute(http.MethodPost, "/suno/submit/:action"))
	assert.True(t, hasRoute(http.MethodPost, "/kling/v1/videos/text2video"))
	// 旧版 SDK 的 billing 兼容路由必须保留
	assert.True(t, hasRoute(http.MethodGet, "/dashboard/billing/subscription"))
	assert.True(t, hasRoute(http.MethodGet, "/v1/dashboard/billing/usage"))

	// 控制台面必须全部裁掉
	assert.False(t, hasPrefix("/api"))
	assert.False(t, hasPrefix("/pg"))
}

func TestRelayOnlyModeHealthzAndFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.RelayOnly = true
	defer func() { common.RelayOnly = false }()

	engine := gin.New()
	SetRouter(engine, ClassicAssets{})

	// 健康检查可用
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// /api 请求落入 NoRoute，返回 OpenAI 风格 404 而非管理接口
	w = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/user/self", nil)
	engine.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "invalid_request_error")
}

// 回归保护：管理模式下 playground 保持注册
func TestFullModeKeepsPlayground(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.RelayOnly = false

	engine := gin.New()
	SetDashboardRouter(engine)
	SetRelayRouter(engine)

	for _, r := range engine.Routes() {
		if r.Method == http.MethodPost && r.Path == "/pg/chat/completions" {
			return
		}
	}
	t.Fatal("route POST /pg/chat/completions not found in full mode")
}
