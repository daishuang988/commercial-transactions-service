package repository

import (
	"fmt"
	"time"
)

var cstLocation *time.Location

func init() {
	if loc, err := time.LoadLocation("Asia/Shanghai"); err == nil {
		cstLocation = loc
	} else {
		cstLocation = time.FixedZone("CST", 8*3600)
	}
}

func CSTLocation() *time.Location { return cstLocation }

const FlashSaleLuaScript = `
	local stock_key = KEYS[1]
	local stream_key = KEYS[2]
	local user_id = ARGV[1]
	local product_id = ARGV[2]
	local price = ARGV[3]
	local now = ARGV[4]

	local current = tonumber(redis.call('GET', stock_key))
	if current == nil or current <= 0 then
	    return {0, 'sold_out'}
	end
	redis.call('DECR', stock_key)

	local order = string.format('{"user_id":"%s","product_id":"%s","price":"%s","time":"%s"}',
	    user_id, product_id, price, now)
	redis.call('XADD', stream_key, '*', 'data', order)

	return {1, 'success'}
	`

func IsFlashSaleTime() bool {
	now := time.Now().In(cstLocation)
	daysStr := getConfig("flash_sale_days")
	if daysStr == "" { return false }
	startStr := getConfig("flash_sale_start")
	if startStr == "" { return false }
	endStr := getConfig("flash_sale_end")
	if endStr == "" { return false }

	startDay, endDay := parseDays(daysStr)
	today := int(now.Weekday())
	if startDay <= endDay {
		if today < startDay || today > endDay { return false }
	} else {
		if today < startDay && today > endDay { return false }
	}

	startH, startM := 10, 0
	endH, endM := 10, 30
	fmt.Sscanf(startStr, "%d:%d", &startH, &startM)
	fmt.Sscanf(endStr, "%d:%d", &endH, &endM)

	startTime := time.Date(now.Year(), now.Month(), now.Day(), startH, startM, 0, 0, cstLocation)
	endTime := time.Date(now.Year(), now.Month(), now.Day(), endH, endM, 0, 0, cstLocation)
	return !now.Before(startTime) && now.Before(endTime)
}

func parseDays(s string) (int, int) {
	var a, b int
	fmt.Sscanf(s, "%d-%d", &a, &b)
	return normalizeDay(a), normalizeDay(b)
}

func normalizeDay(d int) int {
	if d == 7 { return 0 }
	if d < 0 { return 1 }
	if d > 7 { return 6 }
	return d
}

func isWeekday(startDay, endDay int) bool {
	today := int(time.Now().In(cstLocation).Weekday())
	if startDay <= endDay { return today >= startDay && today <= endDay }
	return today >= startDay || today <= endDay
}

func getConfig(key string) string {
	var val string
	DB.Table("system_configs").Select("config_value").Where("config_key = ?", key).Scan(&val)
	return val
}

func GetConfigInt(key string, defaultVal int) int {
	s := getConfig(key)
	if s == "" { return defaultVal }
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

func FlashSaleTimeInfo() map[string]interface{} {
	now := time.Now().In(cstLocation)
	daysStr := getConfig("flash_sale_days")
	startStr := getConfig("flash_sale_start")
	endStr := getConfig("flash_sale_end")

	if daysStr == "" || startStr == "" || endStr == "" {
		return map[string]interface{}{
			"is_open":                  false,
			"in_window":                false,
			"start_time":               "",
			"end_time":                 "",
			"priority_start_time":      "",
			"priority_advance_minutes": 0,
			"weekday_rule":             "未配置",
			"time_rule":                "",
			"server_time":              now.Format("2006-01-02 15:04:05"),
			"is_weekday":               false,
			"config_days":              "",
		}
	}

	inWindow := IsFlashSaleTime()
	startDay, endDay := parseDays(daysStr)

	weekNames := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
	var weekdayRule string
	if (startDay == 0 && endDay == 6) || (startDay == 1 && endDay == 0) {
		weekdayRule = "每天"
	} else if startDay == endDay {
		weekdayRule = weekNames[startDay]
	} else {
		weekdayRule = fmt.Sprintf("%s至%s", weekNames[startDay], weekNames[endDay])
	}

	advanceMin := GetConfigInt("priority_advance_minutes", 0)
	priorityStartTime := ""
	if advanceMin > 0 {
		sh, sm := 0, 0
		fmt.Sscanf(startStr, "%d:%d", &sh, &sm)
		pt := time.Date(now.Year(), now.Month(), now.Day(), sh, sm, 0, 0, cstLocation).
			Add(-time.Duration(advanceMin) * time.Minute)
		priorityStartTime = pt.Format("2006-01-02 15:04:05")
	}

	return map[string]interface{}{
		"is_open":                  inWindow,
		"in_window":                inWindow,
		"start_time":               fmt.Sprintf("%s %s:00", now.Format("2006-01-02"), startStr),
		"end_time":                 fmt.Sprintf("%s %s:00", now.Format("2006-01-02"), endStr),
		"priority_start_time":      priorityStartTime,
		"priority_advance_minutes": advanceMin,
		"weekday_rule":             weekdayRule,
		"time_rule":                fmt.Sprintf("%s-%s", startStr, endStr),
		"server_time":              now.Format("2006-01-02 15:04:05"),
		"is_weekday":               isWeekday(startDay, endDay),
		"config_days":              daysStr,
	}
}
