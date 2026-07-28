package gf

import (
	"gofly/utils/tools/gconv"
	"time"
)

// any数据类型转成int
func Int(i interface{}) int {
	return gconv.Int(i)
}

// any数据类型转成int8
func Int8(i interface{}) int8 {
	return gconv.Int8(i)
}

// any数据类型转成int16
func Int16(i interface{}) int16 {
	return gconv.Int16(i)
}

// any数据类型转成int32
func Int32(i interface{}) int32 {
	return gconv.Int32(i)
}

// any数据类型转成int64
func Int64(i interface{}) int64 {
	return gconv.Int64(i)
}

// any数据类型转成uint
func Uint(i interface{}) uint {
	return gconv.Uint(i)
}

// any数据类型转成uint8
func Uint8(i interface{}) uint8 {
	return gconv.Uint8(i)
}

// any数据类型转成uint16
func Uint16(i interface{}) uint16 {
	return gconv.Uint16(i)
}

// any数据类型转成uint32
func Uint32(i interface{}) uint32 {
	return gconv.Uint32(i)
}

// any数据类型转成uint64
func Uint64(i interface{}) uint64 {
	return gconv.Uint64(i)
}

// any数据类型转成float32
func Float32(i interface{}) float32 {
	return gconv.Float32(i)
}

// any数据类型转成float64
func Float64(i interface{}) float64 {
	return gconv.Float64(i)
}

// any数据类型转成bool
func Bool(i interface{}) bool {
	return gconv.Bool(i)
}

// any数据类型转成string
func String(i interface{}) string {
	return gconv.String(i)
}

// any数据类型转成 []byte
func Bytes(i interface{}) []byte {
	return gconv.Bytes(i)
}

// any数据类型转成 []string 数组
func Strings(i interface{}) []string {
	return gconv.Strings(i)
}

// any数据类型转成 []int 数组
func Ints(i interface{}) []int {
	return gconv.Ints(i)
}

// any数据类型转成 []float64 数组
func Floats(i interface{}) []float64 {
	return gconv.Floats(i)
}

// any数据类型转成 []interface{}  数组
func Interfaces(i interface{}) []interface{} {
	return gconv.Interfaces(i)
}

// []interface{} 数据类型转成 int32 数组
func InterfaceToInt32Slice(interfaceList []interface{}) []int32 {
	int32List := make([]int32, 0, len(interfaceList))
	for _, v := range interfaceList {
		int32List = append(int32List, Int32(v))
	}
	return int32List
}

// []interface{} 数据类型转成 int64 数组
func InterfaceToInt64Slice(interfaceList []interface{}) []int64 {
	int64List := make([]int64, 0, len(interfaceList))
	for _, v := range interfaceList {
		int64List = append(int64List, Int64(v))
	}
	return int64List
}

// interface{} 数据类型转成 int32 数组
func InterfaceToInt32(i interface{}) []int32 {
	return InterfaceToInt32Slice(gconv.Interfaces(i))
}

// interface{} 数据类型转成 int64 数组
func InterfaceToInt64(i interface{}) []int64 {
	return InterfaceToInt64Slice(gconv.Interfaces(i))
}

// StrToTime 把 "2025-12-19 15:04:05" 格式字符串转为 time.Time
// 日期格式不匹配，返回零值 time.Time
func StrToTime(timeStr string) time.Time {
	return gconv.StrToTime(timeStr)
}

// DateTimeToTimestamp 通用时间转时间戳
// 自动支持：2025-12-19 | 2025-12-19 15:04 | 2025-12-19 15:04:05
// 返回：毫秒级时间戳，错误
func DateTimeToTimestamp(timeStr string) (int64, error) {
	return gconv.DateTimeToTimestamp(timeStr)
}

// IsSlice 判断一个值是否为切片/数组
func IsSlice(i interface{}) bool {
	return gconv.IsSlice(i)
}
