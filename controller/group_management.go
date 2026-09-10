package controller

import (
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type updateChannelGroupsRequest struct {
	Groups []string `json:"groups"`
}

// UpdateChannelGroups updates only the existing channels.group mapping and the
// derived abilities rows. It deliberately does not expose any other channel
// field; channel creation/deletion/configuration remains in Channel Management.
func UpdateChannelGroups(c *gin.Context) {
	channelID, err := strconv.Atoi(c.Param("id"))
	if err != nil || channelID <= 0 {
		common.ApiErrorMsg(c, "渠道 ID 无效")
		return
	}

	var req updateChannelGroupsRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		common.ApiErrorMsg(c, "分组参数无效")
		return
	}

	validGroups := ratio_setting.GetGroupRatioCopy()
	seen := make(map[string]struct{}, len(req.Groups))
	groups := make([]string, 0, len(req.Groups))
	for _, group := range req.Groups {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		if _, ok := validGroups[group]; !ok {
			common.ApiErrorMsg(c, "分组不存在: "+group)
			return
		}
		if _, ok := seen[group]; ok {
			continue
		}
		seen[group] = struct{}{}
		groups = append(groups, group)
	}
	if len(groups) == 0 {
		common.ApiErrorMsg(c, "渠道至少需要关联一个分组")
		return
	}
	sort.Strings(groups)
	groupValue := strings.Join(groups, ",")

	channel, err := model.GetChannelById(channelID, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	err = model.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&model.Channel{}).
			Where("id = ?", channelID).
			Update("group", groupValue).Error; err != nil {
			return err
		}
		channel.Group = groupValue
		return channel.UpdateAbilities(tx)
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// abilities and the channel cache must agree immediately after the mapping
	// change, otherwise routing may continue using the previous group relation.
	model.InitChannelCache()
	recordManageAudit(c, "channel.groups.update", map[string]interface{}{
		"channel_id": channelID,
		"groups":     groups,
	})
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"id":     channelID,
			"name":   channel.Name,
			"groups": groups,
		},
	})
}
