package analyzer

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// ColumnInfo 列信息
type ColumnInfo struct {
	Name         string
	DataType     string
	Nullable     bool
	IsPrimaryKey bool
	MaxLen       int    // VARCHAR 时记录最大观测长度
	SeenValues   []any  // 采样值（用于人工确认类型推断是否正确）
	SeenCount    int    // 该字段出现的总次数
	NullCount    int    // 该字段为 null 的次数
	IsArray      bool
	IsObject     bool
	ChildSchema  *TableSchema // 嵌套对象/数组的子结构
}

// TableSchema 表结构
type TableSchema struct {
	TableName string
	Columns   []*ColumnInfo
	Indexes   []string
}

// AnalysisResult 分析结果
type AnalysisResult struct {
	Endpoint    string
	Method      string
	TableName   string
	Schemas     []*TableSchema // 主表 + 关联子表
	SampleCount int
}

// AnalyzeResponse 分析一个 API 的响应体，推断表结构
func AnalyzeResponse(endpoint, method string, responseBodies [][]byte) *AnalysisResult {
	result := &AnalysisResult{
		Endpoint:    endpoint,
		Method:      method,
		TableName:   endpointToTableName(endpoint),
		SampleCount: len(responseBodies),
	}

	// 合并多页数据
	var allRecords []map[string]any
	for _, body := range responseBodies {
		records := extractRecords(body)
		allRecords = append(allRecords, records...)
	}

	if len(allRecords) == 0 {
		return result
	}

	// 分析主表
	mainTable := analyzeRecords(allRecords, result.TableName)
	result.Schemas = append(result.Schemas, mainTable)

	// 检测嵌套对象和数组 → 生成关联子表
	for _, col := range mainTable.Columns {
		if col.IsObject && col.ChildSchema != nil {
			col.ChildSchema.TableName = result.TableName + "_" + col.Name
			col.ChildSchema.Columns = append([]*ColumnInfo{
				{Name: "id", DataType: "BIGINT AUTO_INCREMENT", IsPrimaryKey: true},
				{Name: result.TableName + "_id", DataType: "BIGINT", Nullable: false},
			}, col.ChildSchema.Columns...)
			col.ChildSchema.Indexes = []string{"INDEX idx_" + result.TableName + "_id (" + result.TableName + "_id)"}
			result.Schemas = append(result.Schemas, col.ChildSchema)
		}
		if col.IsArray && col.ChildSchema != nil {
			col.ChildSchema.TableName = result.TableName + "_" + col.Name
			col.ChildSchema.Columns = append([]*ColumnInfo{
				{Name: "id", DataType: "BIGINT AUTO_INCREMENT", IsPrimaryKey: true},
				{Name: result.TableName + "_id", DataType: "BIGINT", Nullable: false},
			}, col.ChildSchema.Columns...)
			col.ChildSchema.Indexes = []string{"INDEX idx_" + result.TableName + "_id (" + result.TableName + "_id)"}
			result.Schemas = append(result.Schemas, col.ChildSchema)
		}
	}

	return result
}

// analyzeRecords 分析一批记录，合并出表结构
func analyzeRecords(records []map[string]any, tableName string) *TableSchema {
	schema := &TableSchema{
		TableName: tableName,
	}
	colMap := make(map[string]*ColumnInfo)

	for _, record := range records {
		for key, val := range record {
			colName := toSnakeCase(key)
			if _, exists := colMap[colName]; !exists {
				colMap[colName] = &ColumnInfo{
					Name:     colName,
					DataType: "VARCHAR(255)", // 默认
				}
			}
			col := colMap[colName]
			updateColumnInfo(col, val)
		}
	}

	// 最终确定类型 & 排序
	for _, col := range colMap {
		finalizeType(col)
		schema.Columns = append(schema.Columns, col)
	}

	// 排序：主键排第一，其余按字母
	sort.Slice(schema.Columns, func(i, j int) bool {
		if schema.Columns[i].IsPrimaryKey {
			return true
		}
		if schema.Columns[j].IsPrimaryKey {
			return false
		}
		return schema.Columns[i].Name < schema.Columns[j].Name
	})

	return schema
}

// updateColumnInfo 根据一个值更新列的统计信息
func updateColumnInfo(col *ColumnInfo, val any) {
	col.SeenCount++
	if val == nil {
		col.NullCount++
		return
	}

	// 采样（最多保留 5 个样本值）
	if len(col.SeenValues) < 5 {
		col.SeenValues = append(col.SeenValues, val)
	}

	switch v := val.(type) {
	case float64:
		if v == float64(int64(v)) {
			// 整数
			if !col.IsPrimaryKey && col.Name == "id" {
				col.IsPrimaryKey = true
				col.DataType = "BIGINT AUTO_INCREMENT"
			}
		}
		// 长度不需要跟踪，数字类型用 DECIMAL

	case string:
		strLen := len(v)
		if strLen > col.MaxLen {
			col.MaxLen = strLen
		}
		// 检测 ID 列
		if (strings.HasSuffix(col.Name, "id") || strings.HasSuffix(col.Name, "_id")) && v != "" {
			if isNumeric(v) {
				col.IsPrimaryKey = strings.EqualFold(col.Name, "id")
			}
		}

	case bool:
		// nothing special

	case map[string]any:
		col.IsObject = true
		if col.ChildSchema == nil {
			col.ChildSchema = &TableSchema{}
		}
		for k, childVal := range v {
			childCol := &ColumnInfo{Name: toSnakeCase(k)}
			updateColumnInfo(childCol, childVal)
			finalizeType(childCol)
			col.ChildSchema.Columns = append(col.ChildSchema.Columns, childCol)
		}

	case []any:
		col.IsArray = true
		if col.ChildSchema == nil && len(v) > 0 {
			if childObj, ok := v[0].(map[string]any); ok {
				childSchema := analyzeRecords([]map[string]any{childObj}, "")
				col.ChildSchema = childSchema
			}
		}
	}
}

// finalizeType 根据统计信息最终确定列的数据类型
func finalizeType(col *ColumnInfo) {
	if col.IsPrimaryKey {
		return // 已经确定了
	}
	if col.IsObject || col.IsArray {
		col.DataType = "——关联子表——" // 不在主表建列
		return
	}
	if col.NullCount == col.SeenCount {
		col.DataType = "VARCHAR(255)"
		col.Nullable = true
		return
	}

	for _, v := range col.SeenValues {
		if v == nil {
			continue
		}
		switch val := v.(type) {
		case float64:
			if val == float64(int64(val)) {
				col.DataType = "BIGINT"
				if strings.HasSuffix(col.Name, "_id") || strings.HasPrefix(col.Name, "is_") {
					col.DataType = "BIGINT"
				}
			} else {
				col.DataType = "DECIMAL(12,2)"
			}
		case string:
			if isDateTime(val) {
				col.DataType = "DATETIME"
			} else if isDate(val) {
				col.DataType = "DATE"
			} else if isTime(val) {
				col.DataType = "TIME"
			} else if isJSON(val) {
				col.DataType = "JSON"
				col.MaxLen = 0
			} else {
				// 动态 VARCHAR：取最大长度 * 2 向上取整到 2 的幂
				maxLen := col.MaxLen
				if maxLen == 0 {
					maxLen = 64
				}
				maxLen = maxLen * 2
				// 向上对齐到 2 的幂
				aligned := 64
				for aligned < maxLen {
					aligned *= 2
				}
				if aligned > 4096 {
					col.DataType = "TEXT"
				} else {
					col.DataType = fmt.Sprintf("VARCHAR(%d)", aligned)
				}
			}
		case bool:
			col.DataType = "TINYINT(1)"
		default:
			col.DataType = "VARCHAR(255)"
		}
		// 只看第一个非 null 值确定类型
		break
	}

	if col.NullCount > 0 {
		col.Nullable = true
	}
}

// ─── 工具函数 ───

var (
	dateTimePatterns = []*regexp.Regexp{
		regexp.MustCompile(`^\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}`),
		regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}`),
		regexp.MustCompile(`^\d{4}/\d{2}/\d{2}\s+\d{2}:\d{2}:\d{2}`),
	}
	datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	timePattern = regexp.MustCompile(`^\d{2}:\d{2}:\d{2}`)
	numericPat  = regexp.MustCompile(`^\d+$`)
)

func isDateTime(s string) bool {
	for _, p := range dateTimePatterns {
		if p.MatchString(s) {
			return true
		}
	}
	// 额外验证
	_, err := time.Parse("2006-01-02 15:04:05", s)
	if err == nil {
		return true
	}
	_, err = time.Parse("2006-01-02T15:04:05Z", s)
	if err == nil {
		return true
	}
	return false
}

func isDate(s string) bool {
	if !datePattern.MatchString(s) {
		return false
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func isTime(s string) bool {
	return timePattern.MatchString(s)
}

func isNumeric(s string) bool {
	return numericPat.MatchString(s)
}

func isJSON(s string) bool {
	s = strings.TrimSpace(s)
	return (strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}")) ||
		(strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]"))
}

// toSnakeCase 驼峰转下划线
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteRune(r + 32) // to lower
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

// extractRecords 从响应 body 中提取 JSON 对象数组
func extractRecords(body []byte) []map[string]any {
	bodyStr := strings.TrimSpace(string(body))
	return tryExtract(bodyStr)
}

func tryExtract(jsonStr string) []map[string]any {
	// 解析 JSON
	var data any
	// 手动按字符解析，避免依赖 encoding/json 对 interface{} 的默认解析
	// 这里使用 fmt.Sscanf 或简单方式
	_ = data

	// 实际使用 encoding/json
	return extractWithJSON(jsonStr)
}

func extractWithJSON(jsonStr string) []map[string]any {
	// 先用通用方式解析
	var result any
	// 用简单的字符串处理判断最外层类型
	jsonStr = strings.TrimSpace(jsonStr)

	// 用 map[string]any + []any 的递归下降解析
	result = parseJSON(jsonStr)
	if result == nil {
		return nil
	}

	// 如果是数组，直接返回
	if arr, ok := result.([]any); ok {
		var records []map[string]any
		for _, item := range arr {
			if m, ok := item.(map[string]any); ok {
				records = append(records, m)
			}
		}
		return records
	}

	// 如果是对象，尝试找 data/list/records/result 等常见 key
	if obj, ok := result.(map[string]any); ok {
		for _, key := range []string{"data", "list", "records", "result", "items", "rows", "content", "results"} {
			if v, exists := obj[key]; exists {
				if arr, ok := v.([]any); ok {
					var records []map[string]any
					for _, item := range arr {
						if m, ok := item.(map[string]any); ok {
							records = append(records, m)
						}
					}
					return records
				}
				// data.list 双层包裹 { "data": { "list": [...] } }
				if nested, ok := v.(map[string]any); ok {
					for _, key2 := range []string{"list", "records", "items", "rows"} {
						if arr, ok := nested[key2].([]any); ok {
							var records []map[string]any
							for _, item := range arr {
								if m, ok := item.(map[string]any); ok {
									records = append(records, m)
								}
							}
							return records
						}
					}
				}
			}
		}
		// 如果对象本身就是一条记录（常见于详情接口）
		return []map[string]any{obj}
	}

	return nil
}

// endpointToTableName 从 URL 路径推断表名
func endpointToTableName(endpoint string) string {
	// /api/user/list → user
	// /admin/product/page → product
	// /api/v1/order/list → order
	parts := strings.Split(endpoint, "/")
	// 找最后一个非动词路径段
	verbs := map[string]bool{"list": true, "page": true, "query": true, "search": true,
		"get": true, "create": true, "update": true, "delete": true, "detail": true, "info": true,
		"select": true, "index": true, "permission": true, "add": true, "edit": true, "save": true}

	for i := len(parts) - 1; i >= 0; i-- {
		part := parts[i]
		if part != "" && !verbs[part] && !strings.Contains(part, ".") {
			// 去掉 api、v1 等前缀段
			if part != "api" && part != "v1" && part != "v2" && part != "v3" && part != "admin" {
				return toSnakeCase(part)
			}
		}
	}
	return "unknown_table"
}
