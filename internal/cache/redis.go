// Package cache provides a tiny Redis client wrapper for robot pose caching
package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v9"
)

// Cache wraps a Redis client for robot pose storage
type Cache struct {
	client *redis.Client
}

// New creates a new Cache instance connected to the specified Redis address
// If addr is empty, defaults to localhost:6379
func New(addr string) (*Cache, error) {
	if addr == "" {
		addr = "localhost:6379"
	}

	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "", // No password by default
		DB:       0,  // Default DB
	})

	// Test connection
	ctx := context.Background()
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Redis at %s: %w", addr, err)
	}

	return &Cache{client: client}, nil
}

// SetPose stores a robot's pose data with the specified TTL
func (c *Cache) SetPose(robotID uint64, data string, ttl time.Duration) error {
	if c.client == nil {
		return fmt.Errorf("cache client is nil")
	}

	ctx := context.Background()
	key := fmt.Sprintf("robot:%d:pose", robotID)

	err := c.client.Set(ctx, key, data, ttl).Err()
	if err != nil {
		return fmt.Errorf("failed to set pose for robot %d: %w", robotID, err)
	}

	return nil
}

// GetPose retrieves a robot's pose data
func (c *Cache) GetPose(robotID uint64) (string, error) {
	if c.client == nil {
		return "", fmt.Errorf("cache client is nil")
	}

	ctx := context.Background()
	key := fmt.Sprintf("robot:%d:pose", robotID)

	data, err := c.client.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil // Key does not exist
	}
	if err != nil {
		return "", fmt.Errorf("failed to get pose for robot %d: %w", robotID, err)
	}

	return data, nil
}

// Close closes the Redis connection
func (c *Cache) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}
