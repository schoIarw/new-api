package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildRateLimitItemsSeparatesConfiguredScopes(t *testing.T) {
	require.NoError(t, setting.UpdateModelRequestRateLimitGroupByJSONString(
		`{"gold":{"all":[10,5],"model-a":[4,2]}}`,
	))
	require.NoError(t, setting.UpdatePhoneRateLimitPoliciesByJSONString(
		`{"gold":{"default":[3,1]}}`,
	))
	t.Cleanup(func() {
		_ = setting.UpdateModelRequestRateLimitGroupByJSONString(`{}`)
		_ = setting.UpdatePhoneRateLimitPoliciesByJSONString(`{}`)
	})

	items := buildRateLimitItems([]model.RateLimitGroupStat{{
		UserID: 7, Group: "gold", TokenName: "key-a", ModelName: "model-a", Account: "13800138000", Count: 1,
	}})
	require.Len(t, items, 3)

	limits := map[string]int{}
	limited := map[string]bool{}
	for _, item := range items {
		limits[item.LimitType] = item.SuccessLimit
		limited[item.LimitType] = item.RateLimited
	}
	assert.Equal(t, 5, limits["group"])
	assert.Equal(t, 2, limits["group_model"])
	assert.Equal(t, 1, limits["group_phone"])
	assert.True(t, limited["group_phone"])
}
