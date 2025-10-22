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
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/rcrowley/go-metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisHTTPCacheGet(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer client.Close()

	registry := metrics.NewRegistry()
	cache := NewRedisHTTPCache(client, 1024*1024, registry)

	t.Run("localHit", func(t *testing.T) {
		cache.Set("test-key-1", []byte("test-value-1"))

		val, ok := cache.Get("test-key-1")
		require.True(t, ok)
		assert.Equal(t, []byte("test-value-1"), val)

		localHits := metrics.GetOrRegisterCounter(MetricsKeyHTTPCacheLocalHits, registry).Count()
		assert.Equal(t, int64(1), localHits)
	})

	t.Run("remoteHit", func(t *testing.T) {
		client.Set(client.Context(), "http:test-key-2", []byte("test-value-2"), 0)

		val, ok := cache.Get("test-key-2")
		require.True(t, ok)
		assert.Equal(t, []byte("test-value-2"), val)

		remoteHits := metrics.GetOrRegisterCounter(MetricsKeyHTTPCacheRemoteHits, registry).Count()
		assert.Equal(t, int64(1), remoteHits)
	})

	t.Run("totalMiss", func(t *testing.T) {
		val, ok := cache.Get("nonexistent-key")
		require.False(t, ok)
		assert.Nil(t, val)

		totalMisses := metrics.GetOrRegisterCounter(MetricsKeyHTTPCacheTotalMisses, registry).Count()
		assert.Equal(t, int64(1), totalMisses)
	})
}

func TestRedisHTTPCacheSet(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer client.Close()

	registry := metrics.NewRegistry()
	cache := NewRedisHTTPCache(client, 1024*1024, registry)

	cache.Set("test-key", []byte("test-value"))

	localVal, localOk := cache.Get("test-key")
	require.True(t, localOk)
	assert.Equal(t, []byte("test-value"), localVal)

	remoteVal, err := client.Get(client.Context(), "http:test-key").Bytes()
	require.NoError(t, err)
	assert.Equal(t, []byte("test-value"), remoteVal)

	sets := metrics.GetOrRegisterCounter(MetricsKeyHTTPCacheSets, registry).Count()
	assert.Equal(t, int64(1), sets)
}

func TestRedisHTTPCacheDelete(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer client.Close()

	cache := NewRedisHTTPCache(client, 1024*1024, nil)

	cache.Set("test-key", []byte("test-value"))

	localVal, localOk := cache.Get("test-key")
	require.True(t, localOk)
	assert.Equal(t, []byte("test-value"), localVal)

	cache.Delete("test-key")

	_, localOk = cache.Get("test-key")
	assert.False(t, localOk)

	_, err = client.Get(client.Context(), "http:test-key").Result()
	assert.Error(t, err)
	assert.Equal(t, redis.Nil, err)
}

func TestRedisHTTPCacheTwoTier(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer client.Close()

	cache := NewRedisHTTPCache(client, 1024*1024, nil)

	client.Set(client.Context(), "http:test-key", []byte("redis-value"), 0)

	val, ok := cache.Get("test-key")
	require.True(t, ok)
	assert.Equal(t, []byte("redis-value"), val)

	localVal, localOk := cache.Get("test-key")
	require.True(t, localOk)
	assert.Equal(t, []byte("redis-value"), localVal)
}
