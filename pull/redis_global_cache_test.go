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
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/rcrowley/go-metrics"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRedisGlobalCacheGetPushedAt(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer client.Close()

	registry := metrics.NewRegistry()
	cache, err := NewRedisGlobalCache(client, 1000, registry)
	require.NoError(t, err)

	expectedTime := time.Date(2025, 10, 13, 12, 0, 0, 0, time.UTC)

	t.Run("localHit", func(t *testing.T) {
		cache.SetPushedAt(1234, "abc123", expectedTime)

		result, ok := cache.GetPushedAt(1234, "abc123")
		require.True(t, ok)
		assert.Equal(t, expectedTime, result)

		localHits := metrics.GetOrRegisterCounter(MetricsKeyGlobalCacheLocalHits, registry).Count()
		assert.Equal(t, int64(1), localHits)
	})

	t.Run("remoteHit", func(t *testing.T) {
		timeBytes, err := json.Marshal(expectedTime)
		require.NoError(t, err)
		client.Set(client.Context(), "pushedat:5678:def456", timeBytes, 0)

		result, ok := cache.GetPushedAt(5678, "def456")
		require.True(t, ok)
		assert.Equal(t, expectedTime, result)

		remoteHits := metrics.GetOrRegisterCounter(MetricsKeyGlobalCacheRemoteHits, registry).Count()
		assert.Equal(t, int64(1), remoteHits)
	})

	t.Run("totalMiss", func(t *testing.T) {
		result, ok := cache.GetPushedAt(9999, "nonexistent")
		require.False(t, ok)
		assert.Equal(t, time.Time{}, result)

		totalMisses := metrics.GetOrRegisterCounter(MetricsKeyGlobalCacheTotalMisses, registry).Count()
		assert.Equal(t, int64(1), totalMisses)
	})
}

func TestRedisGlobalCacheSetPushedAt(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer client.Close()

	registry := metrics.NewRegistry()
	cache, err := NewRedisGlobalCache(client, 1000, registry)
	require.NoError(t, err)

	expectedTime := time.Date(2025, 10, 13, 12, 0, 0, 0, time.UTC)
	cache.SetPushedAt(1234, "abc123", expectedTime)

	localResult, localOk := cache.GetPushedAt(1234, "abc123")
	require.True(t, localOk)
	assert.Equal(t, expectedTime, localResult)

	remoteVal, err := client.Get(client.Context(), "pushedat:1234:abc123").Result()
	require.NoError(t, err)
	var remoteTime time.Time
	err = json.Unmarshal([]byte(remoteVal), &remoteTime)
	require.NoError(t, err)
	assert.Equal(t, expectedTime, remoteTime)

	sets := metrics.GetOrRegisterCounter(MetricsKeyGlobalCacheSets, registry).Count()
	assert.Equal(t, int64(1), sets)
}

func TestRedisGlobalCacheTwoTier(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})
	defer client.Close()

	cache, err := NewRedisGlobalCache(client, 1000, nil)
	require.NoError(t, err)

	expectedTime := time.Date(2025, 10, 13, 12, 0, 0, 0, time.UTC)
	timeBytes, err := json.Marshal(expectedTime)
	require.NoError(t, err)
	client.Set(client.Context(), "pushedat:1234:abc123", timeBytes, 0)

	result, ok := cache.GetPushedAt(1234, "abc123")
	require.True(t, ok)
	assert.Equal(t, expectedTime, result)

	localResult, localOk := cache.GetPushedAt(1234, "abc123")
	require.True(t, localOk)
	assert.Equal(t, expectedTime, localResult)
}
