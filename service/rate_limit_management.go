package service

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
)

const (
	RateLimitCategoryPoliciesOptionKey = "ModelRequestRateLimitCategoryPolicies"
	RateLimitSpecialPoliciesOptionKey  = "ModelRequestRateLimitSpecialPolicies"
	RateLimitManagedEnabledOptionKey   = "ModelRequestRateLimitManagedEnabled"
)

type ManagedRateLimitPair [2]int

type ManagedCategoryPolicies map[string]ManagedRateLimitPair
type ManagedSpecialPolicies map[string]map[string]ManagedRateLimitPair

type ManagedModelCategoryItem struct {
	ModelName string `json:"model_name"`
	Category  string `json:"category"`
}

type ManagedRateLimitSnapshot struct {
	ManagedEnabled bool                   `json:"managed_enabled"`
	Groups         []string               `json:"groups"`
	Models         []ManagedModelCategoryItem `json:"models"`
	CategoryLimits ManagedCategoryPolicies `json:"category_limits"`
	SpecialLimits  ManagedSpecialPolicies  `json:"special_limits"`
	GeneratedJSON  string                 `json:"generated_json"`
}

func getOptionString(key string) string {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	if value, ok := common.OptionMap[key]; ok {
		return common.Interface2String(value)
	}
	return ""
}

func ManagedRateLimitEnabled() bool {
	value := strings.TrimSpace(strings.ToLower(getOptionString(RateLimitManagedEnabledOptionKey)))
	return value == "true" || value == "1"
}

func validateManagedRateLimitPair(scope string, pair ManagedRateLimitPair) error {
	if pair[0] < 0 || pair[1] < 0 {
		return fmt.Errorf("%s 限流值不能为负数: [%d,%d]", scope, pair[0], pair[1])
	}
	if pair[0] > math.MaxInt32 || pair[1] > math.MaxInt32 {
		return fmt.Errorf("%s 限流值不能超过 2147483647", scope)
	}
	return nil
}

func LoadManagedCategoryPolicies() (ManagedCategoryPolicies, error) {
	result := ManagedCategoryPolicies{}
	raw := strings.TrimSpace(getOptionString(RateLimitCategoryPoliciesOptionKey))
	if raw == "" || raw == "{}" {
		return result, nil
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, err
	}
	for category, pair := range result {
		if !model.IsValidModelCategory(category) {
			return nil, fmt.Errorf("未知模型分类: %s", category)
		}
		if err := validateManagedRateLimitPair("模型分类 "+category, pair); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func LoadManagedSpecialPolicies() (ManagedSpecialPolicies, error) {
	result := ManagedSpecialPolicies{}
	raw := strings.TrimSpace(getOptionString(RateLimitSpecialPoliciesOptionKey))
	if raw == "" || raw == "{}" {
		return result, nil
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return nil, err
	}
	for group, models := range result {
		if strings.TrimSpace(group) == "" {
			return nil, fmt.Errorf("特殊限流分组不能为空")
		}
		for modelName, pair := range models {
			if strings.TrimSpace(modelName) == "" {
				return nil, fmt.Errorf("特殊限流模型不能为空")
			}
			if err := validateManagedRateLimitPair("特殊限流 "+group+"/"+modelName, pair); err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func getManagedGroups() []string {
	groupMap := ratio_setting.GetGroupRatioCopy()
	groups := make([]string, 0, len(groupMap))
	for group := range groupMap {
		group = strings.TrimSpace(group)
		if group != "" {
			groups = append(groups, group)
		}
	}
	sort.Strings(groups)
	return groups
}

func GetManagedRateLimitSnapshot() (*ManagedRateLimitSnapshot, error) {
	categoryLimits, err := LoadManagedCategoryPolicies()
	if err != nil {
		return nil, err
	}
	specialLimits, err := LoadManagedSpecialPolicies()
	if err != nil {
		return nil, err
	}
	modelNames, err := model.ListClassifiableModelNames()
	if err != nil {
		return nil, err
	}
	categoryMap, err := model.GetModelCategoryMap()
	if err != nil {
		return nil, err
	}
	models := make([]ManagedModelCategoryItem, 0, len(modelNames))
	for _, modelName := range modelNames {
		models = append(models, ManagedModelCategoryItem{
			ModelName: modelName,
			Category:  categoryMap[modelName],
		})
	}
	return &ManagedRateLimitSnapshot{
		ManagedEnabled: ManagedRateLimitEnabled(),
		Groups:         getManagedGroups(),
		Models:         models,
		CategoryLimits: categoryLimits,
		SpecialLimits:  specialLimits,
		GeneratedJSON:  setting.ModelRequestRateLimitGroup2JSONString(),
	}, nil
}

func SaveManagedCategoryPolicies(policies ManagedCategoryPolicies) error {
	for category, pair := range policies {
		if !model.IsValidModelCategory(category) {
			return fmt.Errorf("未知模型分类: %s", category)
		}
		if err := validateManagedRateLimitPair("模型分类 "+category, pair); err != nil {
			return err
		}
	}
	raw, err := json.Marshal(policies)
	if err != nil {
		return err
	}
	if err := model.UpdateOptionsBulk(map[string]string{
		RateLimitCategoryPoliciesOptionKey: string(raw),
		RateLimitManagedEnabledOptionKey:   "true",
	}); err != nil {
		return err
	}
	_, err = RebuildManagedModelRateLimits()
	return err
}

func SaveManagedSpecialPolicies(policies ManagedSpecialPolicies) error {
	validGroups := map[string]struct{}{}
	for _, group := range getManagedGroups() {
		validGroups[group] = struct{}{}
	}
	validModels, err := model.ListClassifiableModelNames()
	if err != nil {
		return err
	}
	modelSet := make(map[string]struct{}, len(validModels))
	for _, modelName := range validModels {
		modelSet[modelName] = struct{}{}
	}
	for group, modelRules := range policies {
		if _, ok := validGroups[group]; !ok {
			return fmt.Errorf("分组不存在: %s", group)
		}
		for modelName, pair := range modelRules {
			if _, ok := modelSet[modelName]; !ok {
				return fmt.Errorf("模型不存在: %s", modelName)
			}
			if err := validateManagedRateLimitPair("特殊限流 "+group+"/"+modelName, pair); err != nil {
				return err
			}
		}
	}
	raw, err := json.Marshal(policies)
	if err != nil {
		return err
	}
	if err := model.UpdateOptionsBulk(map[string]string{
		RateLimitSpecialPoliciesOptionKey: string(raw),
		RateLimitManagedEnabledOptionKey:  "true",
	}); err != nil {
		return err
	}
	_, err = RebuildManagedModelRateLimits()
	return err
}

// RebuildManagedModelRateLimits materializes the GUI-managed policy into the
// existing ModelRequestRateLimitGroup JSON consumed by the relay middleware.
// Direct top-level entries are preserved so existing phone/account limits stay
// backward compatible. Group-object entries become GUI-managed once managed
// mode is enabled.
func RebuildManagedModelRateLimits() (string, error) {
	if !ManagedRateLimitEnabled() {
		return setting.ModelRequestRateLimitGroup2JSONString(), nil
	}

	categoryLimits, err := LoadManagedCategoryPolicies()
	if err != nil {
		return "", err
	}
	specialLimits, err := LoadManagedSpecialPolicies()
	if err != nil {
		return "", err
	}
	categoryMap, err := model.GetModelCategoryMap()
	if err != nil {
		return "", err
	}

	generated := make(map[string]setting.ModelRequestRateLimitEntry)

	// Preserve legacy/direct top-level entries, especially phone/account rules.
	currentRaw := setting.ModelRequestRateLimitGroup2JSONString()
	current := make(map[string]setting.ModelRequestRateLimitEntry)
	if strings.TrimSpace(currentRaw) != "" {
		if err := json.Unmarshal([]byte(currentRaw), &current); err != nil {
			return "", err
		}
	}
	for key, entry := range current {
		if entry.Direct != nil {
			limits := *entry.Direct
			generated[key] = setting.ModelRequestRateLimitEntry{Direct: &limits}
		}
	}

	groups := getManagedGroups()
	for _, group := range groups {
		modelRules := map[string][2]int{
			"all": {0, 0},
		}

		// Category policy supplies the common baseline for every group.
		for modelName, category := range categoryMap {
			pair, ok := categoryLimits[category]
			if !ok || (pair[0] == 0 && pair[1] == 0) {
				continue
			}
			modelRules[modelName] = [2]int{pair[0], pair[1]}
		}

		// Group/model special policies have highest priority. [0,0] is kept so
		// it can explicitly disable a general category baseline for one group.
		if overrides, ok := specialLimits[group]; ok {
			for modelName, pair := range overrides {
				modelRules[modelName] = [2]int{pair[0], pair[1]}
			}
		}
		generated[group] = setting.ModelRequestRateLimitEntry{Models: modelRules}
	}

	jsonBytes, err := json.Marshal(generated)
	if err != nil {
		return "", err
	}
	jsonString := string(jsonBytes)
	if err := setting.CheckModelRequestRateLimitGroup(jsonString); err != nil {
		return "", err
	}
	if err := model.UpdateOption("ModelRequestRateLimitGroup", jsonString); err != nil {
		return "", err
	}
	return jsonString, nil
}

func UpdateManagedRateLimitDefaults(enabled bool, durationMinutes, totalCount, successCount int) error {
	if durationMinutes <= 0 {
		return fmt.Errorf("限流周期必须大于 0 分钟")
	}
	if totalCount < 0 || successCount < 0 {
		return fmt.Errorf("最大请求数和最大完成数不能为负数")
	}
	if totalCount > math.MaxInt32 || successCount > math.MaxInt32 {
		return fmt.Errorf("限流值不能超过 2147483647")
	}
	return model.UpdateOptionsBulk(map[string]string{
		"ModelRequestRateLimitEnabled":         strconv.FormatBool(enabled),
		"ModelRequestRateLimitDurationMinutes": strconv.Itoa(durationMinutes),
		"ModelRequestRateLimitCount":           strconv.Itoa(totalCount),
		"ModelRequestRateLimitSuccessCount":    strconv.Itoa(successCount),
	})
}
