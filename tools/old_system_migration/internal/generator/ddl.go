package generator

import (
	"fmt"
	"strings"

	"commercial-transactions-service/internal/analyzer"
)

// GenerateDDL 根据分析结果生成 MySQL CREATE TABLE 语句
func GenerateDDL(results []*analyzer.AnalysisResult) string {
	var sb strings.Builder
	sb.WriteString("-- ============================================\n")
	sb.WriteString("-- 自动生成的数据库 DDL（基于 API 响应推断）\n")
	sb.WriteString(fmt.Sprintf("-- 数据源: %d 个接口\n", len(results)))
	sb.WriteString("-- 注意: 这是推断结果，请人工确认后执行\n")
	sb.WriteString("-- ============================================\n\n")
	sb.WriteString("CREATE DATABASE IF NOT EXISTS migrated_db\n")
	sb.WriteString("  DEFAULT CHARACTER SET utf8mb4\n")
	sb.WriteString("  COLLATE utf8mb4_unicode_ci;\n\n")
	sb.WriteString("USE migrated_db;\n\n")

	// 去重：同一个表名可能来自多个接口
	seenTables := make(map[string]bool)
	ddlStatements := make(map[string]string)

	for _, result := range results {
		for _, schema := range result.Schemas {
			tableName := schema.TableName
			if tableName == "" || tableName == "unknown_table" {
				continue
			}
			// 如果来自不同接口但表名相同，合并（保留列最多的那个）
			existing, exists := ddlStatements[tableName]
			newDDL := buildCreateTable(schema)
			if !exists || len(newDDL) > len(existing) {
				ddlStatements[tableName] = newDDL
			}
			seenTables[tableName] = true
		}
	}

	// 按表名排序输出
	var tableNames []string
	for name := range ddlStatements {
		tableNames = append(tableNames, name)
	}
	// sort not needed, map iteration is fine for small sets
	for _, name := range tableNames {
		sb.WriteString(ddlStatements[name])
		sb.WriteString("\n")
	}

	return sb.String()
}

func buildCreateTable(schema *analyzer.TableSchema) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("DROP TABLE IF EXISTS `%s`;\n", schema.TableName))
	sb.WriteString(fmt.Sprintf("CREATE TABLE `%s` (\n", schema.TableName))

	var colDefs []string
	var pkCol string

	for _, col := range schema.Columns {
		// 跳过关联子表的列（用 -- 注释标记）
		if col.DataType == "——关联子表——" {
			sb.WriteString(fmt.Sprintf("  -- `%s` → 已拆分为关联表: `%s`\n",
				col.Name, schema.TableName+"_"+col.Name))
			continue
		}

		def := buildColumnDef(col)
		colDefs = append(colDefs, "  "+def)

		if col.IsPrimaryKey {
			pkCol = col.Name
		}
	}

	sb.WriteString(strings.Join(colDefs, ",\n"))

	// 主键
	if pkCol != "" {
		sb.WriteString(fmt.Sprintf(",\n  PRIMARY KEY (`%s`)", pkCol))
	}

	// 索引
	for _, idx := range schema.Indexes {
		sb.WriteString(",\n  " + idx)
	}

	sb.WriteString("\n)")
	sb.WriteString(" ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci")
	sb.WriteString(fmt.Sprintf(" COMMENT='从 API 推断: %s';\n", schema.TableName))

	return sb.String()
}

func buildColumnDef(col *analyzer.ColumnInfo) string {
	parts := []string{"`" + col.Name + "`", col.DataType}

	if !col.Nullable {
		parts = append(parts, "NOT NULL")
	} else {
		parts = append(parts, "DEFAULT NULL")
	}

	if col.IsPrimaryKey && !strings.Contains(col.DataType, "AUTO_INCREMENT") {
		// 主键标记(在外面统一处理)
	}

	// 注释：展示采样值帮助确认
	var commentParts []string
	if len(col.SeenValues) > 0 {
		for _, v := range col.SeenValues {
			if v != nil {
				commentParts = append(commentParts, fmt.Sprintf("%v", v))
			}
		}
	}
	if col.NullCount > 0 {
		commentParts = append(commentParts, fmt.Sprintf("(%d次null)", col.NullCount))
	}
	if len(commentParts) > 0 {
		// 截断过长的注释
		comment := strings.Join(commentParts, ", ")
		if len(comment) > 100 {
			comment = comment[:100] + "..."
		}
		parts = append(parts, fmt.Sprintf("COMMENT '%s'", strings.ReplaceAll(comment, "'", "''")))
	}

	return strings.Join(parts, " ")
}

// GenerateReport 生成分析报告（Markdown 格式）
func GenerateReport(results []*analyzer.AnalysisResult) string {
	var sb strings.Builder
	sb.WriteString("# API 数据库结构分析报告\n\n")
	sb.WriteString(fmt.Sprintf("分析时间: 自动生成\n"))
	sb.WriteString(fmt.Sprintf("接口数量: %d\n\n", len(results)))
	sb.WriteString("---\n\n")

	for _, r := range results {
		sb.WriteString(fmt.Sprintf("## `%s %s`\n\n", r.Method, r.Endpoint))
		sb.WriteString(fmt.Sprintf("- 推断表名: `%s`\n", r.TableName))
		sb.WriteString(fmt.Sprintf("- 采样数量: %d 条记录\n\n", r.SampleCount))

		for _, schema := range r.Schemas {
			sb.WriteString(fmt.Sprintf("### 表: `%s` (%d 列)\n\n", schema.TableName, len(schema.Columns)))
			sb.WriteString("| 列名 | 类型 | 可空 | 采样值 |\n")
			sb.WriteString("|------|------|------|--------|\n")
			for _, col := range schema.Columns {
				nullable := "NOT NULL"
				if col.Nullable {
					nullable = "NULL"
				}
				samples := ""
				for _, v := range col.SeenValues {
					if v != nil {
						samples += fmt.Sprintf("`%v` ", v)
					}
				}
				sb.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n",
					col.Name, col.DataType, nullable, samples))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("---\n\n")
	}
	return sb.String()
}
