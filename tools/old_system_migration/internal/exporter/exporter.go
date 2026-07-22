package exporter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"commercial-transactions-service/internal/analyzer"
)

// CapturedAPI 一次 API 调用的完整记录
type CapturedAPI struct {
	URL         string `json:"url"`
	Method      string `json:"method"`
	RequestBody string `json:"request_body,omitempty"`
	StatusCode  int    `json:"status_code"`
	Body        []byte `json:"-"`
	BodyFile    string `json:"body_file"`
}

// ExportResult 导出结果
type ExportResult struct {
	SchemasDir    string
	ReportFile    string
	EndpointsFile string
	DataDir       string
}

// ExportAll 导出所有产物
func ExportAll(outputDir string, results []*analyzer.AnalysisResult, capturedAPIs []*CapturedAPI) (*ExportResult, error) {
	os.MkdirAll(outputDir, 0755)
	dataDir := filepath.Join(outputDir, "data")
	schemasDir := filepath.Join(outputDir, "schemas")
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(schemasDir, 0755)

	result := &ExportResult{
		SchemasDir:    schemasDir,
		ReportFile:    filepath.Join(outputDir, "analysis_report.md"),
		EndpointsFile: filepath.Join(outputDir, "endpoints.json"),
		DataDir:       dataDir,
	}

	// 1. 保存 raw JSON 数据
	for _, api := range capturedAPIs {
		if len(api.Body) == 0 {
			continue
		}
		// 文件名：把 URL 中的特殊字符替换掉
		safeName := sanitizeFilename(api.URL)
		dataFile := filepath.Join(dataDir, safeName+".json")
		os.WriteFile(dataFile, api.Body, 0644)
		api.BodyFile = dataFile
	}

	// 2. 保存分析结果 → endpoints.json
	endpointsData, _ := json.MarshalIndent(struct {
		Endpoints []*CapturedAPI          `json:"endpoints"`
		Tables    []*analyzer.AnalysisResult `json:"inferred_tables"`
	}{
		Endpoints: capturedAPIs,
		Tables:    results,
	}, "", "  ")
	os.WriteFile(result.EndpointsFile, endpointsData, 0644)

	// 3. 保存各表 schema 的 JSON
	for _, r := range results {
		for _, schema := range r.Schemas {
			schemaFile := filepath.Join(schemasDir, schema.TableName+".json")
			schemaJSON, _ := json.MarshalIndent(schema, "", "  ")
			os.WriteFile(schemaFile, schemaJSON, 0644)
		}
	}

	return result, nil
}

// GenerateInsertSQL 根据捕获的 API 数据生成 INSERT 语句
func GenerateInsertSQL(results []*analyzer.AnalysisResult) string {
	var sb strings.Builder
	sb.WriteString("-- ============================================\n")
	sb.WriteString("-- 数据导入 SQL（基于 API 响应）\n")
	sb.WriteString("-- ============================================\n\n")

	for _, r := range results {
		for _, schema := range r.Schemas {
			if schema.TableName == "" || schema.TableName == "unknown_table" {
				continue
			}
			sb.WriteString(fmt.Sprintf("-- 表: %s (来源: %s %s)\n", schema.TableName, r.Method, r.Endpoint))
			sb.WriteString(fmt.Sprintf("-- 列: "))
			var colNames []string
			for _, col := range schema.Columns {
				if col.DataType != "——关联子表——" {
					colNames = append(colNames, col.Name)
				}
			}
			sb.WriteString(strings.Join(colNames, ", "))
			sb.WriteString("\n")
			sb.WriteString("-- 请根据 output/data/ 目录下的 JSON 文件手动构造 INSERT 或使用数据导入工具\n\n")
		}
	}

	return sb.String()
}

func sanitizeFilename(url string) string {
	s := url
	s = strings.ReplaceAll(s, "https://", "")
	s = strings.ReplaceAll(s, "http://", "")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "?", "_")
	s = strings.ReplaceAll(s, "&", "_")
	s = strings.ReplaceAll(s, "=", "_")
	s = strings.ReplaceAll(s, ":", "_")
	if len(s) > 100 {
		s = s[:100]
	}
	return s
}
