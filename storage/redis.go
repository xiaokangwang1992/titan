/*
 @Version : 1.0
 @Author  : steven.wong
 @Email   : 'wangxk1991@gamil.com'
 @Time    : 2024/01/24 17:30:35
 Desc     :
*/

package storage

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type Redis struct {
	client *redis.Client
}

func CheckRedis(url string) {
	redis := NewRedisClient(url)
	defer redis.Close()

	if err := redis.Set("test", "test", time.Duration(10)*time.Second); err != nil {
		panic(err)
	}
}

func NewRedisClient(url string) *Redis {
	opt, err := redis.ParseURL(url)
	if err != nil {
		panic(err)
	}
	rds := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(20)*time.Second)
	defer cancel()
	if _, err := rds.Ping(ctx).Result(); err != nil {
		panic(err)
	}

	return &Redis{client: rds}
}

func (r *Redis) Close() {
	r.client.Close()
}

func (r *Redis) Exists(key string) bool {
	return r.client.Exists(context.Background(), key).Val() == 1
}

func (r *Redis) Get(key string) (string, error) {
	return r.client.Get(context.TODO(), key).Result()
}

func (r *Redis) Set(key string, value interface{}, expiration time.Duration) error {
	return r.client.Set(context.TODO(), key, value, expiration).Err()
}

func (r *Redis) Del(key string) error {
	return r.client.Del(context.TODO(), key).Err()
}
