package gcache

import (
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis 缓存实现
type RedisCache struct {
	client *redis.Client
}

func NewRedisCache(addr, password string, db int) (ICache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})
	// 测试连接
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}
	return &RedisCache{client: client}, nil
}

func (r *RedisCache) Set(key string, value interface{}, exp time.Duration) error {
	return r.client.Set(ctx, key, value, exp).Err()
}

func (r *RedisCache) Get(key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}

func (r *RedisCache) Del(key string) error {
	return r.client.Del(ctx).Err()
}
