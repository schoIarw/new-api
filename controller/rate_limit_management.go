package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/gin-gonic/gin"
)

type managedRateLimitDefaultsRequest struct {
	Enabled         bool `json:"enabled"`
	DurationMinutes int  `json:"duration_minutes"`
	TotalCount      int  `json:"total_count"`
	SuccessCount    int  `json:"success_count"`
}

type managedCategoryPoliciesRequest struct {
	Policies service.ManagedCategoryPolicies `json:"policies"`
}

type managedSpecialPoliciesRequest struct {
	Policies service.ManagedSpecialPolicies `json:"policies"`
}

type modelCategoryRequest struct {
	ModelName string `json:"model_name"`
	Category  string `json:"category"`
}

func GetRateLimitManagement(c *gin.Context) {
	snapshot, err := service.GetManagedRateLimitSnapshot()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"enabled":          setting.ModelRequestRateLimitEnabled,
			"duration_minutes": setting.ModelRequestRateLimitDurationMinutes,
			"total_count":      setting.ModelRequestRateLimitCount,
			"success_count":    setting.ModelRequestRateLimitSuccessCount,
			"categories": []gin.H{
				{"key": model.ModelCategoryFast, "name": "快速"},
				{"key": model.ModelCategoryFlagship, "name": "旗舰"},
				{"key": model.ModelCategoryDedicated, "name": "专用"},
			},
			"managed_enabled": snapshot.ManagedEnabled,
			"groups":          snapshot.Groups,
			"models":          snapshot.Models,
			"category_limits": snapshot.CategoryLimits,
			"special_limits":  snapshot.SpecialLimits,
			"generated_json":  snapshot.GeneratedJSON,
		},
	})
}

func UpdateRateLimitManagementDefaults(c *gin.Context) {
	var req managedRateLimitDefaultsRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的限流参数")
		return
	}
	if err := service.UpdateManagedRateLimitDefaults(req.Enabled, req.DurationMinutes, req.TotalCount, req.SuccessCount); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "rate_limit_management.defaults.update", nil)
	common.ApiSuccess(c, nil)
}

func UpdateRateLimitCategoryPolicies(c *gin.Context) {
	var req managedCategoryPoliciesRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的模型分类限流参数")
		return
	}
	if req.Policies == nil {
		req.Policies = service.ManagedCategoryPolicies{}
	}
	if err := service.SaveManagedCategoryPolicies(req.Policies); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "rate_limit_management.category_policy.update", nil)
	common.ApiSuccess(c, nil)
}

func UpdateRateLimitSpecialPolicies(c *gin.Context) {
	var req managedSpecialPoliciesRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的特殊限流参数")
		return
	}
	if req.Policies == nil {
		req.Policies = service.ManagedSpecialPolicies{}
	}
	if err := service.SaveManagedSpecialPolicies(req.Policies); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "rate_limit_management.special_policy.update", nil)
	common.ApiSuccess(c, nil)
}

func UpsertRateLimitModelCategory(c *gin.Context) {
	var req modelCategoryRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的模型分类参数")
		return
	}
	req.ModelName = strings.TrimSpace(req.ModelName)
	req.Category = strings.TrimSpace(req.Category)
	if req.ModelName == "" || !model.IsValidModelCategory(req.Category) {
		common.ApiErrorMsg(c, "模型名称或模型分类无效")
		return
	}
	if err := model.UpsertModelCategory(req.ModelName, req.Category); err != nil {
		common.ApiError(c, err)
		return
	}
	if _, err := service.RebuildManagedModelRateLimits(); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "rate_limit_management.model_category.update", map[string]interface{}{
		"model":    req.ModelName,
		"category": req.Category,
	})
	common.ApiSuccess(c, nil)
}

func DeleteRateLimitModelCategory(c *gin.Context) {
	var req modelCategoryRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "无效的模型分类参数")
		return
	}
	req.ModelName = strings.TrimSpace(req.ModelName)
	if req.ModelName == "" {
		common.ApiErrorMsg(c, "模型名称不能为空")
		return
	}
	if err := model.DeleteModelCategory(req.ModelName); err != nil {
		common.ApiError(c, err)
		return
	}
	if _, err := service.RebuildManagedModelRateLimits(); err != nil {
		common.ApiError(c, err)
		return
	}
	recordManageAudit(c, "rate_limit_management.model_category.delete", map[string]interface{}{
		"model": req.ModelName,
	})
	common.ApiSuccess(c, nil)
}

func RebuildRateLimitManagement(c *gin.Context) {
	generated, err := service.RebuildManagedModelRateLimits()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{"generated_json": generated},
	})
}
