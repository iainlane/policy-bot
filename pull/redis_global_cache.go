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

package pull

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"
	lru "github.com/hashicorp/golang-lru"
	"github.com/rcrowley/go-metrics"
)

const (
	MetricsKeyGlobalCacheLocalHits   = "cache.global.local.hits"
	MetricsKeyGlobalCacheRemoteHits  = "cache.global.remote.hits"
	MetricsKeyGlobalCacheTotalMisses = "cache.global.total.misses"
	MetricsKeyGlobalCacheSets        = "cache.global.sets"
	MetricsKeyGlobalCacheGetTime     = "cache.global.get.time"
	MetricsKeyGlobalCacheSetTime     = "cache.global.set.time"
)

// RedisGlobalCache is a GlobalCache implementation with a two-tier strategy:
// a local LRU cache backed by Redis. This reduces latency for hot entries
// while sharing cache across pods via Redis.
type RedisGlobalCache struct {
	local    *lru.Cache
	client   *redis.Client
	prefix   string
	registry metrics.Registry
}

// NewRedisGlobalCache creates a global cache with local LRU backed by Redis.
// localSize is the number of entries in the per-pod LRU cache.
func NewRedisGlobalCache(client *redis.Client, localSize int, registry metrics.Registry) (GlobalCache, error) {
	local, err := lru.New(localSize)
	if err != nil {
		return nil, err
	}

	return &RedisGlobalCache{
		local:    local,
		client:   client,
		prefix:   "pushedat:",
		registry: registry,
	}, nil
}

func (c *RedisGlobalCache) GetPushedAt(repoID int64, sha string) (time.Time, bool) {
	start := time.Now()
	key := pushedAtKey(repoID, sha)

	if val, ok := c.local.Get(key); ok {
		if t, ok := val.(time.Time); ok {
			if c.registry != nil {
				metrics.GetOrRegisterTimer(MetricsKeyGlobalCacheGetTime, c.registry).UpdateSince(start)
				metrics.GetOrRegisterCounter(MetricsKeyGlobalCacheLocalHits, c.registry).Inc(1)
			}
			return t, true
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	val, err := c.client.Get(ctx, c.prefix+key).Result()
	if err == nil {
		var t time.Time
		if err := json.Unmarshal([]byte(val), &t); err == nil {
			c.local.Add(key, t)
			if c.registry != nil {
				metrics.GetOrRegisterTimer(MetricsKeyGlobalCacheGetTime, c.registry).UpdateSince(start)
				metrics.GetOrRegisterCounter(MetricsKeyGlobalCacheRemoteHits, c.registry).Inc(1)
			}
			return t, true
		}
	}

	if c.registry != nil {
		metrics.GetOrRegisterTimer(MetricsKeyGlobalCacheGetTime, c.registry).UpdateSince(start)
		metrics.GetOrRegisterCounter(MetricsKeyGlobalCacheTotalMisses, c.registry).Inc(1)
	}

	return time.Time{}, false
}

func (c *RedisGlobalCache) SetPushedAt(repoID int64, sha string, t time.Time) {
	start := time.Now()
	key := pushedAtKey(repoID, sha)

	c.local.Add(key, t)

	val, err := json.Marshal(t)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	c.client.Set(ctx, c.prefix+key, val, 0)

	if c.registry != nil {
		metrics.GetOrRegisterTimer(MetricsKeyGlobalCacheSetTime, c.registry).UpdateSince(start)
		metrics.GetOrRegisterCounter(MetricsKeyGlobalCacheSets, c.registry).Inc(1)
	}
}
