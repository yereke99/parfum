// internal/repository/redis-repository.go
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"parfum/internal/domain"

	"github.com/redis/go-redis/v9"
)

type RedisRepository struct {
	client *redis.Client
}

func NewRedisRepository(client *redis.Client) *RedisRepository {
	return &RedisRepository{client: client}
}

// User state methods
func (r *RedisRepository) SaveUserState(ctx context.Context, userID int64, state *domain.UserState) error {
	key := fmt.Sprintf("user_state:%d", userID)

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal user state: %w", err)
	}

	// Set expiration to 24 hours
	err = r.client.Set(ctx, key, data, 24*time.Hour).Err()
	if err != nil {
		return fmt.Errorf("failed to save user state to redis: %w", err)
	}

	return nil
}

func (r *RedisRepository) GetUserState(ctx context.Context, userID int64) (*domain.UserState, error) {
	key := fmt.Sprintf("user_state:%d", userID)

	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // Key doesn't exist
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user state from redis: %w", err)
	}

	var state domain.UserState
	err = json.Unmarshal([]byte(data), &state)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal user state: %w", err)
	}

	return &state, nil
}

func (r *RedisRepository) DeleteUserState(ctx context.Context, userID int64) error {
	key := fmt.Sprintf("user_state:%d", userID)

	err := r.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete user state from redis: %w", err)
	}

	return nil
}

// Admin state methods (using same UserState structure)
func (r *RedisRepository) SaveAdminState(ctx context.Context, adminID int64, state *domain.UserState) error {
	key := fmt.Sprintf("admin_state:%d", adminID)

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal admin state: %w", err)
	}

	// Set expiration to 24 hours
	err = r.client.Set(ctx, key, data, 24*time.Hour).Err()
	if err != nil {
		return fmt.Errorf("failed to save admin state to redis: %w", err)
	}

	return nil
}

func (r *RedisRepository) GetAdminState(ctx context.Context, adminID int64) (*domain.UserState, error) {
	key := fmt.Sprintf("admin_state:%d", adminID)

	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil, nil // Key doesn't exist
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get admin state from redis: %w", err)
	}

	var state domain.UserState
	err = json.Unmarshal([]byte(data), &state)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal admin state: %w", err)
	}

	return &state, nil
}

func (r *RedisRepository) DeleteAdminState(ctx context.Context, adminID int64) error {
	key := fmt.Sprintf("admin_state:%d", adminID)

	err := r.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete admin state from redis: %w", err)
	}

	return nil
}

// Broadcast state methods
func (r *RedisRepository) SaveBroadcastState(ctx context.Context, adminID int64, broadcastType string) error {
	key := fmt.Sprintf("broadcast_state:%d", adminID)

	// Set expiration to 1 hour for broadcast states
	err := r.client.Set(ctx, key, broadcastType, time.Hour).Err()
	if err != nil {
		return fmt.Errorf("failed to save broadcast state to redis: %w", err)
	}

	return nil
}

func (r *RedisRepository) GetBroadcastState(ctx context.Context, adminID int64) (string, error) {
	key := fmt.Sprintf("broadcast_state:%d", adminID)

	data, err := r.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil // Key doesn't exist
	}
	if err != nil {
		return "", fmt.Errorf("failed to get broadcast state from redis: %w", err)
	}

	return data, nil
}

func (r *RedisRepository) DeleteBroadcastState(ctx context.Context, adminID int64) error {
	key := fmt.Sprintf("broadcast_state:%d", adminID)

	err := r.client.Del(ctx, key).Err()
	if err != nil {
		return fmt.Errorf("failed to delete broadcast state from redis: %w", err)
	}

	return nil
}

// Helper method to clear all states for a user (useful for cleanup)
func (r *RedisRepository) ClearAllUserStates(ctx context.Context, userID int64) error {
	keys := []string{
		fmt.Sprintf("user_state:%d", userID),
		fmt.Sprintf("admin_state:%d", userID),
		fmt.Sprintf("broadcast_state:%d", userID),
	}

	err := r.client.Del(ctx, keys...).Err()
	if err != nil {
		return fmt.Errorf("failed to clear all user states from redis: %w", err)
	}

	return nil
}

// Health check method
func (r *RedisRepository) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}
