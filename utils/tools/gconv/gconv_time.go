package gconv

import (
	"errors"
	"time"
)

// 固定时间模板
const (
	DateFormat     = "2006-01-02"          // 纯日期
	DateTimeMinute = "2006-01-02 15:04"    // 日期+时分
	DateTimeFormat = "2006-01-02 15:04:05" // 日期+时分秒
)

// StrToTime 把 "2025-12-19 15:04:05" 格式字符串转为 time.Time
// 日期格式不匹配，返回零值 time.Time
func StrToTime(timeStr string) time.Time {
	var t time.Time
	var err error
	t, err = time.ParseInLocation(DateTimeFormat, timeStr, time.Local)
	if err == nil {
		return t
	}
	t, err = time.ParseInLocation(DateTimeMinute, timeStr, time.Local)
	if err == nil {
		return t
	}
	t, err = time.ParseInLocation(DateFormat, timeStr, time.Local)
	if err == nil {
		return t
	}
	return time.Time{}
}

// DateTimeToTimestamp 通用时间转时间戳
// 自动支持：2025-12-19 | 2025-12-19 15:04 | 2025-12-19 15:04:05
// 返回：毫秒级时间戳，错误
func DateTimeToTimestamp(timeStr string) (int64, error) {
	var t time.Time
	var err error

	t, err = time.ParseInLocation(DateTimeFormat, timeStr, time.Local)
	if err == nil {
		return t.UnixMilli(), nil
	}

	t, err = time.ParseInLocation(DateTimeMinute, timeStr, time.Local)
	if err == nil {
		return t.UnixMilli(), nil
	}

	t, err = time.ParseInLocation(DateFormat, timeStr, time.Local)
	if err == nil {
		return t.UnixMilli(), nil
	}
	return 0, errors.New("Unsupported time formats are only supported as follows: 2025-12-19 | 2025-12-19 15:04 | 2025-12-19 15:04:05")
}
