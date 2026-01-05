package idempotency

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Store struct {
	rdb *redis.Client
	ttl time.Duration
}

func NewStore(rdb *redis.Client, ttl time.Duration) *Store {
	return &Store{rdb: rdb, ttl: ttl}
}

func (s *Store) Key(topic string, partition int, offset int64) string {
	return fmt.Sprintf("idem:%s:%d:%d", topic, partition, offset)
}

func (s *Store) Seen(ctx context.Context, key string) (bool, error) {
	ok, err := s.rdb.SetNX(ctx, key, "1", s.ttl).Result()
	if err != nil {
		return false, err
	}

	return !ok, nil
}
