// Copyright 2025 Palantir Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cache

import (
	"context"
	"crypto/tls"
	"time"

	"github.com/c2h5oh/datasize"
	"github.com/die-net/lrucache"
	"github.com/go-redis/redis/v8"
	"github.com/gregjones/httpcache"
	"github.com/palantir/policy-bot/pull"
	"github.com/pkg/errors"
	"github.com/rcrowley/go-metrics"
	"github.com/rs/zerolog"
)

type RedisConfig struct {
	Enabled            bool
	Address            string
	Password           string
	TLS                bool
	LocalHTTPCacheSize datasize.ByteSize
	LocalPushedAtSize  int
}

// NewHTTPCacheFactory returns a factory function that creates HTTP caches.
// If Redis is configured, it returns Redis-backed caches with local LRU.
// Otherwise, it returns plain LRU caches.
func NewHTTPCacheFactory(redisConfig RedisConfig, defaultSize int64, registry metrics.Registry, logger zerolog.Logger) (func() httpcache.Cache, error) {
	if !redisConfig.Enabled {
		return func() httpcache.Cache {
			return lrucache.New(defaultSize, 0)
		}, nil
	}

	client, err := newRedisClient(redisConfig, logger)
	if err != nil {
		return nil, err
	}

	localSize := int64(redisConfig.LocalHTTPCacheSize)
	if localSize == 0 {
		localSize = defaultSize / 5
	}

	return func() httpcache.Cache {
		return NewRedisHTTPCache(client, localSize, registry)
	}, nil
}

// NewGlobalCache returns a GlobalCache implementation. If Redis is configured,
// it returns a Redis-backed cache with local LRU. Otherwise, it returns a
// plain LRU cache.
func NewGlobalCache(redisConfig RedisConfig, defaultSize int, registry metrics.Registry, logger zerolog.Logger) (pull.GlobalCache, error) {
	if !redisConfig.Enabled {
		return pull.NewLRUGlobalCache(defaultSize)
	}

	client, err := newRedisClient(redisConfig, logger)
	if err != nil {
		return nil, err
	}

	localSize := redisConfig.LocalPushedAtSize
	if localSize == 0 {
		localSize = defaultSize / 5
	}

	return pull.NewRedisGlobalCache(client, localSize, registry)
}

func newRedisClient(config RedisConfig, logger zerolog.Logger) (*redis.Client, error) {
	opts := &redis.Options{
		Addr:     config.Address,
		Password: config.Password,
	}
	if config.TLS {
		opts.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	client := redis.NewClient(opts)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		return nil, errors.Wrap(err, "failed to connect to Redis")
	}

	logger.Info().Str("address", config.Address).Msg("Connected to Redis cache")
	return client, nil
}
