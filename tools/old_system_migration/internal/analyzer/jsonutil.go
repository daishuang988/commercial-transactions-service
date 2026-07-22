package analyzer

import (
	"encoding/json"
	"strings"
)

// parseJSON 解析 JSON 字符串，返回 map/array/基本类型
func parseJSON(jsonStr string) any {
	jsonStr = strings.TrimSpace(jsonStr)
	if jsonStr == "" {
		return nil
	}
	var result any
	dec := json.NewDecoder(strings.NewReader(jsonStr))
	dec.UseNumber()
	if err := dec.Decode(&result); err != nil {
		return nil
	}
	return normalizeJSON(result)
}

// normalizeJSON 把 json.Number 转为 int64 或 float64，方便后续类型推断
func normalizeJSON(v any) any {
	switch val := v.(type) {
	case json.Number:
		if i, err := val.Int64(); err == nil {
			return float64(i)
		}
		if f, err := val.Float64(); err == nil {
			return f
		}
		return val.String()
	case map[string]any:
		out := make(map[string]any, len(val))
		for k, vv := range val {
			out[k] = normalizeJSON(vv)
		}
		return out
	case []any:
		out := make([]any, len(val))
		for i, vv := range val {
			out[i] = normalizeJSON(vv)
		}
		return out
	default:
		return v
	}
}
