package controller

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
)

func GetPhoneRateLimitPolicies(c *gin.Context) {
	var policies setting.PhoneRateLimitPolicies
	if err := json.Unmarshal([]byte(setting.PhoneRateLimitPolicies2JSONString()), &policies); err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": policies})
}

func UpdatePhoneRateLimitPolicies(c *gin.Context) {
	var request struct {
		Policies json.RawMessage `json:"policies"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || len(request.Policies) == 0 {
		common.ApiErrorMsg(c, "手机号限流策略必须是 JSON 对象")
		return
	}
	policies, err := setting.ParsePhoneRateLimitPolicies(string(request.Policies))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	groups := ratio_setting.GetGroupRatioCopy()
	for group := range policies {
		if group != "auto" {
			if _, exists := groups[group]; !exists {
				common.ApiErrorMsg(c, "不存在的令牌分组: "+group)
				return
			}
		}
		if strings.TrimSpace(group) == "" {
			common.ApiErrorMsg(c, "令牌分组不能为空")
			return
		}
	}
	encoded, err := json.Marshal(policies)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err = model.UpdateOptionsBulk(map[string]string{
		setting.PhoneRateLimitPoliciesOptionKey: string(encoded),
	}); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
