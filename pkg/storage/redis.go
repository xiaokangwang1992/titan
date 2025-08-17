package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v9"
	goredislib "github.com/redis/go-redis/v9"
)

type Redis struct {
	client *goredislib.Client
	rs     *redsync.Redsync
	mutexs sync.Map
}

var (
	redis *Redis
	once  sync.Once
)

func InitRedis(url string) {
	once.Do(func() {
		// define redis options
		opt, err := goredislib.ParseURL(url)
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

		redisClient := goredislib.NewClient(opt)
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(3)*time.Second)
		defer cancel()
		if _, err := redisClient.Ping(ctx).Result(); err != nil {
			panic(err)
		}

		rs := redsync.New(goredis.NewPool(redisClient))
		mutex := rs.NewMutex("gmicloud-redis-lock")
		if err := mutex.Lock(); err != nil {
			panic(err)
		}
		mutex.Unlock()

		redis = &Redis{client: redisClient, rs: rs, mutexs: sync.Map{}}
	})
}

func RedisClient() *Redis {
	if redis == nil {
		panic("redis client is nil")
	}
	return redis
}

func (r *Redis) GetRedisOptions() *goredislib.Options {
	if r.client == nil {
		panic("redis client is nil")
	}
	return r.client.Options()
}

func (r *Redis) Exists(key string) bool {
	return r.client.Exists(context.Background(), key).Val() == 1
}

func (r *Redis) Stats() *goredislib.PoolStats {
	return r.client.PoolStats()
}

func (r *Redis) Get(key string) (string, error) {
	if !r.Exists(key) {
		return "", fmt.Errorf("key not found")
	}
	return r.client.Get(context.TODO(), key).Result()
}

func (r *Redis) Set(key string, value any, exp int64) error {
	if r.client == nil {
		return fmt.Errorf("redis client is nil")
	}
	return r.client.Set(context.TODO(), key, value, time.Duration(exp)).Err()
}

func (r *Redis) Del(key string) error {
	if r.client == nil {
		return fmt.Errorf("redis client is nil")
	}
	return r.client.Del(context.TODO(), key).Err()
}

func (r *Redis) LLen(key string) int64 {
	if r.client == nil {
		return 0
	}
	return r.client.LLen(context.TODO(), key).Val()
}

func (r *Redis) LRange(key string, start, stop int64) ([]string, error) {
	if r.client == nil {
		return nil, fmt.Errorf("redis client is nil")
	}
	return r.client.LRange(context.TODO(), key, start, stop).Result()
}

func (r *Redis) SAdd(key string, members ...string) error {
	if r.client == nil {
		return fmt.Errorf("redis client is nil")
	}
	return r.client.SAdd(context.TODO(), key, members).Err()
}

func (r *Redis) Sismembers(key string, member string) (bool, error) {
	if r.client == nil {
		return false, fmt.Errorf("redis client is nil")
	}
	return r.client.SIsMember(context.TODO(), key, member).Result()
}

func (r *Redis) SMembers(key string, members ...string) ([]string, error) {
	if r.client == nil {
		return nil, fmt.Errorf("redis client is nil")
	}
	return r.client.SMembers(context.TODO(), key).Result()
}

func (r *Redis) SRem(key string, members ...string) error {
	if r.client == nil {
		return fmt.Errorf("redis client is nil")
	}
	return r.client.SRem(context.TODO(), key, members).Err()
}

func (r *Redis) LPush(key string, values ...string) error {
	if r.client == nil {
		return fmt.Errorf("redis client is nil")
	}
	return r.client.LPush(context.TODO(), key, values).Err()
}
func (r *Redis) RPop(key string) (string, error) {
	if r.client == nil {
		return "", fmt.Errorf("redis client is nil")
	}
	return r.client.RPop(context.TODO(), key).Result()
}

func (r *Redis) RPush(key string, values ...string) error {
	if r.client == nil {
		return fmt.Errorf("redis client is nil")
	}
	return r.client.LPush(context.TODO(), key, values).Err()
}
func (r *Redis) LPop(key string) (string, error) {
	if r.client == nil {
		return "", fmt.Errorf("redis client is nil")
	}
	return r.client.LPop(context.TODO(), key).Result()
}
func (r *Redis) LRem(key, value string, size int64) error {
	if r.client == nil {
		return fmt.Errorf("redis client is nil")
	}
	return r.client.LRem(context.TODO(), key, size, value).Err()
}

func (r *Redis) ZAdd(key string, members ...goredislib.Z) error {
	if r.client == nil {
		return fmt.Errorf("redis client is nil")
	}
	return r.client.ZAdd(context.TODO(), key, members...).Err()
}

func (r *Redis) ZRange(key string, start, stop int64) ([]string, error) {
	if r.client == nil {
		return nil, fmt.Errorf("redis client is nil")
	}
	return r.client.ZRange(context.TODO(), key, start, stop).Result()
}

func (r *Redis) ZRem(key string, members ...interface{}) error {
	if r.client == nil {
		return fmt.Errorf("redis client is nil")
	}
	return r.client.ZRem(context.TODO(), key, members...).Err()
}

func (r *Redis) ZAddXX(key string, member goredislib.Z) error {
	if r.client == nil {
		return fmt.Errorf("redis client is nil")
	}
	return r.client.ZAddXX(context.TODO(), key, member).Err()
}

func (r *Redis) SetEx(key string, value any, exp time.Duration) error {
	if r.client == nil {
		return nil
	}
	return r.client.SetEx(context.TODO(), key, value, exp).Err()
}

func (r *Redis) SCard(key string) (int64, error) {
	if r.client == nil {
		return 0, fmt.Errorf("redis client is nil")
	}
	return r.client.SCard(context.TODO(), key).Result()
}

func (r *Redis) Lock(key string) error {
	if r.rs == nil {
		return fmt.Errorf("redsync is nil")
	}
	var mutex *redsync.Mutex
	if _, ok := r.mutexs.Load(key); !ok {
		mutex = r.rs.NewMutex(key)
		r.mutexs.Store(key, mutex)
	}
	if err := mutex.Lock(); err != nil {
		return err
	}
	return nil
}

func (r *Redis) Unlock(key string) (bool, error) {
	if r.rs == nil {
		return false, fmt.Errorf("redsync is nil")
	}
	mutex, ok := r.mutexs.Load(key)
	if !ok {
		return false, fmt.Errorf("mutex not found")
	}
	ok, err := mutex.(*redsync.Mutex).Unlock()
	if err != nil {
		return false, err
	}
	r.mutexs.Delete(key)
	return ok, nil
}
