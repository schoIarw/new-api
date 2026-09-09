package setting

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

var ModelRequestRateLimitEnabled = false
var ModelRequestRateLimitDurationMinutes = 1
var ModelRequestRateLimitCount = 0
var ModelRequestRateLimitSuccessCount = 1000

// ModelRequestRateLimitEntry supports both legacy/direct limits and model-scoped group limits.
// Direct form (kept for phone/account and backward compatibility):
//   "18946512326": [10, 5]
// Group model form:
//   "testgroup": {"all": [10, 5], "qwen35-27b": [10, 5]}
//
// [0, 0] means no limit. For a group object, a missing "all" also means no all-model limit.
type ModelRequestRateLimitEntry struct {
	Direct *[2]int
	Models map[string][2]int
}

func (e *ModelRequestRateLimitEntry) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return fmt.Errorf("empty rate limit entry")
	}

	switch data[0] {
	case '[':
		var values []int
		if err := json.Unmarshal(data, &values); err != nil {
			return err
		}
		if len(values) != 2 {
			return fmt.Errorf("rate limit array must contain exactly 2 integers")
		}
		limits := [2]int{values[0], values[1]}
		e.Direct = &limits
		e.Models = nil
		return nil
	case '{':
		var rawModels map[string][]int
		if err := json.Unmarshal(data, &rawModels); err != nil {
			return err
		}
		models := make(map[string][2]int, len(rawModels))
		for modelName, values := range rawModels {
			if modelName == "" {
				return fmt.Errorf("model name cannot be empty")
			}
			if len(values) != 2 {
				return fmt.Errorf("model %s rate limit array must contain exactly 2 integers", modelName)
			}
			models[modelName] = [2]int{values[0], values[1]}
		}
		e.Direct = nil
		e.Models = models
		return nil
	default:
		return fmt.Errorf("rate limit entry must be [total, success] or an object of model limits")
	}
}

func (e ModelRequestRateLimitEntry) MarshalJSON() ([]byte, error) {
	if e.Direct != nil {
		return json.Marshal(e.Direct)
	}
	if e.Models == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(e.Models)
}

var ModelRequestRateLimitGroup = map[string]ModelRequestRateLimitEntry{}
var ModelRequestRateLimitMutex sync.RWMutex

func ModelRequestRateLimitGroup2JSONString() string {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	jsonBytes, err := json.Marshal(ModelRequestRateLimitGroup)
	if err != nil {
		common.SysLog("error marshalling model request rate limit group: " + err.Error())
	}
	return string(jsonBytes)
}

func validateRateLimitPair(name string, limits [2]int) error {
	if limits[0] < 0 || limits[1] < 0 {
		return fmt.Errorf("%s has negative rate limit values: [%d, %d]", name, limits[0], limits[1])
	}
	if limits[0] > math.MaxInt32 || limits[1] > math.MaxInt32 {
		return fmt.Errorf("%s [%d, %d] has max rate limits value 2147483647", name, limits[0], limits[1])
	}
	return nil
}

func validateModelRequestRateLimitEntries(parsed map[string]ModelRequestRateLimitEntry) error {
	for group, entry := range parsed {
		if entry.Direct != nil {
			if err := validateRateLimitPair("group "+group, *entry.Direct); err != nil {
				return err
			}
			continue
		}
		for modelName, limits := range entry.Models {
			if err := validateRateLimitPair(fmt.Sprintf("group %s model %s", group, modelName), limits); err != nil {
				return err
			}
		}
	}
	return nil
}

func UpdateModelRequestRateLimitGroupByJSONString(jsonStr string) error {
	parsed := make(map[string]ModelRequestRateLimitEntry)
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return err
	}
	if err := validateModelRequestRateLimitEntries(parsed); err != nil {
		return err
	}

	ModelRequestRateLimitMutex.Lock()
	defer ModelRequestRateLimitMutex.Unlock()
	ModelRequestRateLimitGroup = parsed
	return nil
}

// GetGroupRateLimit returns a direct [total, success] entry.
// It keeps phone/account limits and legacy top-level array entries compatible.
func GetGroupRateLimit(group string) (totalCount, successCount int, found bool) {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	if ModelRequestRateLimitGroup == nil {
		return 0, 0, false
	}

	entry, found := ModelRequestRateLimitGroup[group]
	if !found || entry.Direct == nil {
		return 0, 0, false
	}
	return entry.Direct[0], entry.Direct[1], true
}

// HasGroupModelRateLimit reports whether the group is configured with the new object form.
func HasGroupModelRateLimit(group string) bool {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	entry, found := ModelRequestRateLimitGroup[group]
	return found && entry.Direct == nil && entry.Models != nil
}

// GetGroupModelRateLimit returns an exact model rule from a group object.
// modelName may be "all" for the group's all-model aggregate rule.
func GetGroupModelRateLimit(group, modelName string) (totalCount, successCount int, found bool) {
	ModelRequestRateLimitMutex.RLock()
	defer ModelRequestRateLimitMutex.RUnlock()

	entry, found := ModelRequestRateLimitGroup[group]
	if !found || entry.Direct != nil || entry.Models == nil {
		return 0, 0, false
	}
	limits, found := entry.Models[modelName]
	if !found {
		return 0, 0, false
	}
	return limits[0], limits[1], true
}

func CheckModelRequestRateLimitGroup(jsonStr string) error {
	parsed := make(map[string]ModelRequestRateLimitEntry)
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return err
	}
	return validateModelRequestRateLimitEntries(parsed)
}
