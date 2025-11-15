package orchestration

import (
	"encoding/json"
	"strconv"
)

func extractActualCost(defaultCost float64, result map[string]interface{}) float64 {
	if result == nil {
		return defaultCost
	}

	if cost, ok := parseFloat(result["actual_cost"]); ok {
		return cost
	}
	if cost, ok := parseFloat(result["cost"]); ok {
		return cost
	}
	return defaultCost
}

func parseFloat(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		f, err := v.Float64()
		if err == nil {
			return f, true
		}
	case string:
		if val, err := strconv.ParseFloat(v, 64); err == nil {
			return val, true
		}
	}
	return 0, false
}
