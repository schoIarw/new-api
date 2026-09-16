package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func readinessHTTPStatus() int {
	response := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(response)
	c.Request = httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	Readiness(c)
	return response.Code
}

func TestReadinessChecksDatabase(t *testing.T) {
	oldDB, oldRedis := model.DB, common.RedisEnabled
	t.Cleanup(func() {
		model.DB = oldDB
		common.RedisEnabled = oldRedis
	})
	common.RedisEnabled = false
	model.DB = nil
	if got := readinessHTTPStatus(); got != http.StatusServiceUnavailable {
		t.Fatalf("nil database: got %d, want 503", got)
	}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	model.DB = db
	if got := readinessHTTPStatus(); got != http.StatusOK {
		t.Fatalf("healthy database: got %d, want 200", got)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	if got := readinessHTTPStatus(); got != http.StatusServiceUnavailable {
		t.Fatalf("closed database: got %d, want 503", got)
	}
}
