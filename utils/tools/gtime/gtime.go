package gtime

import (
	"fmt"
	"reflect"
	"strings"
	"time"
)

// 统一转为CST东八区，中国时区 CST +08:00
var cstZone = time.FixedZone("CST", 8*3600)

// Date函数，日期时间格式化时间
// format日期格式，支持：Y m d H i s 例如：Y-m-d H:i:s
// timestamp可选 秒级时间戳，不传则使用当前时间
func Date(format string, timestamp ...int64) string {
	if format == "" {
		format = "Y-m-d H:i:s"
	}
	var t time.Time
	if len(timestamp) > 0 && timestamp[0] > 0 {
		// 时间戳转 CST 时区时间
		t = time.Unix(timestamp[0], 0).In(cstZone)
	} else {
		// 当前 CST 时区时间
		t = time.Now().In(cstZone)
	}
	return formatDate(format, t)
}

// DateFromTime函数是对time.Time 类型进行格式化（CST时区）
// format日期格式，支持：Y m d H i s 例如：Y-m-d H:i:s
// t是time.Time类型时间
func DateFromTime(format string, t time.Time) string {
	cstTime := t.In(cstZone)
	return formatDate(format, cstTime)
}

// formatDate 内部方法：PHP格式字符映射为Go格式
func formatDate(format string, t time.Time) string {
	replacer := strings.NewReplacer(
		"Y", "2006", // 4位年
		"m", "01", // 2位月
		"d", "02", // 2位日
		"H", "15", // 24小时制
		"i", "04", // 分钟
		"s", "05", // 秒
	)
	goFormat := replacer.Replace(format)
	return t.Format(goFormat)
}

// InterfaceToTime 把 interface{} 安全转 time.Time
func InterfaceToTime(v interface{}) (time.Time, error) {
	switch val := v.(type) {
	case time.Time:
		return val, nil
	case string:
		// 常用格式
		layouts := []string{
			time.RFC3339,
			"2006-01-02 15:04:05",
			"2006-01-02",
		}
		for _, layout := range layouts {
			if t, err := time.Parse(layout, val); err == nil {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("无法解析字符串时间: %s", val)
	case int, int64:
		ts := reflect.ValueOf(val).Int()
		return time.Unix(ts, 0), nil
	default:
		return time.Time{}, fmt.Errorf("不支持的类型: %T", v)
	}
}

// 返回当前日期时间，支持设置Y-m-d H:i:s输出格式
func Datetime(format ...string) string {
	formatStr := ""
	if len(format) > 0 && format[0] != "" {
		formatStr = format[0]
	}
	return Date(formatStr)
}
