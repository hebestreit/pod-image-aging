package cache

import (
	"sync"
	"time"
)

type CacheItem struct {
	Value      time.Time
	Expiration int64 // Unix timestamp to determine expiration time
}

type Cache struct {
	data  map[string]CacheItem
	mutex sync.RWMutex
}

// NewCache Create a new cache
func NewCache() *Cache {
	return &Cache{
		data: make(map[string]CacheItem),
	}
}

// Set a key-value pair with expiration time (in seconds)
func (c *Cache) Set(key string, value time.Time, duration time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	c.data[key] = CacheItem{
		Value:      value,
		Expiration: time.Now().Add(duration).Unix(),
	}
}

// Get the value by key, returns the value and a bool indicating if it exists and is not expired
func (c *Cache) Get(key string) (*time.Time, bool) {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	item, exists := c.data[key]
	if !exists || time.Now().Unix() > item.Expiration {
		return nil, false
	}

	return &item.Value, true
}
