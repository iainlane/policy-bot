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
	"os"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/c2h5oh/datasize"
	"github.com/palantir/policy-bot/pull"
	"github.com/rcrowley/go-metrics"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHTTPCacheFactoryDisabled(t *testing.T) {
	logger := zerolog.New(os.Stdout)
	registry := metrics.NewRegistry()

	config := RedisConfig{
		Enabled: false,
	}

	factory, err := NewHTTPCacheFactory(config, 50*1024*1024, registry, logger)
	require.NoError(t, err)

	cache := factory()
	require.NotNil(t, cache)

	_, ok := cache.(*RedisHTTPCache)
	assert.False(t, ok, "expected plain LRU cache when Redis is disabled")
}

func TestNewHTTPCacheFactoryEnabled(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	logger := zerolog.New(os.Stdout)
	registry := metrics.NewRegistry()

	config := RedisConfig{
		Enabled:            true,
		Address:            mr.Addr(),
		LocalHTTPCacheSize: 10 * datasize.MB,
	}

	factory, err := NewHTTPCacheFactory(config, 50*1024*1024, registry, logger)
	require.NoError(t, err)

	cache := factory()
	require.NotNil(t, cache)

	_, ok := cache.(*RedisHTTPCache)
	assert.True(t, ok, "expected RedisHTTPCache when Redis is enabled")
}

func TestNewHTTPCacheFactoryEnabledDefaultLocalSize(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	logger := zerolog.New(os.Stdout)
	registry := metrics.NewRegistry()

	config := RedisConfig{
		Enabled: true,
		Address: mr.Addr(),
	}

	factory, err := NewHTTPCacheFactory(config, 50*1024*1024, registry, logger)
	require.NoError(t, err)

	cache := factory()
	require.NotNil(t, cache)

	redisCache, ok := cache.(*RedisHTTPCache)
	assert.True(t, ok, "expected RedisHTTPCache when Redis is enabled")
	assert.NotNil(t, redisCache)
}

func TestNewHTTPCacheFactoryConnectionFailure(t *testing.T) {
	logger := zerolog.New(os.Stdout)
	registry := metrics.NewRegistry()

	config := RedisConfig{
		Enabled: true,
		Address: "localhost:9999",
	}

	_, err := NewHTTPCacheFactory(config, 50*1024*1024, registry, logger)
	assert.Error(t, err, "expected error when Redis connection fails")
}

func TestNewGlobalCacheDisabled(t *testing.T) {
	logger := zerolog.New(os.Stdout)
	registry := metrics.NewRegistry()

	config := RedisConfig{
		Enabled: false,
	}

	cache, err := NewGlobalCache(config, 100000, registry, logger)
	require.NoError(t, err)
	require.NotNil(t, cache)

	_, ok := cache.(*pull.RedisGlobalCache)
	assert.False(t, ok, "expected plain LRU cache when Redis is disabled")
}

func TestNewGlobalCacheEnabled(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	logger := zerolog.New(os.Stdout)
	registry := metrics.NewRegistry()

	config := RedisConfig{
		Enabled:           true,
		Address:           mr.Addr(),
		LocalPushedAtSize: 20000,
	}

	cache, err := NewGlobalCache(config, 100000, registry, logger)
	require.NoError(t, err)
	require.NotNil(t, cache)

	_, ok := cache.(*pull.RedisGlobalCache)
	assert.True(t, ok, "expected RedisGlobalCache when Redis is enabled")
}

func TestNewGlobalCacheEnabledDefaultLocalSize(t *testing.T) {
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	logger := zerolog.New(os.Stdout)
	registry := metrics.NewRegistry()

	config := RedisConfig{
		Enabled: true,
		Address: mr.Addr(),
	}

	cache, err := NewGlobalCache(config, 100000, registry, logger)
	require.NoError(t, err)
	require.NotNil(t, cache)

	redisCache, ok := cache.(*pull.RedisGlobalCache)
	assert.True(t, ok, "expected RedisGlobalCache when Redis is enabled")
	assert.NotNil(t, redisCache)
}

func TestNewGlobalCacheConnectionFailure(t *testing.T) {
	logger := zerolog.New(os.Stdout)
	registry := metrics.NewRegistry()

	config := RedisConfig{
		Enabled: true,
		Address: "localhost:9999",
	}

	_, err := NewGlobalCache(config, 100000, registry, logger)
	assert.Error(t, err, "expected error when Redis connection fails")
}
