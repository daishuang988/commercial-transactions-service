package front

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"commercial-transactions-service/internal/model"
	"commercial-transactions-service/internal/repository"
	"commercial-transactions-service/pkg/app"

	pdfcpu "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/gin-gonic/gin"
)

// UpdateProfile 修改个人信息(头像/昵称) PUT /api/v1/front/user/profile
func UpdateProfile(c *gin.Context) {
	uid := c.GetInt64("user_id")
	var req struct {
		Avatar   string `json:"avatar"`
		Nickname string `json:"nickname"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "参数错误")
		return
	}
	updates := map[string]interface{}{"updated_at": time.Now()}
	if req.Avatar != "" { updates["avatar"] = req.Avatar }
	if req.Nickname != "" { updates["nickname"] = req.Nickname }
	if len(updates) <= 1 {
		app.BadRequest(c, "没有要更新的字段")
		return
	}
	repository.DB.Model(&model.User{}).Where("id = ?", uid).Updates(updates)
	app.OK(c, gin.H{"msg": "更新成功"})
}

// Addresses 收货地址列表 GET /api/v1/front/user/addresses
func Addresses(c *gin.Context) {
	uid := c.GetInt64("user_id")
	var list []map[string]interface{}
	repository.DB.Table("user_addresses").Where("user_id = ?", uid).Order("is_default DESC, id DESC").Find(&list)
	if list == nil { list = []map[string]interface{}{} }
	app.OK(c, list)
}

// AddAddress 添加收货地址 POST /api/v1/front/user/address
func AddAddress(c *gin.Context) {
	uid := c.GetInt64("user_id")
	var req struct {
		Consignee string `json:"consignee" binding:"required"`
		Phone     string `json:"phone" binding:"required"`
		Province  string `json:"province"`
		City      string `json:"city"`
		Area      string `json:"area"`
		Address   string `json:"address" binding:"required"`
		IsDefault int8   `json:"is_default"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "请填写收货人和地址")
		return
	}
	now := time.Now()
	if req.IsDefault == 1 {
		repository.DB.Exec("UPDATE user_addresses SET is_default=0 WHERE user_id=?", uid)
	}
	repository.DB.Exec(
		"INSERT INTO user_addresses (user_id,consignee,phone,province,city,area,address,is_default,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)",
		uid, req.Consignee, req.Phone, req.Province, req.City, req.Area, req.Address, req.IsDefault, now, now)
	app.OK(c, gin.H{"msg": "地址已添加"})
}

// UpdateAddress 编辑收货地址 PUT /api/v1/front/user/address/:id
func UpdateAddress(c *gin.Context) {
	uid := c.GetInt64("user_id")
	id := parseIntParam(c, "id")
	var req struct {
		Consignee string `json:"consignee"`
		Phone     string `json:"phone"`
		Province  string `json:"province"`
		City      string `json:"city"`
		Area      string `json:"area"`
		Address   string `json:"address"`
		IsDefault int8   `json:"is_default"`
	}
	c.ShouldBindJSON(&req)
	now := time.Now()
	if req.IsDefault == 1 {
		repository.DB.Exec("UPDATE user_addresses SET is_default=0 WHERE user_id=?", uid)
	}
	updates := map[string]interface{}{"updated_at": now}
	if req.Consignee != "" { updates["consignee"] = req.Consignee }
	if req.Phone != "" { updates["phone"] = req.Phone }
	if req.Province != "" { updates["province"] = req.Province }
	if req.City != "" { updates["city"] = req.City }
	if req.Area != "" { updates["area"] = req.Area }
	if req.Address != "" { updates["address"] = req.Address }
	updates["is_default"] = req.IsDefault
	repository.DB.Table("user_addresses").Where("id=? AND user_id=?", id, uid).Updates(updates)
	app.OK(c, gin.H{"msg": "地址已更新"})
}

// DeleteAddress 删除收货地址 DELETE /api/v1/front/user/address/:id
func DeleteAddress(c *gin.Context) {
	uid := c.GetInt64("user_id")
	id := parseIntParam(c, "id")
	if repository.DB.Table("user_addresses").Where("id=? AND user_id=?", id, uid).Delete(nil).RowsAffected == 0 {
		app.NotFound(c, "地址不存在")
		return
	}
	app.OK(c, nil)
}

// ContractStatus 合同签署状态 GET /api/v1/front/user/contract-status
func ContractStatus(c *gin.Context) {
	uid := c.GetInt64("user_id")
	var signature string
	repository.DB.Table("user_contracts").Select("contract_path").Where("user_id=? AND contract_path IS NOT NULL AND contract_path != ''", uid).Order("id DESC").Scan(&signature)
	var signTime string
	if signature != "" {
		repository.DB.Table("user_contracts").Select("created_at").Where("user_id=? AND contract_path IS NOT NULL AND contract_path != ''", uid).Order("id DESC").Scan(&signTime)
	}
	app.OK(c, gin.H{"signed": signature != "", "sign_time": signTime, "signature": signature})
}

// SignContract 签署合同 POST /api/v1/front/user/contract/sign
func SignContract(c *gin.Context) {
	uid := c.GetInt64("user_id")
	var req struct{ Signature string `json:"signature" binding:"required"` }
	if err := c.ShouldBindJSON(&req); err != nil {
		app.BadRequest(c, "请提供签名")
		return
	}

	// 检查是否已签过
	var existCount int64
	repository.DB.Table("user_contracts").Where("user_id=? AND contract_path IS NOT NULL AND contract_path != ''", uid).Count(&existCount)
	if existCount > 0 {
		app.BadRequest(c, "合同已签署，无需重复操作")
		return
	}

	now := time.Now()
	dateStr := now.Format("2006-01-02")

	// 取合同PDF URL
	var pdfURL string
	repository.DB.Table("system_configs").Select("config_value").Where("config_key=?", "contract_content").Scan(&pdfURL)
	// 去掉可能的HTML标签包裹
	pdfURL = strings.TrimSpace(pdfURL)
	pdfURL = strings.TrimPrefix(pdfURL, "<p>")
	pdfURL = strings.TrimSuffix(pdfURL, "</p>")

	// 生成签名页 → headless转PDF → pdfcpu合并到合同末尾
	dir := filepath.Join(".", "upload", "sign_contract")
	os.MkdirAll(dir, 0755)
	filename := fmt.Sprintf("%d_%s", uid, now.Format("20060102150405"))
	signHTML := filepath.Join(dir, filename+"_sign.html")
	signPDF := filepath.Join(dir, filename+"_sign.pdf")
	outPDF := filepath.Join(dir, filename+".pdf")

	html := fmt.Sprintf(`<!DOCTYPE html><html><head><meta charset="UTF-8"><style>
@page{size:615pt 870pt;margin:0} body{margin:0;padding:0}
.wrap{position:absolute;left:40px;top:40px;text-align:left}
.wrap p{margin:14px 0;font-size:22px;font-family:"WenQuanYi Micro Hei","Microsoft YaHei",sans-serif}
.wrap img{max-width:200px;max-height:80px;vertical-align:middle}
</style></head><body>
<div class="wrap">
<p>用户签名：<img src="%s" alt="签名"/></p>
<p>签署日期：%s</p>
</div>
</body></html>`, req.Signature, dateStr)
	os.WriteFile(signHTML, []byte(html), 0644)

	// headless 签名页 → PDF
	absSign, _ := filepath.Abs(signPDF)
	absHTML, _ := filepath.Abs(signHTML)

	// 优先 wkhtmltopdf（轻量，无 snap AppArmor 限制）
	wk := findTool("wkhtmltopdf", "/usr/bin/wkhtmltopdf", "/usr/local/bin/wkhtmltopdf")
	if wk != "" {
		if err := exec.Command(wk, "--page-width", "615pt", "--page-height", "870pt",
			"--margin-top", "0", "--margin-bottom", "0",
			"--margin-left", "0", "--margin-right", "0", absHTML, absSign).Run(); err != nil {
			log.Printf("[SignContract] wkhtmltopdf 失败: %v", err)
		}
	} else if browser := findBrowser(); browser != "" {
		signURL := fmt.Sprintf("http://localhost:8080/upload/sign_contract/%s_sign.html", filename)
		if err := exec.Command(browser, "--headless", "--disable-gpu", "--no-sandbox",
			"--no-pdf-header-footer",
			fmt.Sprintf("--print-to-pdf=%s", absSign), signURL).Run(); err != nil {
			log.Printf("[SignContract] browser PDF 失败: %v", err)
		}
	} else {
		log.Printf("[SignContract] 未找到 HTML→PDF 工具，签名页无法生成")
	}

	// pdfcpu 合并：原合同 + 签名页
	absOut, _ := filepath.Abs(outPDF)
	origFile := filepath.Join(".", pdfURL)
	if _, err := os.Stat(signPDF); err == nil {
		pdfcpu.MergeCreateFile([]string{origFile, signPDF}, absOut, false, nil)
	} else {
		os.WriteFile(outPDF, mustRead(origFile), 0644)
	}

	contractPdfURL := "/upload/sign_contract/" + filename + ".pdf"
	repository.DB.Exec("INSERT INTO user_contracts (user_id,contract_path,created_at) VALUES(?,?,?)", uid, contractPdfURL, now)
	repository.DB.Model(&model.User{}).Where("id=?", uid).Update("contract", contractPdfURL)

	app.OK(c, gin.H{
		"msg":       "合同签署成功",
		"sign_time": dateStr,
		"contract":  contractPdfURL,
	})
}

// findTool 按顺序查找可执行文件，返回第一个存在的路径
func findTool(names ...string) string {
	for _, n := range names {
		if _, err := os.Stat(n); err == nil { return n }
	}
	return ""
}

func findBrowser() string {
	for _, p := range []string{
		// Linux
		`/usr/bin/chromium-browser`,
		`/usr/bin/chromium`,
		`/usr/bin/google-chrome`,
		`/usr/bin/google-chrome-stable`,
		// macOS
		`/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`,
		`/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge`,
		// Windows
		`C:\Program Files\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Google\Chrome\Application\chrome.exe`,
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		`C:\Program Files\Microsoft\Edge\Application\msedge.exe`,
	} { if _, err := os.Stat(p); err == nil { return p } }
	return ""
}
func mustRead(path string) []byte { d, _ := os.ReadFile(path); return d }

// TradeRules 交易规则配置 GET /api/v1/front/config/trade-rules
func TradeRules(c *gin.Context) {
	app.OK(c, gin.H{
		"resell_rate":          repository.GetConfigStr("resell_rate"),
		"static_income_rate":   repository.GetConfigStr("static_income_rate"),
		"order_reward_rate":    repository.GetConfigStr("order_reward_rate"),
		"direct_referral_rate": repository.GetConfigStr("direct_referral_rate"),
		"store_manager_rate":   repository.GetConfigStr("store_manager_rate"),
		"resell_deadline":      repository.GetConfigStr("resell_deadline"),
		"resell_product_id":    repository.GetConfigStr("resell_product_id"),
		"flash_sale_product":   repository.GetConfigStr("flash_sale_product"),
	})
}
