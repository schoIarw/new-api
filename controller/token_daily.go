package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// GetAllTokenDailyModel handles admin querying token daily model stats with pagination
func GetAllTokenDailyModel(c *gin.Context) {
	params := parseTokenDailyParams(c)
	resp, err := model.GetTokenDailyModel(params)
	if err != nil {
		logger.LogError(c, "GetTokenDailyModel error: "+err.Error())
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, resp)
}

// GetUserTokenDailyModel handles user querying their own token daily model stats with pagination
func GetUserTokenDailyModel(c *gin.Context) {
	userId := c.GetInt("id")
	params := parseTokenDailyParams(c)
	resp, err := model.GetTokenDailyModelByUser(userId, params)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, resp)
}

// GetAllTokenDailyTotal handles admin querying token daily total stats with pagination
func GetAllTokenDailyTotal(c *gin.Context) {
	params := parseTokenDailyParams(c)
	resp, err := model.GetTokenDailyTotal(params)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, resp)
}

// GetUserTokenDailyTotal handles user querying their own token daily total stats with pagination
func GetUserTokenDailyTotal(c *gin.Context) {
	userId := c.GetInt("id")
	params := parseTokenDailyParams(c)
	resp, err := model.GetTokenDailyTotalByUser(userId, params)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, resp)
}

// parseTokenDailyParams extracts query params from request
func parseTokenDailyParams(c *gin.Context) model.TokenDailyQueryParams {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	return model.TokenDailyQueryParams{
		StartDate: c.Query("start_date"),
		EndDate:   c.Query("end_date"),
		TokenKey:  c.Query("token_key"),
		Page:      page,
		PageSize:  pageSize,
	}
}
