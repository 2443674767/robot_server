package auth

import (
	"encoding/json"
	"gofly/utils/tools/gcache"
	"time"
)

// 设置缓存
func setCache(key string, value *TokenData) error {
	str, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return gcache.New().Set(key, string(str), time.Duration(TokenTimeout+MaxRefresh)*time.Second)
}

// 获取缓存值
func getCache(key string) (tData *TokenData, err error) {
	var result string
	result, err = gcache.New().Get(key)
	if err != nil {
		return
	}
	if result != "" {
		err = json.Unmarshal([]byte(result), &tData)
	}
	return
}

// 删除
func removeCache(key string) (err error) {
	err = gcache.New().Del(key)
	return
}

// 设置字符串缓存
func setStrCache(key, value string) error {
	return gcache.New().Set(key, value, time.Duration(TokenTimeout+MaxRefresh)*time.Second)
}

// 获取字符串缓存
func getStrCache(key string) (string, error) {
	return gcache.New().Get(key)
}
