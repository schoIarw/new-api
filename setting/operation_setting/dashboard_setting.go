package operation_setting

import "github.com/QuantumNous/new-api/setting/config"

// DashboardSetting 看板功能开关配置
type DashboardSetting struct {
	ModelDashboardEnabled        bool `json:"model_dashboard_enabled"`        // 模型看板
	RateLimitDashboardEnabled    bool `json:"rate_limit_dashboard_enabled"` // 限流看板
	PerformanceDashboardEnabled  bool `json:"performance_dashboard_enabled"` // 性能看板
	MySQLDashboardEnabled        bool `json:"mysql_dashboard_enabled"`        // MySQL面板
}

var dashboardSetting = DashboardSetting{
	ModelDashboardEnabled:        true,
	RateLimitDashboardEnabled:    true,
	PerformanceDashboardEnabled:  true,
	MySQLDashboardEnabled:        true,
}

func init() {
	config.GlobalConfig.Register("dashboard_setting", &dashboardSetting)
}

func GetDashboardSetting() *DashboardSetting {
	return &dashboardSetting
}
