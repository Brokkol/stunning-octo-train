package repository

import (
	"context"
	"encoding/json"
	"time"

	"Go_project/internal/domain"

	"github.com/go-redis/redis/v8"
)

type RedisCacheRepository struct {
	client *redis.Client
}

func NewRedisCacheRepository(client *redis.Client) *RedisCacheRepository {
	return &RedisCacheRepository{client: client}
}

func (r *RedisCacheRepository) SetUser(key string, user *domain.User) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	data, err := json.Marshal(user)
	if err != nil {
		return err
	}

	return r.client.Set(ctx, key, data, 10*time.Minute).Err()
}

func (r *RedisCacheRepository) GetUser(key string) (*domain.User, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	val, err := r.client.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var user domain.User
	if err := json.Unmarshal([]byte(val), &user); err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *RedisCacheRepository) IncrementCounter(key string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return r.client.Incr(ctx, key).Result()
}