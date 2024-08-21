package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	Client *redis.Client
}

var redisClient *redis.Client

func InitRedis(url string) {
	// define redis options
	opt, err := redis.ParseURL(url)
	if err != nil {
		panic(err)
	}
	opt.MaxRetries = 3
	opt.MaxIdleConns = 20
	opt.MinIdleConns = 10
	opt.MaxActiveConns = 80
	opt.ReadTimeout = time.Duration(5) * time.Second
	opt.WriteTimeout = time.Duration(5) * time.Second
	opt.PoolSize = 80

	redisClient = redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(3)*time.Second)
	defer cancel()
	if _, err := redisClient.Ping(ctx).Result(); err != nil {
		panic(err)
	}
}

func RedisClient() *Redis {
	if redisClient == nil {
		panic("redis client is nil")
	}
	return &Redis{Client: redisClient}
}

func (r *Redis) Exists(key string) bool {
	return r.Client.Exists(context.Background(), key).Val() == 1
}

func (r *Redis) Stats() *redis.PoolStats {
	return r.Client.PoolStats()
}

func (r *Redis) Get(key string) (string, error) {
	if !r.Exists(key) {
		return "", fmt.Errorf("key not found")
	}
	return r.Client.Get(context.TODO(), key).Result()
}

func (r *Redis) Set(key string, value any, exp int64) error {
	if r.Client == nil {
		return fmt.Errorf("redis client is nil")
	}
	return r.Client.Set(context.TODO(), key, value, time.Duration(exp)).Err()
}

func (r *Redis) Del(key string) error {
	if r.Client == nil {
		return fmt.Errorf("redis client is nil")
	}
	return r.Client.Del(context.TODO(), key).Err()
}

func (r *Redis) LLen(key string) int64 {
	if r.Client == nil {
		return 0
	}
	return r.Client.LLen(context.TODO(), key).Val()
}

func (r *Redis) LRange(key string, start, stop int64) ([]string, error) {
	if r.Client == nil {
		return nil, fmt.Errorf("redis client is nil")
	}
	return r.Client.LRange(context.TODO(), key, start, stop).Result()
}

func (r *Redis) SAdd(key string, members ...string) error {
	if r.Client == nil {
		return fmt.Errorf("redis client is nil")
	}
	return r.Client.SAdd(context.TODO(), key, members).Err()
}

func (r *Redis) Sismembers(key string, member string) (bool, error) {
	if r.Client == nil {
		return false, fmt.Errorf("redis client is nil")
	}
	return r.Client.SIsMember(context.TODO(), key, member).Result()
}

func (r *Redis) SMembers(key string, members ...string) ([]string, error) {
	if r.Client == nil {
		return nil, fmt.Errorf("redis client is nil")
	}
	return r.Client.SMembers(context.TODO(), key).Result()
}

func (r *Redis) SRem(key string, members ...string) error {
	if r.Client == nil {
		return fmt.Errorf("redis client is nil")
	}
	return r.Client.SRem(context.TODO(), key, members).Err()
}

func (r *Redis) LPush(key string, values ...string) error {
	if r.Client == nil {
		return fmt.Errorf("redis client is nil")
	}
	return r.Client.LPush(context.TODO(), key, values).Err()
}
func (r *Redis) RPop(key string) (string, error) {
	if r.Client == nil {
		return "", fmt.Errorf("redis client is nil")
	}
	return r.Client.RPop(context.TODO(), key).Result()
}

func (r *Redis) RPush(key string, values ...string) error {
	if r.Client == nil {
		return fmt.Errorf("redis client is nil")
	}
	return r.Client.LPush(context.TODO(), key, values).Err()
}
func (r *Redis) LPop(key string) (string, error) {
	if r.Client == nil {
		return "", fmt.Errorf("redis client is nil")
	}
	return r.Client.LPop(context.TODO(), key).Result()
}
func (r *Redis) LRem(key, value string, size int64) error {
	if r.Client == nil {
		return fmt.Errorf("redis client is nil")
	}
	return r.Client.LRem(context.TODO(), key, size, value).Err()
}
