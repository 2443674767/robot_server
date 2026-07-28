// ======================================
// Ristretto 高性能内存缓存管理库
// ======================================
package gcache

import (
	"errors"
	"fmt"
	"time"

	"github.com/dgraph-io/ristretto"
)

// 内存缓存实现
type MemoryCache struct {
	cache *ristretto.Cache
}

// 初始化
func NewMemoryCache() (ICache, error) {
	// 生产级配置（可根据服务器调整）
	cache, err := ristretto.NewCache(&ristretto.Config{
		NumCounters: 1000000, // 100w 计数器，适合绝大多数项目
		MaxCost:     1 << 28, // 最大内存 256MB（1<<28 = 268,435,456）
		BufferItems: 64,      // 内部缓冲，固定64即可
	})
	if err != nil {
		return nil, fmt.Errorf("ristretto init failed:%v ", err.Error())
	}
	return &MemoryCache{cache: cache}, nil
}

// 设置内容
func (m *MemoryCache) Set(key string, value interface{}, exp time.Duration) error {
	// Ristretto 单位：秒
	m.cache.SetWithTTL(key, value, 1, exp)
	m.cache.Wait()
	return nil
}

// 获取内容
func (m *MemoryCache) Get(key string) (string, error) {
	val, ok := m.cache.Get(key)
	if !ok {
		return "", errors.New("key not found")
	}
	return val.(string), nil
}

// 删除
func (m *MemoryCache) Del(key string) error {
	m.cache.Del(key)
	return nil
}
