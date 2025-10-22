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
	"time"

	"github.com/die-net/lrucache"
	"github.com/go-redis/redis/v8"
	"github.com/gregjones/httpcache"
	"github.com/rcrowley/go-metrics"
)

const (
	MetricsKeyHTTPCacheLocalHits   = "cache.http.local.hits"
	MetricsKeyHTTPCacheRemoteHits  = "cache.http.remote.hits"
	MetricsKeyHTTPCacheTotalMisses = "cache.http.total.misses"
	MetricsKeyHTTPCacheSets        = "cache.http.sets"
	MetricsKeyHTTPCacheGetTime     = "cache.http.get.time"
	MetricsKeyHTTPCacheSetTime     = "cache.http.set.time"
)

// RedisHTTPCache implements httpcache.Cache with a two-tier strategy:
// a local LRU cache backed by Redis. This reduces latency for hot entries
// while sharing cache across pods via Redis.
type RedisHTTPCache struct {
	local    httpcache.Cache
	client   *redis.Client
	prefix   string
	registry metrics.Registry
}

// NewRedisHTTPCache creates an HTTP cache with local LRU backed by Redis.
// localSize is the size of the per-pod LRU cache in bytes.
func NewRedisHTTPCache(client *redis.Client, localSize int64, registry metrics.Registry) httpcache.Cache {
	return &RedisHTTPCache{
		local:    lrucache.New(localSize, 0),
		client:   client,
		prefix:   "http:",
		registry: registry,
	}
}

func (c *RedisHTTPCache) Get(key string) ([]byte, bool) {
	start := time.Now()

	if val, ok := c.local.Get(key); ok {
		if c.registry != nil {
			metrics.GetOrRegisterTimer(MetricsKeyHTTPCacheGetTime, c.registry).UpdateSince(start)
			metrics.GetOrRegisterCounter(MetricsKeyHTTPCacheLocalHits, c.registry).Inc(1)
		}
		return val, true
	}

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	val, err := c.client.Get(ctx, c.prefix+key).Bytes()
	if err == nil {
		c.local.Set(key, val)
		if c.registry != nil {
			metrics.GetOrRegisterTimer(MetricsKeyHTTPCacheGetTime, c.registry).UpdateSince(start)
			metrics.GetOrRegisterCounter(MetricsKeyHTTPCacheRemoteHits, c.registry).Inc(1)
		}
		return val, true
	}

	if c.registry != nil {
		metrics.GetOrRegisterTimer(MetricsKeyHTTPCacheGetTime, c.registry).UpdateSince(start)
		metrics.GetOrRegisterCounter(MetricsKeyHTTPCacheTotalMisses, c.registry).Inc(1)
	}

	return nil, false
}

func (c *RedisHTTPCache) Set(key string, responseBytes []byte) {
	start := time.Now()

	c.local.Set(key, responseBytes)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	c.client.Set(ctx, c.prefix+key, responseBytes, 0)

	if c.registry != nil {
		metrics.GetOrRegisterTimer(MetricsKeyHTTPCacheSetTime, c.registry).UpdateSince(start)
		metrics.GetOrRegisterCounter(MetricsKeyHTTPCacheSets, c.registry).Inc(1)
	}
}

func (c *RedisHTTPCache) Delete(key string) {
	c.local.Delete(key)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	c.client.Del(ctx, c.prefix+key)
}
