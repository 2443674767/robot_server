// ===========================
// 封装应用缓存功能
// 自适应缓存 有 Redis 就用 Redis（分布式），没 Redis 就自动降级切换为内存缓存（单机）
// ===========================
package gcache

import (
	"fmt"
	"gofly/utils/tools/gcfg"
	"gofly/utils/tools/gconv"
	"gofly/utils/tools/gctx"
	"gofly/utils/tools/instance"
	"time"
)

// 全局缓存实例（自适应：Redis/内存）
var (
	// Use ICache
	ctx = gctx.New()
)

const (
	defaultGroupName = "default" // Default configuration group name.
)

// ICache 缓存接口（统一API）
type ICache interface {
	Set(key string, value interface{}, expiration time.Duration) error
	Get(key string) (string, error)
	Del(key string) error
}

// 初始化：自动选择 Redis/内存
func InitCache() (ICache, error) {
	cacheConf, err := gcfg.Instance().Get(ctx, "redis.cache")
	if err == nil {
		cacheConf_arr := gconv.Map(cacheConf)
		password := ""
		if pass, ok := cacheConf_arr["pass"]; ok {
			password = gconv.String(pass)
		}
		// 尝试连接 Redis
		redisCache, err := NewRedisCache(gconv.String(cacheConf_arr["address"]), password, gconv.Int(cacheConf_arr["db"]))
		if err == nil {
			//fmt.Errorf("缓存启动成功：使用 Redis")
			return redisCache, nil
		}
	}
	// Redis 连接失败，自动降级为内存缓存
	ristrettoCache, err := NewMemoryCache()
	if err != nil {
		return nil, fmt.Errorf("自动降级为内存缓存失败:%v", err.Error())
	}
	//"Redis 未连接，自动降级使用：内存缓存"
	return ristrettoCache, nil
}

func New(name ...string) ICache {
	var group = defaultGroupName
	if len(name) > 0 && name[0] != "" {
		group = name[0]
	}
	instanceKey := fmt.Sprintf("%s.%s", "gcache", group)
	cache := instance.GetOrSetFuncLock(instanceKey, func() interface{} {
		if cache, err := InitCache(); err == nil {
			return cache
		} else {
			// If panics, often because it does not find its configuration for given group.
			panic(err)
		}
	})
	if cache != nil {
		return cache.(ICache)
	}
	return nil
}

// 清除缓存实例对象
func Clear(name ...string) {
	var group = defaultGroupName
	if len(name) > 0 {
		group = name[0]
	}
	instance.ClearOne(fmt.Sprintf("%s.%s", "gcache", group))
}
