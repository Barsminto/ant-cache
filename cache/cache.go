package cache

import (
	"ant-cache/auth"
	"container/heap"
	"fmt"
	"sync"
	"time"
)

type CacheItem struct {
	Value      interface{}
	Expiration int64
	index      int    // for heap operations
	key        string // for deletion operations
	Type       string // string, array, object
}

type Cache struct {
	items map[string]*CacheItem
	mu    sync.RWMutex
	// Min heap for expiration times, used to quickly find the earliest expiring items
	expirationHeap *ExpirationHeap
	// Persistence manager for data persistence
	persistence *PersistenceManager
	// Authentication manager
	authManager *auth.AuthManager
}

// ExpirationHeap implements min heap for managing expiration times
type ExpirationHeap []*CacheItem

func (h ExpirationHeap) Len() int { return len(h) }

func (h ExpirationHeap) Less(i, j int) bool {
	return h[i].Expiration < h[j].Expiration
}

func (h ExpirationHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *ExpirationHeap) Push(x interface{}) {
	n := len(*h)
	item := x.(*CacheItem)
	item.index = n
	*h = append(*h, item)
}

func (h *ExpirationHeap) Pop() interface{} {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // mark as deleted
	*h = old[0 : n-1]
	return item
}

func New() *Cache {
	h := &ExpirationHeap{}
	heap.Init(h)
	return &Cache{
		items:          make(map[string]*CacheItem),
		expirationHeap: h,
	}
}

// NewWithAuth creates cache with authentication
func NewWithAuth(authManager *auth.AuthManager) *Cache {
	cache := New()
	cache.authManager = authManager
	return cache
}

// NewWithPersistence create cache with persistence
func NewWithPersistence(atdPath, aclPath string, atdInterval, aclInterval time.Duration) *Cache {
	cache := New()
	if atdPath != "" && aclPath != "" {
		cache.persistence = NewPersistenceManager(cache, atdPath, aclPath, atdInterval, aclInterval)
		// Load data when starting
		if err := cache.persistence.LoadAtd(); err != nil {
			// Loading data does not affect startup, just print the error
			fmt.Printf("Failed to load ATD: %v\n", err)
		}
		// Load command log
		if err := cache.persistence.LoadAcl(); err != nil {
			// Loading command log does not affect startup, just print the error
			fmt.Printf("Failed to load ACL: %v\n", err)
		}
		// Start persistence manager
		cache.persistence.Start()
	}
	return cache
}

// NewWithPersistenceAndAuth create cache with persistence and authentication
func NewWithPersistenceAndAuth(atdPath, aclPath string, atdInterval, aclInterval time.Duration, authManager *auth.AuthManager) *Cache {
	cache := NewWithPersistence(atdPath, aclPath, atdInterval, aclInterval)
	cache.authManager = authManager
	return cache
}

// Close the cache and save the data
func (c *Cache) Close() {
	if c.persistence != nil {
		c.persistence.Stop()
	}
}

func (c *Cache) Set(key string, value interface{}, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Determine data type
	var dataType string
	switch value.(type) {
	case []string:
		dataType = "array"
	case map[string]string:
		dataType = "object"
	default:
		dataType = "string"
	}

	item := &CacheItem{
		Value: value,
		key:   key,
		Type:  dataType,
	}

	// Only set expiration time when ttl > 0
	if ttl > 0 {
		item.Expiration = time.Now().Add(ttl).UnixNano()
		// Add to expiration heap
		heap.Push(c.expirationHeap, item)
	}

	c.items[key] = item

	// Removed stats tracking

	// Log the set command to the command log
	if c.persistence != nil {
		c.persistence.LogCommand("SET", key, value, ttl)
	}
}

// SetNX sets a key only if it doesn't exist (atomic operation)
func (c *Cache) SetNX(key string, value interface{}, ttl time.Duration) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Check if key already exists and is not expired
	if item, exists := c.items[key]; exists {
		// Check if item is expired
		if item.Expiration > 0 && time.Now().UnixNano() > item.Expiration {
			// Item is expired, we can set it
		} else {
			// Key exists and is not expired, return false
			return false
		}
	}

	// Determine data type
	var dataType string
	switch value.(type) {
	case []string:
		dataType = "array"
	case map[string]string:
		dataType = "object"
	default:
		dataType = "string"
	}

	item := &CacheItem{
		Value: value,
		key:   key,
		Type:  dataType,
	}

	// Only set expiration time when ttl > 0
	if ttl > 0 {
		item.Expiration = time.Now().Add(ttl).UnixNano()
		// Add to expiration heap
		heap.Push(c.expirationHeap, item)
	}

	c.items[key] = item

	// Log the set command to the command log
	if c.persistence != nil {
		c.persistence.LogCommand("SETNX", key, value, ttl)
	}

	return true
}

func (c *Cache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	item, found := c.items[key]
	if !found {
		// stats tracking removed
		return nil, false
	}

	if item.Expiration > 0 && time.Now().UnixNano() > item.Expiration {
		// stats tracking removed
		return nil, false
	}

	// Removed stats tracking
	return item.Value, true
}

// GetMultiple gets multiple keys at once
func (c *Cache) GetMultiple(keys []string) map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]interface{})
	now := time.Now().UnixNano()

	for _, key := range keys {
		if item, found := c.items[key]; found {
			// Check if item is expired
			if item.Expiration > 0 && now > item.Expiration {
				continue // Skip expired items
			}
			result[key] = item.Value
		}
	}

	return result
}

func (c *Cache) Delete(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if item, exists := c.items[key]; exists {
		// If has expiration time, remove from heap
		if item.Expiration > 0 && item.index >= 0 {
			heap.Remove(c.expirationHeap, item.index)
		}
		delete(c.items, key)

		// Removed stats tracking

		// Log the delete command to the command log
		if c.persistence != nil {
			c.persistence.LogCommand("DEL", key, "", 0)
		}
		return true
	}
	return false
}

// DeleteString method removed - use Delete instead

// DeleteArray method removed - use Delete instead

// DeleteObject method removed - use Delete instead

func (c *Cache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now().UnixNano()

	// Only check the top of the heap for expired items, avoid traversing all keys
	for c.expirationHeap.Len() > 0 {
		item := (*c.expirationHeap)[0]
		if item.Expiration > now {
			// Top item not expired yet, stop checking
			break
		}

		heap.Pop(c.expirationHeap)
		delete(c.items, item.key)
	}
}

// GetStats method removed

// GetAuthManager obtain the authentication manager
func (c *Cache) GetAuthManager() *auth.AuthManager {
	return c.authManager
}

// GetAllKeys returns all keys with their metadata
func (c *Cache) GetAllKeys() []map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var keys []map[string]interface{}
	now := time.Now().UnixNano()

	// Collect all keys from map
	for key, item := range c.items {
		// Skip expired items
		if item.Expiration > 0 && item.Expiration < now {
			continue
		}

		// Calculate TTL
		var ttl int64 = 0
		if item.Expiration > 0 {
			ttl = (item.Expiration - now) / int64(time.Second)
			if ttl < 0 {
				ttl = 0
			}
		}

		// Calculate size
		size := len(fmt.Sprintf("%v", item.Value))

		keyInfo := map[string]interface{}{
			"key":        key,
			"type":       item.Type,
			"value":      item.Value,
			"ttl":        ttl,
			"expires_at": "",
			"size":       size,
		}

		if item.Expiration > 0 {
			keyInfo["expires_at"] = time.Unix(0, item.Expiration).Format(time.RFC3339)
		}

		keys = append(keys, keyInfo)
	}

	// Keys are returned in map iteration order (no specific sorting)

	return keys
}

// FlushAll removes all keys from the cache
func (c *Cache) FlushAll() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	count := len(c.items)

	// Clear all items
	c.items = make(map[string]*CacheItem)

	// Reset expiration heap
	c.expirationHeap = &ExpirationHeap{}
	heap.Init(c.expirationHeap)

	return count
}
