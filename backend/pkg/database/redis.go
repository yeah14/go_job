package database

import (
	"context"
	"fmt"
	"time"

	"go-job/config"

	"github.com/redis/go-redis/v9"
)

var redisClient *redis.Client

func NewRedis(cfg config.RedisConfig) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("connect redis failed: %w", err)
	}

	return client, nil
}

func InitRedis(cfg config.RedisConfig) error {
	client, err := NewRedis(cfg)
	if err != nil {
		return err
	}
	redisClient = client
	return nil
}

func Redis() *redis.Client {
	return redisClient
}

func CloseRedis() error {
	if redisClient == nil {
		return nil
	}
	return redisClient.Close()
}
