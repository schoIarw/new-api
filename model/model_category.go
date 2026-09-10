package model

import (
	"sort"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	ModelCategoryFast      = "fast"
	ModelCategoryFlagship  = "flagship"
	ModelCategoryDedicated = "dedicated"
)

var (
	modelCategoryMigrateOnce sync.Once
	modelCategoryMigrateErr  error
)

// ModelCategory stores the operational classification of one model.
// model_name is unique so a model can belong to at most one category.
type ModelCategory struct {
	Id          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	ModelName   string `json:"model_name" gorm:"size:191;not null;uniqueIndex"`
	Category    string `json:"category" gorm:"size:32;not null;index"`
	CreatedTime int64  `json:"created_time" gorm:"bigint"`
	UpdatedTime int64  `json:"updated_time" gorm:"bigint"`
}

func IsValidModelCategory(category string) bool {
	switch category {
	case ModelCategoryFast, ModelCategoryFlagship, ModelCategoryDedicated:
		return true
	default:
		return false
	}
}

// EnsureModelCategoryTable keeps the feature deployable without changing the
// existing channel/group tables. The table is created lazily once per process.
func EnsureModelCategoryTable() error {
	modelCategoryMigrateOnce.Do(func() {
		modelCategoryMigrateErr = DB.AutoMigrate(&ModelCategory{})
	})
	return modelCategoryMigrateErr
}

func GetModelCategoryMap() (map[string]string, error) {
	if err := EnsureModelCategoryTable(); err != nil {
		return nil, err
	}
	var rows []ModelCategory
	if err := DB.Order("model_name ASC").Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make(map[string]string, len(rows))
	for _, row := range rows {
		result[row.ModelName] = row.Category
	}
	return result, nil
}

func UpsertModelCategory(modelName, category string) error {
	modelName = strings.TrimSpace(modelName)
	category = strings.TrimSpace(category)
	if modelName == "" {
		return gorm.ErrInvalidData
	}
	if !IsValidModelCategory(category) {
		return gorm.ErrInvalidData
	}
	if err := EnsureModelCategoryTable(); err != nil {
		return err
	}
	now := common.GetTimestamp()
	row := ModelCategory{
		ModelName:   modelName,
		Category:    category,
		CreatedTime: now,
		UpdatedTime: now,
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "model_name"}},
		DoUpdates: clause.Assignments(map[string]interface{}{
			"category":     category,
			"updated_time": now,
		}),
	}).Create(&row).Error
}

func DeleteModelCategory(modelName string) error {
	modelName = strings.TrimSpace(modelName)
	if modelName == "" {
		return gorm.ErrInvalidData
	}
	if err := EnsureModelCategoryTable(); err != nil {
		return err
	}
	return DB.Where("model_name = ?", modelName).Delete(&ModelCategory{}).Error
}

// ListClassifiableModelNames returns both model-registry names and actually
// routable ability names, so operational classification never misses a model
// merely because it has not been materialized in the models table.
func ListClassifiableModelNames() ([]string, error) {
	nameSet := map[string]struct{}{}

	var modelNames []string
	if err := DB.Model(&Model{}).Distinct("model_name").Pluck("model_name", &modelNames).Error; err != nil {
		return nil, err
	}
	for _, name := range modelNames {
		name = strings.TrimSpace(name)
		if name != "" {
			nameSet[name] = struct{}{}
		}
	}

	var abilityNames []string
	if err := DB.Model(&Ability{}).Distinct("model").Pluck("model", &abilityNames).Error; err != nil {
		return nil, err
	}
	for _, name := range abilityNames {
		name = strings.TrimSpace(name)
		if name != "" {
			nameSet[name] = struct{}{}
		}
	}

	result := make([]string, 0, len(nameSet))
	for name := range nameSet {
		result = append(result, name)
	}
	sort.Strings(result)
	return result, nil
}
