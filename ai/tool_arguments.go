package ai

import (
	"encoding/json"
	"fmt"
	"math"
)

func parseToolArguments(meta *ToolMeta, raw string) (map[string]any, error) {
	var params map[string]any
	if err := json.Unmarshal([]byte(raw), &params); err != nil || params == nil {
		return nil, fmt.Errorf("参数必须是完整的 JSON 对象")
	}
	known := map[string]bool{}
	for _, param := range meta.Params {
		known[param.Name] = true
		value, exists := params[param.Name]
		if !exists {
			if param.Required {
				return nil, fmt.Errorf("缺少参数 %s", param.Name)
			}
			continue
		}
		if !matchesParamType(value, param.Type) {
			return nil, fmt.Errorf("参数 %s 类型应为 %s", param.Name, param.Type)
		}
		if len(param.Enum) > 0 {
			valid := false
			for _, option := range param.Enum {
				if value == option {
					valid = true
					break
				}
			}
			if !valid {
				return nil, fmt.Errorf("参数 %s 不在允许的选项中", param.Name)
			}
		}
		if param.Type == "array" && param.ItemsType != "" {
			for _, item := range value.([]any) {
				if !matchesParamType(item, param.ItemsType) {
					return nil, fmt.Errorf("参数 %s 数组元素类型应为 %s", param.Name, param.ItemsType)
				}
			}
		}
	}
	for name := range params {
		if !known[name] {
			return nil, fmt.Errorf("未知参数 %s", name)
		}
	}
	return params, nil
}

func matchesParamType(value any, kind string) bool {
	switch kind {
	case "string":
		_, ok := value.(string)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "number":
		_, ok := value.(float64)
		return ok
	case "integer":
		n, ok := value.(float64)
		return ok && math.Trunc(n) == n && n >= -9007199254740991 && n <= 9007199254740991
	case "array":
		_, ok := value.([]any)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	default:
		return false
	}
}
